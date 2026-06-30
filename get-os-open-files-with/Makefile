##[>] 🤖🤖
SHELL := /bin/sh
GO ?= go
BIN := get-os-open-files-with

.PHONY: test build vet install

##[>] Targets [genai-include]
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
##[<] Targets
##[<] 🤖🤖
