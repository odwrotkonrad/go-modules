package main

// [>] 🤖🤖

import (
	"embed"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func TestRun(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/cmds/get-os-open-files-with.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		raw := ""
		if cfg := c.Input.Args.String(t, 0); cfg != "" {
			raw = testyml.ReadFile(t, td, cfg)
		}
		out, err := run(c.Context.CommandArgs(), testyml.ConfigDir(t, configName, raw))
		if c.Expected.Check(t, err) {
			return
		}
		testyml.EqualExpected(t, td, c.Expected.Files, out)
	})
}

func TestHelpVersion(t *testing.T) {
	testyml.RunHelpVersionSpec(t, td, "testdata/spec/funcs/help_version.test.spec.yml", usage, "get-os-open-files-with", version)
}

//[<] 🤖🤖
