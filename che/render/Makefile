##[>] 🤖🤖
SHELL := /bin/sh
GO ?= go
CC ?= clang

.PHONY: test build vet install

##[>] Targets [genai-include]
#[what] run all tests
test:
	CC=$(CC) $(GO) test ./...

#[what] build both binaries into ./dist
build:
	CC=$(CC) $(GO) build -o dist/ ./cmd/...

#[what] go vet all packages
vet:
	CC=$(CC) $(GO) vet ./...

#[what] install both binaries into $GOPATH/bin
install:
	CC=$(CC) $(GO) install ./cmd/...
##[<] Targets
##[<] 🤖🤖
