##[>] 🤖🤖
#[what] che module Makefile
SHELL := zsh
.SHELLFLAGS := -c
GO ?= go
GOCOVER_COBERTURA ?= github.com/t-yuki/gocover-cobertura@v0.0.0-20180217150009-aaee18c8195c
CC ?= clang
BIN := che

WRAPPERS :=
COMMANDS := render-docs test test-cover build vet install release-check release-darwin release-snapshot

.PHONY: $(WRAPPERS) $(COMMANDS)

##[>] Docs [genai-include]
#[what] generate docs/cli.md + assets/data/{che.schema.json,cli-usage.md} from the Go source
render-docs:
	CC=$(CC) $(GO) run ./internal/docgen
##[<] Docs

##[>] Go [genai-include]
#[what] run all tests
test:
	CC=$(CC) $(GO) test ./...

#[what] run all tests with coverage, print the per-package and total percentages
test-cover:
	CC=$(CC) $(GO) test -coverprofile=coverage.out -coverpkg=./... ./...
	$(GO) tool cover -func=coverage.out | tail -1
	$(GO) run $(GOCOVER_COBERTURA) < coverage.out > coverage.xml

#[what] build the binary into ./dist via go build
build:
	CC=$(CC) $(GO) build -o dist/$(BIN) .

#[what] go vet all packages
vet:
	CC=$(CC) $(GO) vet ./...

#[what] install the binary into $GOPATH/bin
install:
	CC=$(CC) $(GO) install .
##[<] Go

##[>] Release [genai-include]
#[what] validate both goreleaser configs
release-check:
	goreleaser check -f .goreleaser.yaml
	goreleaser check -f .goreleaser.darwin.yaml

#[what] build the darwin archives (native cc) into ./darwin-dist
release-darwin:
	goreleaser release --snapshot --clean -f .goreleaser.darwin.yaml

#[what] local snapshot of all module binaries: darwin (native) + linux (zig), no publish
release-snapshot: release-darwin
	goreleaser release --snapshot --clean -f .goreleaser.yaml
##[<] Release
##[<] 🤖🤖
