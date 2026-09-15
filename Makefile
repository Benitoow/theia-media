VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)
# The artifact is theia-server from V3.3 (decision 119). The published assets are
# theia-server-<os>-<arch>, and nothing here may go back to the old name: an
# installed v3.2 looks for that one, and the first V3.3 release answers it with a
# transitional copy rather than with this tree.
BINARY  := theia-server

.PHONY: all web build test vet clean

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
