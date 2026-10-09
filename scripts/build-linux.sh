#!/usr/bin/env bash
# Build a static (musl) hylo for linux/amd64 inside Docker; runs on any distro.
set -euo pipefail

cd "$(dirname "$0")/.."

GO_VERSION="$(awk '/^go /{print $2; exit}' go.mod)"
IMAGE="${GO_IMAGE:-golang:${GO_VERSION%.*}-alpine}"
MODULE="github.com/hardhacker/hylo"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

mkdir -p dist
rm -rf dist/.linux-stage dist/hylo_linux_amd64.tar.gz

docker run --rm --platform linux/amd64 \
  -v "$PWD":/src -w /src \
  -v hylo-go-mod:/go/pkg/mod \
  -v hylo-go-build:/root/.cache/go-build \
  -e CGO_ENABLED=1 \
  "$IMAGE" sh -c "
    set -eu
    apk add --no-cache g++ musl-dev git >/dev/null
    mkdir -p dist/.linux-stage
    go build -trimpath -buildvcs=false \
      -ldflags '-s -w -linkmode external -extldflags \"-static\" \
        -X $MODULE/internal/build.Version=$VERSION \
        -X $MODULE/internal/build.Commit=$COMMIT \
        -X $MODULE/internal/build.BuildDate=$BUILD_DATE' \
      -o dist/.linux-stage/hylo ./cmd/hylo
  "

cp config.example.toml dist/.linux-stage/
cp -R skills dist/.linux-stage/skills
tar -C dist/.linux-stage -czf dist/hylo_linux_amd64.tar.gz .
rm -rf dist/.linux-stage
echo "==> dist/hylo_linux_amd64.tar.gz"
