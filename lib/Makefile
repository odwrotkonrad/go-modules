##[>] 🤖🤖
#[what] lib module Makefile: shared library, no binaries
SHELL := zsh
.SHELLFLAGS := -c
GO ?= go

WRAPPERS :=
COMMANDS := test build vet install release-check release-snapshot

.PHONY: $(WRAPPERS) $(COMMANDS)

##[>] Go [genai-include]
#[what] run all tests
test:
	$(GO) test ./...

#[what] compile all packages (library, no binaries)
build:
	$(GO) build ./...

#[what] go vet all packages
vet:
	$(GO) vet ./...

#[what] no-op: library module, nothing to install
install:
##[<] Go

##[>] Release [genai-include]
#[what] no-op: library module, no goreleaser config
release-check:

#[what] no-op: library module, no release artifacts
release-snapshot:
##[<] Release
##[<] 🤖🤖
