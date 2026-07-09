package cli

// [>] 🤖🤖

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/config"
	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/host"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

const cheRepoPwd = "testdata/fixture/commands/tree-che-repo"

// repoEnv materializes the pwd fixture as a committed git repo with an on-disk
// HOME, chdirs in, exports HOME + CHE_SKIP_EXEC_IF so build() resolves against
// it. Returns HOME. Skips as root (build resolves $HOME).
func repoEnv(t *testing.T, pwd string) string {
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
	dirFlag = dir
	t.Cleanup(func() {
		dirFlag = ""
		if err := os.Chdir(prev); err != nil {
			t.Fatal(err)
		}
	})
	t.Setenv("CHE_SKIP_EXEC_IF", "1")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	return home
}

// setupMock wires the mock che repo with dry-run off, injects a record-only
// MockFS into every built Host plus a launchd-modeling mock executor, and runs
// build() so each command test starts from resolved state. Returns HOME plus
// both mocks.
func setupMock(t *testing.T, pwd, profile string) (string, *testutil.MockFS, *execx.Mock) {
	t.Helper()
	home := repoEnv(t, pwd)
	t.Setenv("CHE_DRY_RUN", "")
	dryRunMode = ""
	profileForce = profile
	t.Cleanup(func() { dryRunMode, profileForce = "", "" })

	mock := &testutil.MockFS{}
	prevHost := newHost
	newHost = func(repoRoot, home, profile string, cfg config.Config) host.Host {
		return host.New(repoRoot, home, profile, cfg).WithFS(mock)
	}
	t.Cleanup(func() { newHost = prevHost })

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

	if err := build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	return home, mock, exe
}

// --profile forces one defined profile, execIf skipped, autoExec irrelevant.
func TestBuildProfileFlag(t *testing.T) {
	repoEnv(t, cheRepoPwd)
	profileForce = "ontoRepo"
	t.Cleanup(func() { profileForce = "" })
	if err := build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	if units[0].host.Profile != "ontoRepo" {
		t.Fatalf("Profile = %q, want ontoRepo (--profile forces one)", units[0].host.Profile)
	}
	profileForce = "nonexistent"
	if err := build(); err == nil {
		t.Fatal("build() with undefined --profile should error")
	}
}

// build() reads CHE_DRY_RUN from env when the flag is unset.
func TestBuildDryRunEnvFallback(t *testing.T) {
	repoEnv(t, cheRepoPwd)
	dryRunMode = ""
	t.Cleanup(func() { dryRunMode = "" })
	t.Setenv("CHE_DRY_RUN", "all")
	if err := build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	if !units[0].host.IsOptionEqualTo(config.OptionDryRun, config.DryRunAll) {
		t.Fatal("DryRunAll() = false, want true (CHE_DRY_RUN=all from env)")
	}
}

// [<] 🤖🤖
