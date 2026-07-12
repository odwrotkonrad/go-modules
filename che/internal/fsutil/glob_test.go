package fsutil

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestMatchGlob(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/is_glob_match.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		return IsGlobMatch(c.Input.Args.String(t, 0), c.Input.Args.String(t, 1)), nil
	})
}

func TestExpandBraces(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/expand_braces.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) ([]string, error) {
		return ExpandBraces(c.Input.Args.String(t, 0)), nil
	})
}

// [<] 🤖🤖
