package main

// [>] 🤖

import (
	"fmt"
	"os"

	"gitlab.com/konradodwrot/go-modules/che/internal/cli"
)

func main() {
	if err := cli.New().Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "che:", err)
		os.Exit(1)
	}
}

// [<] 🤖
