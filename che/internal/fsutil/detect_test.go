package fsutil

// [>] 🤖🤖

import (
	"errors"
	"io/fs"
	"os"
	"os/user"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestNormalizeOS(t *testing.T) {
	type in struct {
		Args []string
	}
	type c struct {
		Name string
		In   in
		Want string
	}
	testyml.Run(t, td, "testdata/spec/normalize_os.spec.yml", func(t *testing.T, c c) {
		if got := NormalizeOS(c.In.Args[0]); got != c.Want {
			t.Errorf("NormalizeOS(%q) = %q, want %q", c.In.Args[0], got, c.Want)
		}
	})
}

// mapReader is a FileSystemReader backed by a path->content map: Stat hits
// mapped paths, ReadFile returns their content, everything else fails.
type mapReader struct{ files map[string]string }

func (r mapReader) Stat(p string) (os.FileInfo, error) {
	if _, ok := r.files[p]; ok {
		return nil, nil
	}
	return nil, fs.ErrNotExist
}

func (r mapReader) ReadFile(p string) ([]byte, error) {
	if b, ok := r.files[p]; ok {
		return []byte(b), nil
	}
	return nil, fs.ErrNotExist
}

func (r mapReader) Lstat(string) (os.FileInfo, error)     { return nil, fs.ErrNotExist }
func (r mapReader) ReadDir(string) ([]os.DirEntry, error) { return nil, fs.ErrNotExist }
func (r mapReader) Readlink(string) (string, error)       { return "", fs.ErrNotExist }
func (r mapReader) EvalSymlinks(string) (string, error)   { return "", fs.ErrNotExist }

func TestIsVirtualized(t *testing.T) {
	type in struct {
		Goos     string
		ExecOut  string `yaml:"execOut"`
		ExecFail bool   `yaml:"execFail"`
		Files    map[string]string
	}
	type c struct {
		Name string
		In   in
		Want bool
	}
	testyml.Run(t, td, "testdata/spec/is_virtualized.spec.yml", func(t *testing.T, c c) {
		m := &execx.Mock{Stub: func(argv []string) ([]byte, error) {
			if c.In.ExecFail {
				return nil, errors.New("stub: detect fail")
			}
			return []byte(c.In.ExecOut), nil
		}}
		if got := isVirtualized(c.In.Goos, m, mapReader{c.In.Files}); got != c.Want {
			t.Errorf("isVirtualized(%q) = %v, want %v", c.In.Goos, got, c.Want)
		}
	})
}

func TestUserHome(t *testing.T) {
	type in struct {
		User  string
		Users map[string]string
	}
	type want struct {
		Value string
		Error bool
	}
	type c struct {
		Name string
		In   in
		Want want
	}
	testyml.Run(t, td, "testdata/spec/user_home.spec.yml", func(t *testing.T, c c) {
		prev := UserLookup
		UserLookup = func(name string) (*user.User, error) {
			home, ok := c.In.Users[name]
			if !ok {
				return nil, user.UnknownUserError(name)
			}
			return &user.User{HomeDir: home}, nil
		}
		t.Cleanup(func() { UserLookup = prev })
		got, err := UserHome(c.In.User)
		if c.Want.Error {
			if err == nil {
				t.Fatalf("UserHome(%q) = %q, want error", c.In.User, got)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if got != c.Want.Value {
			t.Errorf("UserHome(%q) = %q, want %q", c.In.User, got, c.Want.Value)
		}
	})
}

// [<] 🤖🤖
