package fsutil

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestNormalizeOS(t *testing.T) {
	type in struct {
		Args []string
	}
	type c struct {
		Name string
		In   in
		Want string
	}
	testyml.Run(t, td, "testdata/spec/normalize_os.spec.yml", func(t *testing.T, c c) {
		if got := NormalizeOS(c.In.Args[0]); got != c.Want {
			t.Errorf("NormalizeOS(%q) = %q, want %q", c.In.Args[0], got, c.Want)
		}
	})
}

// [<] 🤖🤖
