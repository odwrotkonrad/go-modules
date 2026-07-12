package fsutil

// [>] 🤖🤖

import (
	"embed"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func octal(t *testing.T, s string) os.FileMode {
	t.Helper()
	n, err := strconv.ParseUint(s, 8, 32)
	require.NoErrorf(t, err, "mode %q", s)
	return os.FileMode(n)
}

func TestModeArg(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/mode_arg.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		return ModeArg(octal(t, c.Input.Args.String(t, 0))), nil
	})
}

func TestIsDir(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/is_dir.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		dir := t.TempDir()
		file := filepath.Join(dir, "f")
		require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))
		targets := map[string]string{"dir": dir, "file": file, "missing": filepath.Join(dir, "absent")}
		target, ok := targets[c.Input.Args.String(t, 0)]
		require.Truef(t, ok, "unknown target %q", c.Input.Args.String(t, 0))
		return IsDir(target), nil
	})
}

func TestUnderHome(t *testing.T) {
	f := FS{Home: "/Users/x"}
	testyml.Eq(t, td, "testdata/spec/funcs/is_under_home.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		return f.IsUnderHome(c.Input.Args.String(t, 0)), nil
	})
}

func TestExpandAll(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/expand_all.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) ([]string, error) {
		return ExpandAll(c.Input.Args.Strings(t, 0)), nil
	})
}

// TestTrackedFiles: subtree filtering, untracked exclusion, and go-git index
// parity for the mock root/ tree shapes (hidden files, .gitkeep, markers, nesting).
func TestTrackedFiles(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/tracked_files.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) ([]string, error) {
		dir := testutil.Repo(t, map[string]string{
			"che.yml":                                        "profiles:\n",
			"root/etc/zshrc":                                 "z\n",
			"root/etc/zsh/zshenv":                            "e\n",
			"root/HOME/.config/zsh/.zshrc":                   "hidden\n",
			"root/HOME/.config/zsh/.gitkeep":                 "",
			"root/HOME/.config/git/config.ontoHost.tpl":      "tpl\n",
			"root/Library/LaunchDaemons/x.plist.ontoHost.cp": "cp\n",
		})
		require.NoError(t, os.WriteFile(filepath.Join(dir, "untracked"), []byte("x"), 0o644))
		got, err := TrackedFiles(filepath.Join(dir, c.Input.Args.String(t, 0)))
		if err != nil {
			return nil, err
		}
		slices.Sort(got)
		return got, nil
	})
}

func TestMkdirArgv(t *testing.T) {
	f := FS{Home: "/Users/x"}
	testyml.Eq(t, td, "testdata/spec/funcs/mkdir_argv.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) ([]string, error) {
		a := c.Input.Args
		return f.MkdirArgv(a.String(t, 0), octal(t, a.String(t, 1)), a.Bool(t, 2)), nil
	})
}

// fsOpsWant is fs_ops' expected.output: the recorded argv (matcher) plus the
// install body passed through the temp file.
type fsOpsWant struct {
	Argv string `yaml:"argv"`
	Body string `yaml:"body"`
}

// TestFSOps runs each mutating FS op against the mock executor: recorded argv
// asserts the command shape (sudo escalation, flags), nothing touches the host.
func TestFSOps(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/fs_ops.test.spec.yml", func(t *testing.T, c testyml.Case[fsOpsWant]) {
		if strings.HasPrefix(c.Expected.Output.Argv, "sudo ") && os.Geteuid() == 0 {
			t.Skip("sudo prefix absent when running as root")
		}
		m := testutil.ApplyMocks(t, c.Context.MockedInterfaces)
		f := FS{Home: "/Users/x"}
		a := c.Input.Args
		var err error
		switch c.Context.Function {
		case "fsutil.FS.Mkdir":
			err = f.Mkdir(a.String(t, 0), octal(t, a.String(t, 1)), a.Bool(t, 2))
		case "fsutil.FS.Symlink":
			err = f.Symlink(a.String(t, 0), a.String(t, 1))
		case "fsutil.FS.Copy":
			err = f.Copy(a.String(t, 0), a.String(t, 1), octal(t, a.String(t, 2)))
		case "fsutil.FS.Remove":
			err = f.Remove(a.String(t, 0))
		case "fsutil.FS.Chown":
			err = f.Chown(a.String(t, 0), a.String(t, 1))
		case "fsutil.FS.Chmod":
			err = f.Chmod(a.String(t, 0), a.String(t, 1))
		case "fsutil.FS.Install":
			err = f.Install(a.String(t, 0), []byte(a.String(t, 3)), octal(t, a.String(t, 1)), a.String(t, 2))
		default:
			t.Fatalf("unknown function %q", c.Context.Function)
		}
		require.NoError(t, err)
		calls := m.Exec.Calls()
		require.Len(t, calls, 1)
		noDeref := "-n"
		if runtime.GOOS == "darwin" {
			noDeref = "-h"
		}
		testyml.MustMatch(t, calls[0], testyml.Expand(c.Expected.Output.Argv, map[string]string{"NODEREF": noDeref}))
		if c.Expected.Output.Body != "" {
			require.Len(t, m.Exec.Bodies, 1)
			assert.Equal(t, c.Expected.Output.Body, m.Exec.Bodies[0])
		}
	})
}

// [<] 🤖🤖
