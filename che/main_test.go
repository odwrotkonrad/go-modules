package main

// [>] 🤖

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/konradodwrot/go-modules/che/internal/cli"
)

// cli.New().Root() wires every subcommand to root.
func TestSubcommandsWired(t *testing.T) {
	var got []string
	for _, c := range cli.New().Root().Commands() {
		got = append(got, c.Name())
	}
	for _, want := range []string{
		"link", "copy", "render-templates", "mk-dirs",
		"prune-links", "run-scripts", "detect", "services",
	} {
		assert.Contains(t, got, want, "subcommand not wired to RootCmd")
	}
}

// [<] 🤖
