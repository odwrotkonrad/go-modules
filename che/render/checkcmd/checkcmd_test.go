package checkcmd

// [>] 🤖🤖

import (
	"embed"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func TestToolRun(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/cmds/tool_run.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		tool := Tool{Usage: "u\n", NeedsArg: c.Input.Args.Bool(t, 0), Generate: func(string) (string, error) { return "out", nil }}
		_, err := tool.Run(c.Context.CommandArgs())
		c.Expected.Check(t, err)
	})
}

//[<] 🤖🤖
