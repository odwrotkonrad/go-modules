##[>] 🤖🤖
SHELL := zsh
GO ?= go
CC ?= clang
BIN := che

.PHONY: test build vet install create-tag

##[>] Targets [genai-include]
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

#[what] create the release + git tag for BRANCH at REF via release-cli
create-tag:
	@ci/create-tag.zsh "$(BRANCH)" "$(REF)"
##[<] Targets
##[<] 🤖🤖
