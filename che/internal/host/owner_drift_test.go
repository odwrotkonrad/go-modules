package host

// [>] 🤖🤖

import (
	"io/fs"
	"os/user"
	"strconv"
	"syscall"
	"testing"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// fakeInfo is an os.FileInfo whose Sys() is test-crafted (a *syscall.Stat_t or nil).
type fakeInfo struct{ sys any }

func (f fakeInfo) Name() string       { return "f" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return 0o644 }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return f.sys }

// swapLookups replaces the passwd/group lookups with map-backed fakes so the
// live user db never leaks into results.
func swapLookups(t *testing.T, users, groups map[string]int) {
	t.Helper()
	prevU, prevG := fsutil.UserLookup, fsutil.GroupLookup
	fsutil.UserLookup = func(name string) (*user.User, error) {
		id, ok := users[name]
		if !ok {
			return nil, user.UnknownUserError(name)
		}
		return &user.User{Uid: strconv.Itoa(id)}, nil
	}
	fsutil.GroupLookup = func(name string) (*user.Group, error) {
		id, ok := groups[name]
		if !ok {
			return nil, user.UnknownGroupError(name)
		}
		return &user.Group{Gid: strconv.Itoa(id)}, nil
	}
	t.Cleanup(func() { fsutil.UserLookup, fsutil.GroupLookup = prevU, prevG })
}

func TestIsOwnerDrifted(t *testing.T) {
	type in struct {
		Uid    uint32 `yaml:"uid"`
		Gid    uint32 `yaml:"gid"`
		Owner  string
		NoStat bool `yaml:"noStat"`
		Users  map[string]int
		Groups map[string]int
	}
	type c struct {
		Name string
		In   in
		Want bool
	}
	testyml.Run(t, td, "testdata/spec/unit/owner_drift.spec.yml", func(t *testing.T, c c) {
		swapLookups(t, c.In.Users, c.In.Groups)
		var sys any
		if !c.In.NoStat {
			sys = &syscall.Stat_t{Uid: c.In.Uid, Gid: c.In.Gid}
		}
		if got := isOwnerDrifted(fakeInfo{sys}, c.In.Owner); got != c.Want {
			t.Errorf("isOwnerDrifted(uid=%d gid=%d, %q) = %v, want %v",
				c.In.Uid, c.In.Gid, c.In.Owner, got, c.Want)
		}
	})
}

// [<] 🤖🤖
