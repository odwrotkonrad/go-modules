package cli

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// setupDryRun wires the mock che repo, flips dry-run on (reset on cleanup), and
// runs build() so each command test starts from resolved state. Returns HOME.
func setupDryRun(t *testing.T) string {
	t.Helper()
	home := testutil.MockRepoEnv(t)
	t.Setenv("CHE_DRY_RUN", "")
	dryRunMode = "delta"
	t.Cleanup(func() { dryRunMode = "" })
	if err := build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	return home
}

// --profile wins over CHE_PROFILES_FORCE_ONE and may name a mixinOnly profile.
func TestBuildProfileFlag(t *testing.T) {
	testutil.MockRepoEnv(t)
	profileForce = "base"
	t.Cleanup(func() { profileForce = "" })
	if err := build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	if theHost.Profile != "base" {
		t.Fatalf("Profile = %q, want base (--profile wins over env)", theHost.Profile)
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
	if !theHost.DryRunAll() {
		t.Fatal("DryRunAll() = false, want true (CHE_DRY_RUN=all from env)")
	}
}

// [<] 🤖🤖
