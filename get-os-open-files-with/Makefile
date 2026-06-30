##[>] 🤖🤖
SHELL := /bin/sh
GO ?= go
BIN := get-os-open-files-with

.PHONY: test build vet install release-snapshot release

##[>] Targets [genai-include]
#[what] run all tests
test:
	$(GO) test ./...

#[what] build the binary into ./dist
build:
	$(GO) build -o dist/$(BIN) .

#[what] go vet all packages
vet:
	$(GO) vet ./...

#[what] install the binary into $GOPATH/bin
install:
	$(GO) install .

#[what] build a local snapshot release (no publish)
release-snapshot:
	goreleaser release --snapshot --clean

#[what] cut a release from the current tag (publish)
release:
	goreleaser release --clean
##[<] Targets
##[<] 🤖🤖
