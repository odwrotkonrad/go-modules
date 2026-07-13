package cli

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/che"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// repoEnv: pwd fixture as a committed git repo plus on-disk HOME, fresh app
// pointed at it.
func repoEnv(t *testing.T, pwd string) (*app, *cobra.Command, string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("non-root path only; build resolves home from $HOME")
	}
	dir := t.TempDir()
	testyml.CopyDir(t, td, pwd, dir)
	testutil.GitRepo(t, dir)
	home := filepath.Join(dir, "home")
	require.NoError(t, os.MkdirAll(home, 0o755))
	a := New()
	root := a.Root()
	a.flags.Dir = dir
	t.Setenv("CHE_SKIP_EXEC_IF", "1")
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local/state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	return a, root, home
}

// setupMock: safe doubles in every built Host, a.init() already run.
func setupMock(t *testing.T, pwd, profile string, decl map[string]string) (*app, *cobra.Command, string) {
	t.Helper()
	a, root, home := repoEnv(t, pwd)
	t.Setenv("CHE_DRY_RUN", "")
	if profile != "" {
		a.flags.Profiles = []string{profile}
	}

	m := testutil.ApplyMocks(t, decl)
	realSeams := che.NewSeams
	testyml.Swap(t, &che.NewSeams, func(home string) che.Seams {
		s := realSeams(home)
		s.FS = m.FS
		s.Reader = &testutil.FileSystemMockReader{Roots: []string{a.flags.Dir, home}}
		s.Ledger = nil // [why] record-only tests: no ledger side effects
		return s
	})
	testyml.Swap(t, &fsutil.Sleep, testutil.SleepMock)

	require.NoError(t, a.init("all"))
	return a, root, home
}

type buildWant struct {
	Profiles  string `yaml:"profiles"`
	DryRunAll bool   `yaml:"dryRunAll"`
}

func TestInit(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/init.test.spec.yml",
		func(t *testing.T, c testyml.Case[buildWant]) {
			a, _, _ := repoEnv(t, c.Context.Pwd)
			t.Setenv("CHE_DRY_RUN", "")
			t.Setenv("CHE_VALIDATE_SPEC", "")
			for k, v := range c.Context.Env {
				t.Setenv(k, v)
			}
			if p := c.Input.Args.String(t, 0); p != "" {
				a.flags.Profiles = []string{p}
			}
			a.flags.ValidateSpec = options.ValidateSpecMode(c.Input.Args.String(t, 1))
			if extra := c.Input.Args.String(t, 2); extra != "" {
				f, err := os.OpenFile(filepath.Join(a.flags.Dir, "che.yml"), os.O_APPEND|os.O_WRONLY, 0o644)
				require.NoError(t, err)
				_, err = f.WriteString("\n" + extra)
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}
			err := a.init("all")
			if c.Expected.Check(t, err) {
				return
			}
			var names []string
			for _, p := range a.root.AllProfiles() {
				names = append(names, p.Ref())
			}
			got := buildWant{Profiles: strings.Join(names, ","), DryRunAll: a.opts.DryRun == options.DryRun.All}
			assert.Equal(t, c.Expected.Output, got)
		})
}

// [<] 🤖🤖
