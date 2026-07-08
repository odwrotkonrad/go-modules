##[>] 🤖🤖
#[what] Project's Makefile
SHELL := zsh
.SHELLFLAGS := -c
GO ?= go
CC ?= clang

WRAPPERS :=
COMMANDS := render-templates run-repo-ci-prepare-hooks run-repo-ci-precommit-all test build vet install release-check release-darwin release-snapshot

.PHONY: $(WRAPPERS) $(COMMANDS)

##[>] Docs [genai-include]
#[what] render *.ontoRepo.tpl onto the repo (makefile.agents.md, repo-structure.md, CLAUDE.md, AGENTS.md, README.md)
render-templates:
	@che render-templates --profile=ontoRepo
##[<] Docs

##[>] CI [genai-include]
#[what] install lefthook git hooks
run-repo-ci-prepare-hooks:
	@lefthook install --force

#[what] run pre-commit hooks over all files (not just staged)
run-repo-ci-precommit-all: run-repo-ci-prepare-hooks
	@lefthook run pre-commit --all-files --force
##[<] CI

##[>] Go [genai-include]
#[what] run all tests
test:
	CC=$(CC) $(GO) test ./...

#[what] build all binaries into ./dist
build:
	CC=$(CC) $(GO) build -o dist/ ./cmd/...

#[what] go vet all packages
vet:
	CC=$(CC) $(GO) vet ./...

#[what] install all binaries into $GOPATH/bin
install:
	CC=$(CC) $(GO) install ./cmd/...
##[<] Go

##[>] Release [genai-include]
#[what] validate both goreleaser configs
release-check:
	goreleaser check -f .goreleaser.yaml
	goreleaser check -f .goreleaser.darwin.yaml

#[what] build the darwin archives (native cc) into ./darwin-dist
release-darwin:
	goreleaser release --snapshot --clean -f .goreleaser.darwin.yaml

#[what] local 12-archive snapshot: darwin (native) + linux (zig), no publish
release-snapshot: release-darwin
	goreleaser release --snapshot --clean -f .goreleaser.yaml
##[<] Release
##[<] 🤖🤖
