package main

// [>] 🤖

import (
	"fmt"
	"os"

	"gitlab.com/konradodwrot/go/che/internal/cli"
)

func main() {
	if err := cli.Attach().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "che:", err)
		os.Exit(1)
	}
}

// [<] 🤖
