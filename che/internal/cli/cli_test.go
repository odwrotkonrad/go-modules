package cli

// [>] 🤖🤖

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go-modules/che/internal/config"
	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/host"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

const cheRepoPwd = "testdata/fixture/commands/common/sample-tree"

// repoEnv materializes the pwd fixture as a committed git repo with an on-disk
// HOME, chdirs in, exports HOME + CHE_SKIP_EXEC_IF, and returns a fresh app
// (root built, dirFlag pointed at the repo) so a.build() resolves against it.
// Skips as root (build resolves $HOME).
func repoEnv(t *testing.T, pwd string) (*CheApp, *cobra.Command, string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("non-root path only; build resolves home from $HOME")
	}
	dir := t.TempDir()
	testyml.CopyDir(t, td, pwd, dir)
	testutil.GitRepo(t, dir)
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	a := New()
	root := a.Root()
	a.dirFlag = dir
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Fatal(err)
		}
	})
	t.Setenv("CHE_SKIP_EXEC_IF", "1")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	return a, root, home
}

// setupMock wires the mock che repo with dry-run off, injects a record-only
// MockFS into every built Host plus a launchd-modeling mock executor, and runs
// a.build() so each command test starts from resolved state. Returns the app,
// its root, and HOME.
func setupMock(t *testing.T, pwd, profile string) (*CheApp, *cobra.Command, string) {
	t.Helper()
	a, root, home := repoEnv(t, pwd)
	t.Setenv("CHE_DRY_RUN", "")
	a.profileForce = profile

	mock := &testutil.MockFS{}
	a.newHost = func(repoRoot, home, profile string, cfg config.Config) host.Host {
		reader := testutil.ScopedReader{Roots: []string{repoRoot, home}}
		return host.New(repoRoot, home, profile, cfg).WithFS(mock).WithFSReader(reader)
	}

	exe := &execx.Mock{}
	loaded := true
	exe.Stub = func(argv []string) ([]byte, error) {
		cmd := strings.Join(argv, " ")
		switch {
		case strings.Contains(cmd, "launchctl bootout"):
			loaded = false
		case strings.Contains(cmd, "launchctl bootstrap"):
			loaded = true
		case strings.Contains(cmd, "launchctl print"):
			if !loaded {
				return nil, errors.New("stub: not loaded")
			}
			return []byte("\tpid = 4242\n"), nil
		}
		return nil, nil
	}
	execx.Swap(t, exe)

	prevSleep := host.Sleep
	host.Sleep = func(time.Duration) {}
	t.Cleanup(func() { host.Sleep = prevSleep })

	if err := a.build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	return a, root, home
}

// --profile forces one defined profile, execIf skipped, autoExec irrelevant.
func TestBuildProfileFlag(t *testing.T) {
	a, _, _ := repoEnv(t, cheRepoPwd)
	a.profileForce = "ontoRepo"
	if err := a.build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	if a.units[0].host.Profile != "ontoRepo" {
		t.Fatalf("Profile = %q, want ontoRepo (--profile forces one)", a.units[0].host.Profile)
	}
	a.profileForce = "nonexistent"
	if err := a.build(); err == nil {
		t.Fatal("build() with undefined --profile should error")
	}
}

func TestBuildValidateSchema(t *testing.T) {
	a, _, _ := repoEnv(t, cheRepoPwd)
	specPath := filepath.Join(a.dirFlag, "che.yml")
	f, err := os.OpenFile(specPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\nbogusProfile:\n  includes:\n    link: [HOME/**]\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CHE_VALIDATE_SCHEMA", "")
	if err := a.build(); err != nil {
		t.Fatalf("build() in warn mode errored: %v", err)
	}

	a.validateSchemaMode = "error"
	err = a.build()
	if err == nil {
		t.Fatal("build() with --validate-schema error should fail on the violation")
	}
	if !strings.Contains(err.Error(), "includes") {
		t.Errorf("error does not name the violating key: %v", err)
	}

	a.validateSchemaMode = ""
	t.Setenv("CHE_VALIDATE_SCHEMA", "error")
	if err := a.build(); err == nil {
		t.Fatal("build() with CHE_VALIDATE_SCHEMA=error should fail on the violation")
	}

	a.validateSchemaMode = "bogus"
	if err := a.build(); err == nil {
		t.Fatal("build() with an unknown --validate-schema mode should error")
	}
}

// build() reads CHE_DRY_RUN from env when the flag is unset.
func TestBuildDryRunEnvFallback(t *testing.T) {
	a, _, _ := repoEnv(t, cheRepoPwd)
	t.Setenv("CHE_DRY_RUN", "all")
	if err := a.build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	if !a.units[0].host.IsOptionEqualTo(config.OptionDryRun, config.DryRunAll) {
		t.Fatal("DryRunAll() = false, want true (CHE_DRY_RUN=all from env)")
	}
}

// [<] 🤖🤖
