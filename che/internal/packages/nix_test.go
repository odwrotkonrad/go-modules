package packages

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

type nixEntryGot struct {
	Found   bool   `yaml:"found"`
	Version string `yaml:"version,omitempty"`
}

func TestNixProfileEntry(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/nix_profile_entry.test.spec.yml", func(t *testing.T, c testyml.Case[nixEntryGot]) (nixEntryGot, error) {
		found, ver := findNixEntry(c.Input.Args.String(t, 0), c.Input.Args.String(t, 1))
		return nixEntryGot{Found: found, Version: ver}, nil
	})
}

// [<] 🤖🤖
