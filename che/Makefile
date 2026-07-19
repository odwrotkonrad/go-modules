##[>] 🤖🤖
#[what] che module Makefile
SHELL := zsh
.SHELLFLAGS := -c
GO ?= go
GOCOVER_COBERTURA ?= github.com/t-yuki/gocover-cobertura@v0.0.0-20180217150009-aaee18c8195c
CC ?= clang
BIN := che
STATE_HOME := $(or $(CHE_STATE_HOME),$(if $(XDG_STATE_HOME),$(XDG_STATE_HOME)/che),$(HOME)/.local/state/che)
DB := $(STATE_HOME)/ops.db
BK_APPDB := $(HOME)/Library/Application Support/beekeeper-studio/app.db

WRAPPERS :=
COMMANDS := render-docs test test-cover e2e build vet lint install release-check release-darwin release-snapshot inspect-db dump-db open-db

.PHONY: $(WRAPPERS) $(COMMANDS)

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

#[what] run all tests with coverage, print the per-package and total percentages
test-cover:
	CC=$(CC) $(GO) test -v -coverprofile=coverage.out -coverpkg=./... ./...
	@$(GO) run $(GOCOVER_COBERTURA) < coverage.out > coverage.xml
	@sed -i.bak 's|filename="gitlab.com/konradodwrot/go-modules/|filename="|g' coverage.xml && rm coverage.xml.bak
	@$(GO) tool cover -func=coverage.out | tail -1

#[what] run the real binary through the full command flow over the e2e fixture (hermetic temp HOME)
e2e: build
	zsh e2e/run.zsh

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
	CC=$(CC) $(GO) install -v .
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
