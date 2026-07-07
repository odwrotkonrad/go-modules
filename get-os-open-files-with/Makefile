##[>] 🤖🤖
#[what] Project's Makefile
SHELL := zsh
.SHELLFLAGS := -c
GO ?= go
BIN := get-os-open-files-with

WRAPPERS :=
COMMANDS := render-templates run-repo-ci-prepare-hooks run-repo-ci-precommit-all test build vet install release-check release-snapshot

.PHONY: $(WRAPPERS) $(COMMANDS)

##[>] Docs [genai-include]
#[what] render *.repo.tpl onto the repo (makefile.agents.md, repo-structure.md, CLAUDE.md, AGENTS.md, README.md)
render-templates:
	@che render-templates --repo
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
