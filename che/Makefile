##[>] 🤖🤖
#[what] Project's Makefile
SHELL := zsh
.SHELLFLAGS := -c
GO ?= go
CC ?= clang
BIN := che

WRAPPERS :=
COMMANDS := render-templates render-docs run-repo-ci-prepare-hooks run-repo-ci-precommit-all test build vet install create-tag release-check release-darwin release-snapshot

.PHONY: $(WRAPPERS) $(COMMANDS)

##[>] Docs [genai-include]
#[what] render *.ontoRepo.tpl onto the repo (makefile.agents.md, repo-structure.md, CLAUDE.md, AGENTS.md, README.md) with this checkout's build
render-templates: build
	@dist/$(BIN) render-templates

#[what] generate docs/cli.md + assets/data/{che.schema.json,cli-usage.md} from the Go source
render-docs:
	CC=$(CC) $(GO) run ./internal/docgen
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
#[what] create the release + git tag from BRANCH at REF via release-cli
#[args] BRANCH REF DEFAULT_BRANCH
create-tag:
	@ci/create-tag.zsh $(BRANCH) $(REF) $(DEFAULT_BRANCH)

#[what] validate both goreleaser configs
release-check:
	goreleaser check -f .goreleaser.yaml
	goreleaser check -f .goreleaser.darwin.yaml

#[what] build the darwin archives (native cc) into ./darwin-dist
release-darwin:
	goreleaser release --snapshot --clean -f .goreleaser.darwin.yaml

#[what] local 4-target snapshot: darwin (native) + linux (zig), no publish
release-snapshot: release-darwin
	goreleaser release --snapshot --clean -f .goreleaser.yaml
##[<] Release
##[<] 🤖🤖
