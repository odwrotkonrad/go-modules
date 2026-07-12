package checkcmd

// [>] 🤖🤖

import (
	"embed"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

//go:embed all:testdata
var td embed.FS

func TestToolRun(t *testing.T) {
	type in struct {
		Args     []string
		NeedsArg bool `yaml:"needsArg"`
	}
	type c struct {
		Name string
		In   in
		Want testyml.Want
	}
	testyml.Run(t, td, "testdata/spec/tool_run.spec.yml", func(t *testing.T, c c) {
		tool := Tool{Usage: "u\n", NeedsArg: c.In.NeedsArg, Generate: func(string) (string, error) { return "out", nil }}
		_, err := tool.Run(c.In.Args)
		c.Want.CheckCode(t, yamlcfg.Code(err))
	})
}

//[<] 🤖🤖
