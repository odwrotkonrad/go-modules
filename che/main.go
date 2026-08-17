package main

// [>] 🤖

import (
	"fmt"
	"os"

	"gitlab.com/konradodwrot/go-modules/che/internal/cli"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

func main() {
	err := cli.New().Root().Execute()
	if err == nil {
		return
	}
	if code, ok := cli.CodedExit(err); ok {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(code)
	}
	fmt.Fprintln(os.Stderr, "che:", err)
	os.Exit(yamlcfg.ExitCode(err))
}

// [<] 🤖
