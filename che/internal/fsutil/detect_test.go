package fsutil

// [>] 🤖🤖

import (
	"os/user"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestNormalizeOS(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/normalize_os.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		return NormalizeOS(c.Input.Args.String(t, 0)), nil
	})
}

func TestIsVirtualized(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/is_virtualized.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		m := testutil.ApplyMocks(t, c.Context.MockedInterfaces)
		a := c.Input.Args
		m.Exec.Out = a.String(t, 1)
		m.Exec.Fail = a.Bool(t, 2)
		a.To(t, 3, &m.Reader.Files)
		return isVirtualized(a.String(t, 0), m.Exec, m.Reader), nil
	})
}

func TestUserHome(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/user_home.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		testutil.RequireRegistered(t, c.Context.MockedInterfaces)
		var homes map[string]string
		c.Input.Args.To(t, 1, &homes)
		users := testutil.UserMockLookup{}
		for name, home := range homes {
			users[name] = user.User{HomeDir: home}
		}
		testyml.Swap(t, &UserLookup, users.Lookup)
		return ResolveUserHome(c.Input.Args.String(t, 0))
	})
}

// [<] 🤖🤖
