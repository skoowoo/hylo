#!/bin/bash
# Notarize a Hylo DMG and staple the ticket.
# The CLI shipped inside Hylo.app/Contents/Resources/cli/hylo.tar.gz is a Mach-O
# the notary service unpacks. It must be Developer ID signed, with the hardened
# runtime and a secure timestamp, or the whole DMG is rejected.
#
# Credentials: keychain profile "hylo-notary" (NOTARY_KEYCHAIN_PROFILE overrides),
# or APPLE_ID + APPLE_PASSWORD + APPLE_TEAM_ID.
#
#   notarize-dmg.sh sign-archive <hylo.tar.gz>   # sign the CLI before tauri seals the app
#   notarize-dmg.sh <Hylo.dmg>                   # sign the embedded CLI if needed, submit, staple
set -euo pipefail

ROOT=$(CDPATH= cd -- "$(dirname "$0")" && pwd)

usage() {
  echo "Usage: notarize-dmg.sh sign-archive <hylo.tar.gz>" >&2
  echo "       notarize-dmg.sh <Hylo.dmg>" >&2
  exit 2
}

IDENTITY=$(sh "$ROOT/signing-identity.sh")
case "$IDENTITY" in
  "Developer ID Application:"*) ;;
  *)
    echo "Developer ID Application identity required, got: $IDENTITY" >&2
    exit 1
    ;;
esac

# Sign every Mach-O in a hylo CLI archive, then pack it back the way the app installer expects.
sign_archive() {
  archive=$1
  dir=$(mktemp -d "${TMPDIR:-/tmp}/hylo-cli.XXXX")
  COPYFILE_DISABLE=1 tar -xzf "$archive" -C "$dir"
  if [ ! -f "$dir/hylo" ]; then
    rm -rf "$dir"
    echo "hylo missing from $archive" >&2
    exit 1
  fi

  found=0
  while IFS= read -r -d '' bin; do
    case $(basename "$bin") in
      ._*) continue ;;
    esac
    if file -b "$bin" | grep -q Mach-O; then
      xattr -c "$bin" 2>/dev/null || true
      if [ "$(basename "$bin")" = "hylo" ]; then
        codesign --force --sign "$IDENTITY" --options runtime --timestamp \
          --identifier "dev.hardhacker.hylo.cli" "$bin"
      else
        codesign --force --sign "$IDENTITY" --options runtime --timestamp "$bin"
      fi
      codesign --verify --strict --verbose=2 "$bin"
      chmod 755 "$bin"
      found=1
    fi
  done < <(find "$dir" -type f -print0)

  if [ "$found" -eq 0 ]; then
    rm -rf "$dir"
    echo "No Mach-O in $archive" >&2
    exit 1
  fi

  names=()
  for path in "$dir"/*; do
    names+=("$(basename "$path")")
  done
  packed=$(mktemp "${TMPDIR:-/tmp}/hylo-cli.XXXX")
  rm -f "$packed"
  packed="${packed}.tar.gz"
  COPYFILE_DISABLE=1 tar -czf "$packed" -C "$dir" "${names[@]}"

  check=$(mktemp -d "${TMPDIR:-/tmp}/hylo-cli.XXXX")
  COPYFILE_DISABLE=1 tar -xzf "$packed" -C "$check"
  codesign --verify --strict --verbose=2 "$check/hylo"
  rm -rf "$check" "$dir"

  mv "$packed" "$archive"
}

cli_is_signed() {
  archive=$1
  dir=$(mktemp -d "${TMPDIR:-/tmp}/hylo-cli.XXXX")
  COPYFILE_DISABLE=1 tar -xzf "$archive" -C "$dir"
  ok=0
  if [ -f "$dir/hylo" ]; then
    info=$(codesign -dv --verbose=4 "$dir/hylo" 2>&1 || true)
    if printf '%s\n' "$info" | grep -q 'Developer ID Application:' \
      && printf '%s\n' "$info" | grep -q 'runtime' \
      && printf '%s\n' "$info" | grep -E '^Timestamp=' | grep -qv 'Timestamp=none'; then
      ok=1
    fi
  fi
  rm -rf "$dir"
  [ "$ok" -eq 1 ]
}

notary_auth() {
  if [ -n "${APPLE_ID:-}" ] && [ -n "${APPLE_PASSWORD:-}" ] && [ -n "${APPLE_TEAM_ID:-}" ]; then
    NOTARY_AUTH=(--apple-id "$APPLE_ID" --password "$APPLE_PASSWORD" --team-id "$APPLE_TEAM_ID")
    NOTARY_AUTH_LABEL="Apple ID $APPLE_ID"
    return
  fi
  profile=${NOTARY_KEYCHAIN_PROFILE:-hylo-notary}
  NOTARY_AUTH=(--keychain-profile "$profile")
  NOTARY_AUTH_LABEL="keychain profile $profile"
}

MNT=
WORK=
cleanup() {
  if [ -n "$MNT" ]; then
    hdiutil detach "$MNT" >/dev/null 2>&1 || hdiutil detach -force "$MNT" >/dev/null 2>&1 || true
    MNT=
  fi
  if [ -n "$WORK" ]; then
    rm -rf "$WORK"
    WORK=
  fi
}
trap cleanup EXIT

mount_dmg() {
  image=$1
  mode=$2
  MNT=$(mktemp -d "${TMPDIR:-/tmp}/hylo-dmg.XXXX")
  if [ "$mode" = "rw" ]; then
    hdiutil attach "$image" -nobrowse -readwrite -owners off -noautoopen -mountpoint "$MNT" >/dev/null
  else
    hdiutil attach "$image" -nobrowse -readonly -owners off -noautoopen -mountpoint "$MNT" >/dev/null
  fi
  APP=$(find "$MNT" -maxdepth 2 -type d -name 'Hylo.app' -print -quit)
  if [ -z "$APP" ]; then
    echo "Hylo.app not found in $image" >&2
    exit 1
  fi
  ARCHIVE="$APP/Contents/Resources/cli/hylo.tar.gz"
  if [ ! -f "$ARCHIVE" ]; then
    echo "Bundled CLI archive not found at Contents/Resources/cli/hylo.tar.gz" >&2
    exit 1
  fi
}

detach_dmg() {
  if [ -n "$MNT" ]; then
    hdiutil detach "$MNT" >/dev/null
    rm -rf "$MNT"
    MNT=
  fi
}

# Replace an unsigned CLI inside an already sealed DMG, then sign the app and the image again.
resign_dmg() {
  dmg=$1
  WORK=$(mktemp -d "${TMPDIR:-/tmp}/hylo-notarize.XXXX")
  cp "$dmg" "$WORK/source.dmg"
  hdiutil convert "$WORK/source.dmg" -format UDRW -o "$WORK/rw.dmg" >/dev/null
  if [ ! -f "$WORK/rw.dmg" ] && [ -f "$WORK/rw.dmg.dmg" ]; then
    mv "$WORK/rw.dmg.dmg" "$WORK/rw.dmg"
  fi
  bytes=$(stat -f%z "$WORK/rw.dmg")
  hdiutil resize -size "$((bytes / 1048576 + 64))m" "$WORK/rw.dmg" >/dev/null

  mount_dmg "$WORK/rw.dmg" rw
  staged="$WORK/Hylo.app"
  ditto "$APP" "$staged"
  xattr -cr "$staged" 2>/dev/null || true
  sign_archive "$staged/Contents/Resources/cli/hylo.tar.gz"
  ents="$WORK/entitlements.plist"
  codesign -d --entitlements "$ents" --xml "$staged"
  test -s "$ents"
  codesign --force --sign "$IDENTITY" --options runtime --timestamp \
    --entitlements "$ents" "$staged"
  codesign --verify --strict --verbose=2 "$staged"

  rm -rf "$APP"
  ditto "$staged" "$APP"
  codesign --verify --strict --verbose=2 "$APP"
  sync
  detach_dmg

  hdiutil convert "$WORK/rw.dmg" -format UDZO -imagekey zlib-level=9 -o "$WORK/out.dmg" >/dev/null
  if [ ! -f "$WORK/out.dmg" ] && [ -f "$WORK/out.dmg.dmg" ]; then
    mv "$WORK/out.dmg.dmg" "$WORK/out.dmg"
  fi
  # codesign takes the identifier from the filename. out.dmg would be recorded as "out".
  codesign --force --sign "$IDENTITY" --timestamp \
    --identifier "$(basename "$dmg" .dmg)" "$WORK/out.dmg"
  codesign --verify --verbose=2 "$WORK/out.dmg"

  mount_dmg "$WORK/out.dmg" ro
  if ! cli_is_signed "$ARCHIVE"; then
    echo "CLI signature did not survive repacking the DMG" >&2
    exit 1
  fi
  codesign --verify --strict --verbose=2 "$APP"
  detach_dmg

  mv "$WORK/out.dmg" "$dmg"
  rm -rf "$WORK"
  WORK=
}

submit_and_staple() {
  dmg=$1
  notary_auth
  echo "==> Submitting $(basename "$dmg") ($NOTARY_AUTH_LABEL)"
  set +e
  json=$(xcrun notarytool submit "$dmg" "${NOTARY_AUTH[@]}" --wait --output-format json)
  rc=$?
  set -e
  status=$(printf '%s' "$json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))' 2>/dev/null || true)
  id=$(printf '%s' "$json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("id",""))' 2>/dev/null || true)
  if [ "$rc" -ne 0 ] || [ "$status" != "Accepted" ]; then
    echo "Notarization failed (status=${status:-unknown}, id=${id:-none})" >&2
    printf '%s\n' "$json" >&2
    if [ -n "$id" ]; then
      log=$(mktemp "${TMPDIR:-/tmp}/hylo-notary-log.XXXX")
      xcrun notarytool log "$id" "${NOTARY_AUTH[@]}" "$log" >/dev/null
      python3 -c '
import json, sys
data = json.load(open(sys.argv[1]))
summary = data.get("statusSummary")
if summary:
    print(summary, file=sys.stderr)
for issue in data.get("issues") or []:
    print("%s: %s: %s" % (issue.get("severity"), issue.get("path"), issue.get("message")), file=sys.stderr)
' "$log" >&2 || true
      rm -f "$log"
    fi
    exit 1
  fi
  echo "==> Accepted $id; stapling"
  xcrun stapler staple "$dmg"
  xcrun stapler validate "$dmg"
  echo "==> Stapled $dmg"
}

if [ "${1:-}" = "sign-archive" ]; then
  [ $# -eq 2 ] || usage
  echo "==> Signing CLI in $2"
  sign_archive "$2"
  exit 0
fi

[ $# -eq 1 ] || usage
DMG=$1
[ -f "$DMG" ] || {
  echo "DMG not found: $DMG" >&2
  exit 1
}

mount_dmg "$DMG" ro
needs_resign=0
if cli_is_signed "$ARCHIVE"; then
  echo "==> Embedded CLI is already Developer ID signed"
else
  echo "==> Embedded CLI is not notarization-ready; signing it and re-sealing the DMG"
  needs_resign=1
fi
detach_dmg

if [ "$needs_resign" -eq 1 ]; then
  resign_dmg "$DMG"
fi

if xcrun stapler validate "$DMG" >/dev/null 2>&1; then
  echo "==> Already stapled: $DMG"
  exit 0
fi

submit_and_staple "$DMG"
