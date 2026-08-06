package climain

// [>] 🤖🤖

import (
	"embed"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

type helpVersionOut struct {
	Out  string `yaml:"out"`
	Done bool   `yaml:"done"`
}

func TestHelpVersion(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/help_version.test.spec.yml", func(t *testing.T, c testyml.Case[helpVersionOut]) (helpVersionOut, error) {
		out, done := HelpVersion(c.Input.Args.Strings(t, 0), "usage text", "tool", "1.2.3")
		return helpVersionOut{Out: out, Done: done}, nil
	})
}

// [<] 🤖🤖
