##[>] 🤖🤖
#[what] Monorepo Makefile: repo-wide targets, fans out to module Makefiles
SHELL := zsh
.SHELLFLAGS := -c
MODULES := che get-os-open-files-with get-term-open-files-with lib

WRAPPERS :=
COMMANDS := render-templates render-docs run-repo-ci-prepare-hooks run-repo-ci-precommit-all test test-cover build vet lint install create-tag publish release-check release-snapshot

.PHONY: $(WRAPPERS) $(COMMANDS)

##[>] Docs [genai-include]
#[what] render *.ontoRepo.tpl onto the repo (makefile.agents.md, repo-structure.md, CLAUDE.md, AGENTS.md, README.md) with this checkout's che build
render-templates:
	@$(MAKE) -C che build
	@che/dist/che render-templates

#[what] generate che docs (docs/cli.md, che.schema.json, cli-usage.md) from the Go source
render-docs:
	@$(MAKE) -C che render-docs
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
#[what] run all tests in every module
test:
	@for m in $(MODULES); do $(MAKE) -C $$m test || exit 1; done

#[what] run every module's tests with coverage, print each module's total percentage
test-cover:
	@for m in $(MODULES); do $(MAKE) -C $$m test-cover || exit 1; done

#[what] build every module's binaries into <module>/dist
build:
	@for m in $(MODULES); do $(MAKE) -C $$m build || exit 1; done

#[what] go vet every module
vet:
	@for m in $(MODULES); do $(MAKE) -C $$m vet || exit 1; done

#[what] golangci-lint every module
lint:
	@for m in $(MODULES); do $(MAKE) -C $$m lint || exit 1; done

#[what] install every module's binaries into $GOPATH/bin
install:
	@for m in $(MODULES); do $(MAKE) -C $$m install || exit 1; done
##[<] Go

##[>] Release [genai-include]
#[what] create the MODULE release + dir-prefixed git tag from BRANCH at REF via glab
#[args] MODULE BRANCH REF DEFAULT_BRANCH
create-tag:
	@ci/create-tag.zsh $(MODULE) $(BRANCH) $(REF) $(DEFAULT_BRANCH)

#[what] tag pipeline: goreleaser snapshot build for $CI_COMMIT_TAG's module, upload to the generic package registry, link release assets
publish:
	@ci/publish.zsh

#[what] validate every module's goreleaser configs
release-check:
	@for m in $(MODULES); do $(MAKE) -C $$m release-check || exit 1; done

#[what] local snapshot build of every module, no publish
release-snapshot:
	@for m in $(MODULES); do $(MAKE) -C $$m release-snapshot || exit 1; done
##[<] Release
##[<] 🤖🤖
