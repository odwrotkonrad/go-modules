package envinterp

// [>] 🤖🤖

import (
	"embed"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

type expandWant struct {
	Value string   `yaml:"value"`
	Unset []string `yaml:"unset"`
}

func TestExpand(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/expand.test.spec.yml", func(t *testing.T, c testyml.Case[expandWant]) (expandWant, error) {
		var env map[string]string
		c.Input.Args.To(t, 1, &env)
		value, unset := Expand(c.Input.Args.String(t, 0), func(k string) string { return env[k] })
		return expandWant{Value: value, Unset: unset}, nil
	})
}

func TestRefs(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/refs.test.spec.yml", func(t *testing.T, c testyml.Case[[]Ref]) ([]Ref, error) {
		return Refs(c.Input.Args.String(t, 0)), nil
	})
}

// [<] 🤖🤖
