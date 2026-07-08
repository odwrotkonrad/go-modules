##[>] 🤖🤖
#[what] get-os-open-files-with module Makefile
SHELL := zsh
.SHELLFLAGS := -c
GO ?= go
BIN := get-os-open-files-with

WRAPPERS :=
COMMANDS := test build vet install release-check release-snapshot

.PHONY: $(WRAPPERS) $(COMMANDS)

##[>] Go [genai-include]
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
##[<] Go

##[>] Release [genai-include]
#[what] validate the goreleaser config
release-check:
	goreleaser check

#[what] local 4-target snapshot into ./dist, no publish
release-snapshot:
	goreleaser release --snapshot --clean
##[<] Release
##[<] 🤖🤖
