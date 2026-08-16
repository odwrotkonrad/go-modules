##[>] 🤖🤖
#[what] che module Makefile
SHELL := zsh
.SHELLFLAGS := -c
GO ?= go
GOCOVER_COBERTURA ?= github.com/t-yuki/gocover-cobertura@v0.0.0-20180217150009-aaee18c8195c
CC ?= clang
BIN := che
BUILD_DEP ?= build-cover
STATE_HOME := $(or $(CHE_STATE_HOME),$(if $(XDG_STATE_HOME),$(XDG_STATE_HOME)/che),$(HOME)/.local/state/che)
DB := $(STATE_HOME)/ops.db
BK_APPDB := $(HOME)/Library/Application Support/beekeeper-studio/app.db

WRAPPERS :=
COMMANDS := render-docs test test-cover cover-report e2e e2e-dryrun e2e-run e2e-registry e2e-install-methods-vm-template build build-cover build-e2e-test build-linux build-cover-linux build-e2e-test-linux vet lint install release-check release-darwin release-snapshot inspect-db dump-db open-db

.PHONY: $(WRAPPERS) $(COMMANDS)

E2E_GO_SRC := $(shell find . ../lib -name '*.go' -not -path './dist/*' -not -path './cover-*/*') go.mod go.sum ../go.work

dist/$(BIN): $(E2E_GO_SRC)
	CC=$(CC) $(GO) build -cover -coverpkg=./... -o dist/$(BIN) .

dist/e2e.test: $(E2E_GO_SRC)
	CC=$(CC) $(GO) test -c -o dist/e2e.test ./e2e

##[>] Docs [genai-include]
#[what] generate docs/cli.md + assets/data/{che.schema.json,cli-usage.md} from the Go source
render-docs:
	CC=$(CC) $(GO) run ./internal/docgen
##[<] Docs

##[>] Database [genai-include]
#[what] open the ops ledger DB (~/.local/state/che/ops.db) in an interactive sqlite3 shell (.tables/.schema, read-only)
inspect-db:
	sqlite3 -readonly "file:$(DB)?mode=ro" -header -column

#[what] dump the current installed set: every table's rows to stdout, read-only
dump-db:
	sqlite3 -readonly "file:$(DB)?mode=ro" -header -column \
		".mode box" \
		"SELECT * FROM spec_dones;" \
		"SELECT * FROM profile_dones;" \
		"SELECT * FROM operation_dones;" \
		"SELECT * FROM backups;"

#[what] register the ops ledger DB as a read-only Beekeeper saved connection (if absent), then launch Beekeeper Studio
open-db:
	osascript -e 'quit app "Beekeeper Studio"' 2>/dev/null || true
	sqlite3 "$(BK_APPDB)" "INSERT INTO saved_connection (version,connectionType,uniqueHash,name,defaultDatabase,options,readOnlyMode,workspaceId) SELECT 2,'sqlite','DEPRECATED','che ops ledger','$(DB)','{ \"connectionMethod\": \"manual\" }',1,-1 WHERE NOT EXISTS (SELECT 1 FROM saved_connection WHERE defaultDatabase='$(DB)' AND workspaceId=-1);"
	open -a "Beekeeper Studio"
##[<] Database

##[>] Go [genai-include]
#[what] run all tests
test:
	CC=$(CC) $(GO) test -v ./...

#[what] run all tests with coverage (binary covdata into ./cover-unit), print the per-package and total percentages
test-cover:
	if [[ -e cover-unit ]] rm -rf cover-unit; mkdir -p cover-unit
	CC=$(CC) $(GO) test -v -cover -coverpkg=./... ./... -args -test.gocoverdir=$(CURDIR)/cover-unit
	@$(GO) tool covdata textfmt -i=cover-unit -o coverage.out
	@sed -i.bak '\|/che/e2e/|d' coverage.out && rm coverage.out.bak
	@$(GO) run $(GOCOVER_COBERTURA) < coverage.out > coverage.xml
	@sed -i.bak 's|filename="gitlab.com/konradodwrot/go-modules/|filename="|g' coverage.xml && rm coverage.xml.bak
	@$(GO) tool cover -func=coverage.out | tail -1

#[what] merge every present cover-* covdata dir into coverage.out/coverage.xml, print the combined total
cover-report:
	@dirs=$$(print -l cover-unit(N) cover-e2e-dryrun(N) cover-e2e-run(N) cover-e2e-backup(N) cover-e2e-packages(N) cover-e2e-install-methods(N) | paste -sd, -); \
		[[ -n $$dirs ]] || { print "no cover-* covdata dirs"; exit 1 }; \
		print "merging covdata: $$dirs"; \
		$(GO) tool covdata textfmt -i=$$dirs -o coverage.out
	@sed -i.bak '\|/che/e2e/|d' coverage.out && rm coverage.out.bak
	@$(GO) run $(GOCOVER_COBERTURA) < coverage.out > coverage.xml
	@sed -i.bak 's|filename="gitlab.com/konradodwrot/go-modules/|filename="|g' coverage.xml && rm coverage.xml.bak
	@$(GO) tool cover -func=coverage.out | tail -1

#[what] build the coverage-instrumented binary into ./dist (e2e prerequisite, skipped when current)
build-cover: dist/$(BIN)

#[what] compile the e2e harness into ./dist/e2e.test (skipped when current)
build-e2e-test: dist/e2e.test

HOST_GOARCH := $(if $(filter arm64 aarch64,$(shell uname -m)),arm64,amd64)
E2E_LINUX_ARCH := $(or $(patsubst linux-%,%,$(filter linux-%,$(PLATFORM))),$(HOST_GOARCH))
ZIG_LINUX_TARGET := $(if $(filter arm64,$(E2E_LINUX_ARCH)),aarch64,x86_64)-linux-gnu
ZIG_CC := CC="zig cc -target $(ZIG_LINUX_TARGET)" CXX="zig c++ -target $(ZIG_LINUX_TARGET)"
#[what] cross-build the linux binary into ./dist/che-linux-<arch> (zig cc, arch from PLATFORM=linux-<arch>, default host arch), the linux-container binary
build-linux:
	GOOS=linux GOARCH=$(E2E_LINUX_ARCH) CGO_ENABLED=1 $(ZIG_CC) $(GO) build -o dist/che-linux-$(E2E_LINUX_ARCH) .

#[what] cross-build the coverage-instrumented binary for PLATFORM=linux-<arch> into ./dist/linux-<arch>/che (zig cc)
build-cover-linux:
	GOOS=linux GOARCH=$(E2E_LINUX_ARCH) CGO_ENABLED=1 $(ZIG_CC) $(GO) build -cover -coverpkg=./... -o dist/linux-$(E2E_LINUX_ARCH)/$(BIN) .

#[what] cross-compile the e2e harness for PLATFORM=linux-<arch> into ./dist/linux-<arch>/e2e.test (zig cc)
build-e2e-test-linux:
	GOOS=linux GOARCH=$(E2E_LINUX_ARCH) CGO_ENABLED=1 $(ZIG_CC) $(GO) test -c -o dist/linux-$(E2E_LINUX_ARCH)/e2e.test ./e2e

ifeq ($(E2E_USE_TEST_BIN),1)
E2E_RUN = sh -c 'cd e2e && exec ../dist/e2e.test -test.v -test.count=1 -test.timeout 45m -test.run "$$0"'
else
E2E_RUN = sh -c 'CC=$(CC) exec $(GO) test -v -count=1 -timeout 45m -run "$$0" ./e2e'
endif

#[what] dry-run e2e flow (every command --dry-run=all, CHE_LOG_LEVEL=trace), binary covdata into ./cover-e2e-dryrun
e2e-dryrun: $(BUILD_DEP)
	if [[ -e cover-e2e-dryrun ]] rm -rf cover-e2e-dryrun; mkdir -p cover-e2e-dryrun
	E2E_BIN=$(CURDIR)/dist/$(BIN) E2E_GOCOVERDIR=$(CURDIR)/cover-e2e-dryrun $(E2E_RUN) TestE2EDryRun

#[what] real full-flow e2e (CHE_LOG_LEVEL=info), binary covdata into ./cover-e2e-run
e2e-run: $(BUILD_DEP)
	if [[ -e cover-e2e-run ]] rm -rf cover-e2e-run; mkdir -p cover-e2e-run
	E2E_BIN=$(CURDIR)/dist/$(BIN) E2E_GOCOVERDIR=$(CURDIR)/cover-e2e-run $(E2E_RUN) TestE2ERun

#[what] backup e2e flow (create/ls/restore selectors, CHE_LOG_LEVEL=info), binary covdata into ./cover-e2e-backup
e2e-backup: $(BUILD_DEP)
	if [[ -e cover-e2e-backup ]] rm -rf cover-e2e-backup; mkdir -p cover-e2e-backup
	E2E_BIN=$(CURDIR)/dist/$(BIN) E2E_GOCOVERDIR=$(CURDIR)/cover-e2e-backup $(E2E_RUN) TestE2EBackup

#[what] packages e2e flow (install + checks against fake PATH managers, CHE_LOG_LEVEL=info), binary covdata into ./cover-e2e-packages
e2e-packages: $(BUILD_DEP)
	if [[ -e cover-e2e-packages ]] rm -rf cover-e2e-packages; mkdir -p cover-e2e-packages
	E2E_BIN=$(CURDIR)/dist/$(BIN) E2E_GOCOVERDIR=$(CURDIR)/cover-e2e-packages $(E2E_RUN) TestE2EPackages

E2E_INSTALL_SELECT := E2E_INSTALL_METHOD=$(or $(METHOD),$(E2E_INSTALL_METHOD),all) E2E_INSTALL_PACKAGE=$(or $(PACKAGE),$(E2E_INSTALL_PACKAGE)) E2E_INSTALL_MODE=$(or $(MODE),$(E2E_INSTALL_MODE)) E2E_INSTALL_PLATFORM=$(or $(PLATFORM),$(E2E_INSTALL_PLATFORM))
E2E_INSTALL_MODE_SEL := $(or $(MODE),$(E2E_INSTALL_MODE))
E2E_INSTALL_PLATFORM_SEL := $(or $(PLATFORM),$(E2E_INSTALL_PLATFORM))
E2E_TARGET_LINUX := $(or $(filter linux-%,$(E2E_INSTALL_PLATFORM_SEL)),$(if $(E2E_INSTALL_PLATFORM_SEL),,$(filter Linux,$(shell uname -s))))
E2E_BM_DARWIN := $(if $(CI)$(E2E_ALLOW_HOST),,$(if $(filter Darwin,$(shell uname -s)),$(if $(filter 1,$(shell sysctl -n kern.hv_vmm_present 2>/dev/null)),,1)))
E2E_WITH_DEPS := $(filter with_deps,$(E2E_INSTALL_MODE_SEL))
ifneq ($(and $(E2E_BM_DARWIN),$(E2E_WITH_DEPS),$(if $(E2E_TARGET_LINUX),,1)),)
E2E_INSTALL_METHODS_DEPS := build-e2e-test
E2E_INSTALL_METHODS_RUN = $(E2E_INSTALL_SELECT) ../ci/e2e-install-methods-vm.zsh
else
ifneq ($(and $(if $(E2E_WITH_DEPS),,1),$(E2E_TARGET_LINUX)),)
E2E_INSTALL_METHODS_DEPS := build-linux
E2E_INSTALL_METHODS_ENV := E2E_BIN_LINUX=$(CURDIR)/dist/che-linux-$(E2E_LINUX_ARCH)
endif
ifneq ($(and $(E2E_BM_DARWIN),$(if $(E2E_WITH_DEPS),,1),$(if $(E2E_TARGET_LINUX),,1)),)
E2E_INSTALL_METHODS_DEPS := e2e-install-methods-vm-template
endif
E2E_INSTALL_METHODS_RUN = E2E_BIN=$(CURDIR)/dist/$(BIN) E2E_GOCOVERDIR=$(CURDIR)/cover-e2e-install-methods $(E2E_INSTALL_SELECT) $(E2E_INSTALL_METHODS_ENV) $(E2E_RUN) TestE2EInstallMethods
endif
#[what] ensure the darwin no-deps tart VM template exists (clone vanilla base, bootstrap ssh key, stop)
e2e-install-methods-vm-template:
	../ci/e2e-install-methods-vm-template.zsh
#[what] real-install e2e, see spec/vetted_title_only/che/e2e-install-methods.md: each selected package installs live once per eligible entry method, then runs; MODE=with_no_deps (default) gives each install a fresh debian container (linux) or tart VM (darwin), MODE=with_deps a throwaway HOME (bare-metal darwin delegates the run into a tart VM)
#[vals] METHOD: <method>|<method>/ prefix|all; PACKAGE: <pkg>[,<pkg>...]|all; MODE: with_no_deps|with_deps; PLATFORM: darwin-arm64|linux-arm64|linux-amd64; E2E_ALLOW_HOST: 1; E2E_INSTALL_PACKAGES_PER_METHOD: <n>; CHE_LOG_LEVEL: info (default)|debug|trace
e2e-install-methods: $(BUILD_DEP) $(E2E_INSTALL_METHODS_DEPS)
	if [[ -e cover-e2e-install-methods ]] rm -rf cover-e2e-install-methods; mkdir -p cover-e2e-install-methods
	$(E2E_INSTALL_METHODS_RUN)

#[what] registry e2e: every brew formula and cask name in the builtin packages file resolves upstream
e2e-registry:
	E2E_REGISTRY=1 $(E2E_RUN) TestE2ERegistry

#[what] run all e2e flows
e2e: e2e-dryrun e2e-run e2e-backup e2e-packages

#[what] build the binary into ./dist
build:
	CC=$(CC) $(GO) build -v -o dist/$(BIN) .

#[what] golangci-lint all packages
lint:
	CC=$(CC) golangci-lint run

#[what] go vet all packages
vet:
	CC=$(CC) $(GO) vet -v ./...

#[what] install the binary into $GOPATH/bin
install:
	CC=$(CC) $(GO) install -v -ldflags="-s -w" .
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
