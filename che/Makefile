##[>] 🤖🤖
#[what] Project's Makefile
SHELL := zsh
.SHELLFLAGS := -c
GO ?= go
CC ?= clang
BIN := che

COMMANDS := test build vet install create-tag

.PHONY: $(COMMANDS)

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
##[<] Release
##[<] 🤖🤖
