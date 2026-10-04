#!/bin/sh
# Prints the codesign identity electron-builder would pick. "-" is ad-hoc.
set -eu

if [ -n "${APPLE_SIGNING_IDENTITY:-}" ]; then
  printf '%s\n' "$APPLE_SIGNING_IDENTITY"
  exit 0
fi

if [ "${CSC_IDENTITY_AUTO_DISCOVERY:-}" = "false" ] && [ -z "${CSC_NAME:-}" ]; then
  printf '%s\n' "-"
  exit 0
fi

identities=$(security find-identity -v -p codesigning 2>/dev/null || true)

pick() {
  prefix=$1
  printf '%s\n' "$identities" | sed -n "s/.*\"\\(${prefix}:[^\"]*\\)\".*/\\1/p"
}

name=$(pick "Developer ID Application" | head -1)
if [ -n "${CSC_NAME:-}" ]; then
  name=$(pick "Developer ID Application" | grep -F "$CSC_NAME" | head -1 || true)
fi

if [ -z "$name" ]; then
  name=$(pick "Mac Developer" | head -1)
  if [ -n "${CSC_NAME:-}" ]; then
    name=$(pick "Mac Developer" | grep -F "$CSC_NAME" | head -1 || true)
  fi
fi

if [ -n "$name" ]; then
  printf '%s\n' "$name"
else
  printf '%s\n' "-"
fi
