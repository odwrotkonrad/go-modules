package climain

// [>] 🤖🤖

import (
	"embed"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func TestHelpVersion(t *testing.T) {
	type want struct {
		Out  string
		Done bool
	}
	type in struct {
		Args []string
	}
	type c struct {
		Name string
		In   in
		Want want
	}
	testyml.Run(t, td, "testdata/spec/help_version.spec.yml", func(t *testing.T, c c) {
		out, done := HelpVersion(c.In.Args, "usage text", "tool", "1.2.3")
		if done != c.Want.Done || out != c.Want.Out {
			t.Fatalf("got (%q, %v) want (%q, %v)", out, done, c.Want.Out, c.Want.Done)
		}
	})
}

// [<] 🤖🤖
