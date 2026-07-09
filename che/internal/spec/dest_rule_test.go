package spec

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestDestRule(t *testing.T) {
	type in struct {
		Rule string
		Path string
	}
	type want struct {
		testyml.Want `yaml:",inline"`
		Value        string
	}
	type c struct {
		Name string
		In   in
		Want want
	}
	testyml.Run(t, td, "testdata/spec/dest_rule.spec.yml", func(t *testing.T, c c) {
		rule, err := parseDestRule(c.In.Rule)
		if c.Want.IsErrorWanted() {
			c.Want.CheckErr(t, err)
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if got := rule.apply(c.In.Path); got != c.Want.Value {
			t.Errorf("apply(%q, %q) = %q, want %q", c.In.Rule, c.In.Path, got, c.Want.Value)
		}
	})
}

// [<] 🤖🤖
