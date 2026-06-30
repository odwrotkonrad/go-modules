package main

// [>] 🤖

import (
	"slices"
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/cli"
)

// init() wires every subcommand to root.
func TestSubcommandsWired(t *testing.T) {
	var got []string
	for _, c := range cli.RootCmd.Commands() {
		got = append(got, c.Name())
	}
	for _, want := range []string{
		"link", "copy", "render-templates", "mk-dirs",
		"prune-links", "run-scripts", "detect", "services",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("subcommand %q not wired to RootCmd; have %v", want, got)
		}
	}
}

// [<] 🤖
