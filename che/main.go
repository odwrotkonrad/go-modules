package main

// [>] 🤖

import (
	"fmt"
	"os"

	"gitlab.com/konradodwrot/go/che/internal/cli"
)

func init() {
	cli.RootCmd.AddCommand(
		cli.LinkCmd,
		cli.CopyCmd,
		cli.RenderCmd,
		cli.DirsCmd,
		cli.PruneCmd,
		cli.RunScriptsCmd,
		cli.DetectCmd,
		cli.ServicesCmd,
	)
}

func main() {
	if err := cli.RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "che:", err)
		os.Exit(1)
	}
}

// [<] 🤖
