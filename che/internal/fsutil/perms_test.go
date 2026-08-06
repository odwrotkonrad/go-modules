package fsutil

// [>] 🤖🤖

import (
	"io/fs"
	"os/user"
	"strconv"
	"syscall"
	"testing"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestPrependEnvVar(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/prepend_env_var.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) ([]string, error) {
		a := c.Input.Args
		return PrependEnvVar(a.Strings(t, 0), a.String(t, 1), a.String(t, 2)), nil
	})
}

// fakeInfo is an os.FileInfo whose Sys() is test-crafted (a *syscall.Stat_t or nil).
type fakeInfo struct{ sys any }

func (f fakeInfo) Name() string       { return "f" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return 0o644 }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return f.sys }

// swapLookups replaces the passwd/group lookups with the map-served mocks so
// the live user db never leaks into results.
func swapLookups(t *testing.T, users, groups map[string]int) {
	t.Helper()
	u := testutil.UserMockLookup{}
	for name, id := range users {
		u[name] = user.User{Uid: strconv.Itoa(id)}
	}
	g := testutil.GroupMockLookup{}
	for name, id := range groups {
		g[name] = user.Group{Gid: strconv.Itoa(id)}
	}
	testyml.Swap(t, &UserLookup, u.Lookup)
	testyml.Swap(t, &GroupLookup, g.Lookup)
}

func TestIsOwnerDrifted(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/is_owner_drifted.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		testutil.RequireRegistered(t, c.Context.MockedInterfaces)
		a := c.Input.Args
		var users, groups map[string]int
		a.To(t, 1, &users)
		a.To(t, 2, &groups)
		swapLookups(t, users, groups)
		var sys any
		if !a.Bool(t, 5) {
			sys = &syscall.Stat_t{Uid: uint32(a.Int(t, 3)), Gid: uint32(a.Int(t, 4))}
		}
		return IsOwnerDrifted(fakeInfo{sys}, a.String(t, 0)), nil
	})
}

// [<] 🤖🤖
