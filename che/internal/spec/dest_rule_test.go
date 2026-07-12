package spec

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestDestRule(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/parse_dest_rule.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		rule, err := parseDestRule(c.Input.Args.String(t, 0))
		if err != nil {
			return "", err
		}
		return rule.apply(c.Input.Args.String(t, 1)), nil
	})
}

// [<] 🤖🤖
