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

const cheRepoPwd = "testdata/fixture/commands/common/sample-tree"

// repoEnv materializes the pwd fixture as a committed git repo with an on-disk
// HOME, exports HOME + CHE_SKIP_EXEC_IF, and returns a fresh app (root built,
// dirFlag pointed at the repo) so a.build() resolves against it. Skips as root
// (build resolves $HOME).
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

// setupMock wires the mock che repo with dry-run off, injects the safe-double
// set (record-only writer, scoped reader, launchd-modeling executor, no-op
// sleep) into every built Host, and runs a.build() so each command test starts
// from resolved state. Returns the app, its root, and HOME.
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

// --profile forces one defined profile, execIf skipped, autoExec irrelevant.
func TestBuildProfileFlag(t *testing.T) {
	a, _, _ := repoEnv(t, cheRepoPwd)
	a.profileForce = "ontoRepo"
	require.NoError(t, a.build())
	assert.Equal(t, "ontoRepo", a.units[0].host.Profile, "--profile forces one")
	a.profileForce = "nonexistent"
	assert.Error(t, a.build(), "undefined --profile must error")
}

func TestBuildValidateSpec(t *testing.T) {
	a, _, _ := repoEnv(t, cheRepoPwd)
	specPath := filepath.Join(a.dirFlag, "che.yml")
	f, err := os.OpenFile(specPath, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = f.WriteString("\nbogusProfile:\n  includes:\n    link: [HOME/**]\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	t.Setenv("CHE_VALIDATE_SPEC", "")
	require.NoError(t, a.build(), "warn mode must not fail")

	a.validateSpecMode = "error"
	err = a.build()
	require.Error(t, err, "--validate-spec error must fail on the violation")
	assert.Contains(t, err.Error(), "includes", "error must name the violating key")

	a.validateSpecMode = ""
	t.Setenv("CHE_VALIDATE_SPEC", "error")
	assert.Error(t, a.build(), "CHE_VALIDATE_SPEC=error must fail on the violation")

	a.validateSpecMode = "bogus"
	assert.Error(t, a.build(), "unknown --validate-spec mode must error")
}

// build() reads CHE_DRY_RUN from env when the flag is unset.
func TestBuildDryRunEnvFallback(t *testing.T) {
	a, _, _ := repoEnv(t, cheRepoPwd)
	t.Setenv("CHE_DRY_RUN", "all")
	require.NoError(t, a.build())
	assert.True(t, a.units[0].host.IsDryRun(), "CHE_DRY_RUN=all from env")
}

// [<] 🤖🤖
