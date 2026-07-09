package fsutil

// [>] 🤖🤖

import (
	"slices"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestMatchGlob(t *testing.T) {
	type in struct {
		Args []string
	}
	type c struct {
		Name string
		In   in
		Want bool
	}
	testyml.Run(t, td, "testdata/spec/is_glob_match.spec.yml", func(t *testing.T, c c) {
		if got := IsGlobMatch(c.In.Args[0], c.In.Args[1]); got != c.Want {
			t.Errorf("IsGlobMatch(%v) = %v, want %v", c.In.Args, got, c.Want)
		}
	})
}

func TestExpandBraces(t *testing.T) {
	type in struct {
		Args []string
	}
	type c struct {
		Name string
		In   in
		Want []string
	}
	testyml.Run(t, td, "testdata/spec/expand_braces.spec.yml", func(t *testing.T, c c) {
		if got := ExpandBraces(c.In.Args[0]); !slices.Equal(got, c.Want) {
			t.Errorf("ExpandBraces(%q) = %v, want %v", c.In.Args[0], got, c.Want)
		}
	})
}

// [<] 🤖🤖
