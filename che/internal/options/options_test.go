package options

// [>] 🤖🤖

import (
	"embed"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func TestResolveBoolOr(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_bool_or.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		env := func(k string) string { return c.Context.Env[k] }
		var user, spec *bool
		c.Input.Args.To(t, 0, &user)
		c.Input.Args.To(t, 1, &spec)
		switch c.Context.Function {
		case "options.boolOr":
			return boolOr(env, false, "CHE_BOOL", user, spec), nil
		case "options.boolDefaultTrue":
			return boolDefaultTrue(env, "CHE_BOOL", user, spec), nil
		default:
			t.Fatalf("unknown function %q", c.Context.Function)
			return false, nil
		}
	})
}

// [<] 🤖🤖
