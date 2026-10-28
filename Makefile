MODULE  := github.com/hardhacker/hylo
BINARY  := hylo
CMD_DIR := ./cmd/hylo
CLIP_DIR := extensions/clip

# Clip extension zip (manifest version → dist/hylo-clip-v*.zip at repo root).
CLIP_VER := $(shell node -p "require('./$(CLIP_DIR)/manifest.json').version" 2>/dev/null || echo "0.0.0")
CLIP_ZIP := dist/hylo-clip-v$(CLIP_VER).zip

# Per-platform goreleaser config (auto-detected from current OS).
GORELEASER_CONFIG := .goreleaser-$(shell go env GOOS).yaml

# Build-time metadata injected via ldflags.
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Image goes to GHCR under this checkout's GitHub repo; GHCR names must be lowercase.
REPO_URL := $(shell git remote get-url origin 2>/dev/null | sed -E 's\#^git@github\.com:\#https://github.com/\#; s\#\.git$$\#\#')
IMAGE    ?= $(shell echo "$(REPO_URL)" | sed -E 's\#^https://github\.com/\#ghcr.io/\#' | tr A-Z a-z)

LDFLAGS := -s -w \
  -X $(MODULE)/internal/build.Version=$(VERSION) \
  -X $(MODULE)/internal/build.Commit=$(COMMIT) \
  -X $(MODULE)/internal/build.BuildDate=$(BUILD_DATE)

.PHONY: build test test-js icons editor desktop-ui dist-all dist-clean dist-cli-snapshot dist-cli-linux dist-docker dist-clip dist-dmg dist-checksum help

## build: compile the binary into ./bin/hylo
build:
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD_DIR)

## test: run all tests
test:
	go test -race -v ./...

## test-js: run the editor's and static assets' JS test suites, plus the editor's
## WebKit e2e (skipped if WebKit is missing: cd editor && npx playwright-core install webkit)
test-js:
	cd editor && npm test
	node --test "internal/server/view/assets/**/*.test.js"
	cd editor && npm run test:e2e

## icons: regenerate shared_icons_neo.go (pixelarticons) and shared_icons_line.go (view/icons/*.svg)
icons:
	cd editor && node gen-icons.mjs
	gofmt -w internal/server/view/shared_icons_line.go

## editor: bundle editor JS into internal/server/static/editor.js
editor:
	cd editor && npm run build

## desktop-ui: sync tokens, Alpine and fonts into the desktop shell page
desktop-ui:
	UPDATE_DESKTOP_TOKENS=1 go test ./internal/server/view -run TestDesktopShellTokens -count=1
	node desktop-tauri/scripts/sync-ui.mjs

## dist-all: build CLI archive, Clip extension zip, and the desktop DMG into ./dist, then checksum
dist-all: dist-clean dist-dmg dist-cli-linux dist-clip dist-checksum

## dist-clean: remove all previous dist artifacts before a fresh release build
dist-clean:
	rm -rf dist/ desktop-tauri/bundled/ desktop-tauri/src-tauri/target/release/bundle
	@mkdir -p dist

## dist-cli-snapshot: build hylo CLI for the current platform via goreleaser (no git tag needed)
dist-cli-snapshot:
	goreleaser release --snapshot --clean --config $(GORELEASER_CONFIG)

## dist-cli-linux: build hylo CLI for linux/amd64 via Docker into ./dist
dist-cli-linux:
	./scripts/build-linux.sh

## dist-docker: build the linux/amd64 server image with Pi, tagged for GHCR (push with docker push)
dist-docker:
	docker buildx build --platform linux/amd64 \
	  --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE) \
	  --label org.opencontainers.image.source=$(REPO_URL) \
	  -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

## dist-clip: build Clip browser extension and zip into ./dist
dist-clip:
	@mkdir -p dist
	cd $(CLIP_DIR) && npm ci && npm run build
	cd $(CLIP_DIR)/dist && zip -r "$(CURDIR)/$(CLIP_ZIP)" .

## dist-dmg: build the Tauri desktop DMG into ./dist (macOS only)
dist-dmg: dist-cli-snapshot
	@mkdir -p desktop-tauri/bundled
	@TAR_GZ=$$(find dist -maxdepth 1 -name "hylo_$$(go env GOOS)_$$(go env GOARCH).tar.gz" | head -1); \
	  test -n "$$TAR_GZ" || (echo "Error: dist/hylo_$$(go env GOOS)_$$(go env GOARCH).tar.gz not found"; exit 1); \
	  echo "==> Bundling $$TAR_GZ into Hylo.app..."; \
	  cp "$$TAR_GZ" desktop-tauri/bundled/hylo.tar.gz
	@IDENTITY=$$(sh desktop-tauri/scripts/signing-identity.sh); \
	  echo "==> Signing with $$IDENTITY; notarization is not configured"; \
	  cd desktop-tauri && APPLE_SIGNING_IDENTITY="$$IDENTITY" npm install && APPLE_SIGNING_IDENTITY="$$IDENTITY" npm run build -- --bundles dmg --config src-tauri/tauri.dist.macos.json
	@VER=$$(node -p "require('./desktop-tauri/src-tauri/tauri.conf.json').version"); \
	  DMG=$$(find desktop-tauri/src-tauri/target/release/bundle/dmg -name '*.dmg' | head -1); \
	  test -n "$$DMG" || (echo "Error: Tauri DMG not found"; exit 1); \
	  cp "$$DMG" "dist/Hylo-$$VER.dmg"
	@rm -rf desktop-tauri/bundled

## dist-checksum: generate SHA-256 checksums for all dist artifacts into dist/checksums.txt
dist-checksum:
	cd dist && shasum -a 256 $(shell ls dist/*.tar.gz dist/*.zip dist/*.dmg 2>/dev/null | xargs -n1 basename) > checksums.txt

## help: list targets
help:
	@grep -E '^##' Makefile | sed 's/## //'
