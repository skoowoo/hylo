MODULE  := github.com/hardhacker/vaultr
BINARY  := vaultr
CMD_DIR := ./cmd/vaultr
CLIP_DIR := extensions/clip

# Clip extension zip (manifest version → dist/vaultr-clip-v*.zip at repo root).
CLIP_VER := $(shell node -p "require('./$(CLIP_DIR)/manifest.json').version" 2>/dev/null || echo "0.0.0")
CLIP_ZIP := dist/vaultr-clip-v$(CLIP_VER).zip

# Per-platform goreleaser config (auto-detected from current OS).
GORELEASER_CONFIG := .goreleaser-$(shell go env GOOS).yaml

# Build-time metadata injected via ldflags.
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -s -w \
  -X $(MODULE)/internal/build.Version=$(VERSION) \
  -X $(MODULE)/internal/build.Commit=$(COMMIT) \
  -X $(MODULE)/internal/build.BuildDate=$(BUILD_DATE)

.PHONY: build run serve lint test test-js clean tidy clip-zip icons editor dist-all dist-clean dist-cli dist-cli-snapshot dist-clip dist-dmg dist-win-app dist-checksum

## build: compile the binary into ./bin/vaultr
build:
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD_DIR)

## run: build and print version (smoke test)
run: build
	./bin/$(BINARY) version

## serve: build and start the HTTP server
serve: build
	./bin/$(BINARY) serve

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## test: run all tests
test:
	go test -race -v ./...

## test-js: run the editor's and static assets' JS test suites (node --test, no deps)
test-js:
	cd desktop-app/editor && npm test
	node --test "internal/server/view/assets/**/*.test.js"

## tidy: tidy and verify go modules
tidy:
	go mod tidy
	go mod verify

## icons: regenerate internal/server/view/shared_icons_pixel.go from pixelarticons
icons:
	cd desktop-app/editor && node gen-icons.mjs

## editor: bundle editor JS into internal/server/static/editor.js
editor:
	cd desktop-app/editor && npm run build

## clean: remove build artifacts
clean:
	rm -rf bin/

## dist-all: build CLI tar.gz, Clip extension zip, and Electron DMG into ./dist, then checksum
dist-all: dist-clean dist-dmg dist-clip dist-checksum

## dist-clean: remove all previous dist artifacts before a fresh release build
dist-clean:
	rm -rf dist/ desktop-app/dist/ desktop-app/bundled/
	@mkdir -p dist

## dist-cli: build vaultr CLI for the current platform only (via goreleaser, requires git tag)
dist-cli:
	goreleaser release --clean --config $(GORELEASER_CONFIG)

## dist-cli-snapshot: local test build without a git tag, current platform only
dist-cli-snapshot:
	goreleaser release --snapshot --clean --config $(GORELEASER_CONFIG)

## dist-clip: build Clip browser extension and zip into ./dist
dist-clip:
	@mkdir -p dist
	cd $(CLIP_DIR) && npm ci && npm run build
	cd $(CLIP_DIR)/dist && zip -r "$(CURDIR)/$(CLIP_ZIP)" .

## dist-dmg: build Electron desktop app DMG into ./dist (macOS only)
dist-dmg: dist-cli-snapshot
	@mkdir -p desktop-app/bundled
	@TAR_GZ=$$(find dist -maxdepth 1 -name "vaultr_$$(go env GOOS)_$$(go env GOARCH).tar.gz" | head -1); \
	  test -n "$$TAR_GZ" || (echo "Error: dist/vaultr_$$(go env GOOS)_$$(go env GOARCH).tar.gz not found"; exit 1); \
	  echo "==> Bundling $$TAR_GZ into Electron app..."; \
	  cp "$$TAR_GZ" desktop-app/bundled/vaultr.tar.gz
	rm -f desktop-app/dist/*.dmg desktop-app/dist/*.zip
	cd desktop-app && npm install && npm run dist -- --mac
	cp desktop-app/dist/*.dmg dist/
	@rm -rf desktop-app/bundled

## dist-win-app: build Electron desktop app NSIS installer into ./dist (Windows only)
dist-win-app: dist-cli-snapshot
	@mkdir -p desktop-app/bundled
	@ZIP=$$(find dist -maxdepth 1 -name "vaultr_$$(go env GOOS)_$$(go env GOARCH).zip" | head -1); \
	  test -n "$$ZIP" || (echo "Error: dist/vaultr_$$(go env GOOS)_$$(go env GOARCH).zip not found"; exit 1); \
	  echo "==> Bundling $$ZIP into Electron app..."; \
	  cp "$$ZIP" desktop-app/bundled/vaultr.zip
	cd desktop-app && npm install && npm run dist -- --win
	cp desktop-app/dist/*.exe dist/ 2>/dev/null || true
	@rm -rf desktop-app/bundled

## dist-checksum: generate SHA-256 checksums for all dist artifacts into dist/checksums.txt
dist-checksum:
	cd dist && shasum -a 256 $(shell ls dist/*.tar.gz dist/*.zip dist/*.dmg dist/*.exe 2>/dev/null | xargs -n1 basename) > checksums.txt

help:
	@grep -E '^##' Makefile | sed 's/## //'
