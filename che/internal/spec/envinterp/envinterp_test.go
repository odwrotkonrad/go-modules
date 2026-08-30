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
		var env, vars, builtins map[string]string
		c.Input.Args.To(t, 1, &env)
		c.Input.Args.To(t, 2, &vars)
		c.Input.Args.To(t, 3, &builtins)
		value, unset := Expand(c.Input.Args.String(t, 0), MapLookup(env, vars, builtins))
		var names []string
		for _, ref := range unset {
			names = append(names, refName(ref))
		}
		return expandWant{Value: value, Unset: names}, nil
	})
}

func refName(ref Ref) string {
	if ref.Namespace == Namespaces.Builtin {
		return ref.Name
	}
	return string(ref.Namespace) + "." + ref.Name
}

func TestRefs(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/refs.test.spec.yml", func(t *testing.T, c testyml.Case[[]Ref]) ([]Ref, error) {
		return Refs(c.Input.Args.String(t, 0)), nil
	})
}

// [<] 🤖🤖
