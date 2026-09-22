VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)
# The artifact is theia-server from V3.3 (decision 119). The published assets are
# theia-server-<os>-<arch>, and nothing here may go back to the old name: an
# installed v3.2 looks for that one, and the first V3.3 release answers it with a
# transitional copy rather than with this tree.
BINARY  := theia-server

# This machine's platform, in the words a release publishes: on macOS the asset
# is theia-server-darwin-arm64 and not theia-server, and the offline archive is
# assembled from dist/ under those names. Overridable, so a cross build can name
# the platform it is building for.
#
# Recursive (`=`) and not immediate (`:=`) on purpose: `go env` is then asked
# only by the target that needs the names, so `make clean` on a machine without
# Go on PATH does not report an error it cannot act on.
GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
EXT     = $(if $(filter windows,$(GOOS)),.exe,)

.PHONY: all web build test vet clean dist-programs

all: build

## web: compile the SvelteKit frontend into web-dist/
web:
	cd web && npm ci && npm run build
	@# The static adapter wipes web-dist/, including the placeholder that keeps
	@# the directory present in a fresh clone so //go:embed resolves.
	@git checkout -- web-dist/.gitkeep 2>/dev/null || touch web-dist/.gitkeep

## build: produce the single binary with the frontend embedded
build: web
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/theia-server

## dist-programs: the programs a release publishes, named for this platform
# The macOS offline archive is assembled from these and builds nothing itself
# (`.\build-release.ps1 -Version 3.3.4 -Target darwin-arm64`): it takes the
# server, the installer and the launcher out of dist/ under the names
# scripts/check-release-assets.ps1 accepts. So on a Mac this is the step that
# produces what that archive is made of, which is why the names are the published
# ones and not $(BINARY).
dist-programs: web
	mkdir -p dist
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/theia-server-$(GOOS)-$(GOARCH)$(EXT) ./cmd/theia-server
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/theia-setup-$(GOOS)-$(GOARCH)$(EXT) ./cmd/theia-setup
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/theia-launcher-$(GOOS)-$(GOARCH)$(EXT) ./cmd/theia

## test: run the Go test suite
test:
	go test ./...

## vet: static analysis
vet:
	go vet ./...

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf dist web/.svelte-kit web/node_modules
	find web-dist -mindepth 1 ! -name '.gitkeep' -delete 2>/dev/null || true
