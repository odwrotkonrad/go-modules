##[>] 🤖🤖
#[what] lib module Makefile: shared library, no binaries
SHELL := zsh
.SHELLFLAGS := -c
GO ?= go
GOCOVER_COBERTURA ?= github.com/t-yuki/gocover-cobertura@v0.0.0-20180217150009-aaee18c8195c

WRAPPERS :=
COMMANDS := test test-cover build vet lint install release-check release-snapshot

.PHONY: $(WRAPPERS) $(COMMANDS)

##[>] Go [genai-include]
#[what] run all tests
test:
	$(GO) test -v ./...

#[what] run all tests with coverage, print the per-package and total percentages
test-cover:
	$(GO) test -v -coverprofile=coverage.out -coverpkg=./... ./...
	@$(GO) run $(GOCOVER_COBERTURA) < coverage.out > coverage.xml
	@sed -i.bak 's|filename="gitlab.com/konradodwrot/go-modules/|filename="|g' coverage.xml && rm coverage.xml.bak
	@$(GO) tool cover -func=coverage.out | tail -1

#[what] compile all packages (library, no binaries)
build:
	$(GO) build -v ./...

#[what] golangci-lint all packages
lint:
	golangci-lint run

#[what] go vet all packages
vet:
	$(GO) vet -v ./...

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
