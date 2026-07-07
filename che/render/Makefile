##[>] 🤖🤖
#[what] Project's Makefile
SHELL := zsh
.SHELLFLAGS := -c
GO ?= go
CC ?= clang

COMMANDS := render-templates run-repo-ci-prepare-hooks run-repo-ci-precommit-all test build vet install

.PHONY: $(COMMANDS)

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
##[<] 🤖🤖
