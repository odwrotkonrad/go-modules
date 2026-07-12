// Che loads declarative host configuration: resolves eligible che.yml profiles and applies their links, copies, renders, dirs, scripts, services.
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
