package cli

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/config"
	"gitlab.com/konradodwrot/go-modules/che/internal/host"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// repoEnv: pwd fixture as a committed git repo plus on-disk HOME, fresh app
// pointed at it.
func repoEnv(t *testing.T, pwd string) (*CheApp, *cobra.Command, string) {
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
	a.dirFlag = dir
	t.Setenv("CHE_SKIP_EXEC_IF", "1")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	return a, root, home
}

// setupMock: safe doubles in every built Host, a.build() already run.
func setupMock(t *testing.T, pwd, profile string, decl map[string]string) (*CheApp, *cobra.Command, string) {
	t.Helper()
	a, root, home := repoEnv(t, pwd)
	t.Setenv("CHE_DRY_RUN", "")
	a.profileForce = profile

	m := testutil.ApplyMocks(t, decl)
	a.newHost = func(repoRoot, home, profile string, cfg config.Config) host.Host {
		reader := &testutil.FileSystemMockReader{Roots: []string{repoRoot, home}}
		return host.New(repoRoot, home, profile, cfg).WithFS(m.FS).WithFSReader(reader)
	}
	testyml.Swap(t, &host.Sleep, testutil.SleepMock)

	require.NoError(t, a.build())
	return a, root, home
}

type buildWant struct {
	Profile   string `yaml:"profile"`
	DryRunAll bool   `yaml:"dryRunAll"`
}

func TestBuild(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/build.test.spec.yml",
		func(t *testing.T, c testyml.Case[buildWant]) {
			a, _, _ := repoEnv(t, c.Context.Pwd)
			t.Setenv("CHE_DRY_RUN", "")
			t.Setenv("CHE_VALIDATE_SPEC", "")
			for k, v := range c.Context.Env {
				t.Setenv(k, v)
			}
			a.profileForce = c.Input.Args.String(t, 0)
			a.validateSpecMode = c.Input.Args.String(t, 1)
			if extra := c.Input.Args.String(t, 2); extra != "" {
				f, err := os.OpenFile(filepath.Join(a.dirFlag, "che.yml"), os.O_APPEND|os.O_WRONLY, 0o644)
				require.NoError(t, err)
				_, err = f.WriteString("\n" + extra)
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}
			err := a.build()
			if c.Expected.Check(t, err) {
				return
			}
			got := buildWant{Profile: a.units[0].host.Profile, DryRunAll: a.units[0].host.IsDryRun()}
			assert.Equal(t, c.Expected.Output, got)
		})
}

// [<] 🤖🤖
