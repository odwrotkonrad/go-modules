package main

// [>] 🤖

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/konradodwrot/go-modules/che/internal/cli"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

// cli.New().Root() wires every subcommand to root.
func TestSubcommandsWired(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/subcommands.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) {
		var got []string
		for _, cmd := range cli.New().Root().Commands() {
			got = append(got, cmd.Name())
		}
		for _, want := range c.Expected.Output {
			assert.Contains(t, got, want, "subcommand not wired to RootCmd")
		}
	})
}

// [<] 🤖
