package cli

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

// setupDryRun wires the mock che repo, flips dry-run on (reset on cleanup), and
// runs build() so each command test starts from resolved state. Returns HOME.
func setupDryRun(t *testing.T, spec, tree, profile string) string {
	t.Helper()
	home := testutil.RepoEnv(t, spec, tree)
	t.Setenv("CHE_DRY_RUN", "")
	dryRunMode = "delta"
	profileForce = profile
	t.Cleanup(func() { dryRunMode, profileForce = "", "" })
	if err := build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	return home
}

// --profile forces one defined profile, execIf skipped, autoExec irrelevant.
func TestBuildProfileFlag(t *testing.T) {
	testutil.MockRepoEnv(t)
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
	testutil.MockRepoEnv(t)
	dryRunMode = ""
	t.Cleanup(func() { dryRunMode = "" })
	t.Setenv("CHE_DRY_RUN", "all")
	if err := build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	if !units[0].host.DryRunAll() {
		t.Fatal("DryRunAll() = false, want true (CHE_DRY_RUN=all from env)")
	}
}

// [<] 🤖🤖
