package cli

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// run-scripts: dry-run logs each script's abs path, no exec.
func TestRunScriptsCmd(t *testing.T) {
	setupDryRun(t)
	out := testutil.RunDry(t, RunScriptsCmd, true)
	testutil.WantLines(t, out, "run-scripts(dry-run): "+theHost.RepoRoot+"/install/unit")
}

// a NAME arg narrows the script list; an unmatched NAME errors.
func TestRunScriptsCmdFilter(t *testing.T) {
	setupDryRun(t)
	out, err := testutil.CaptureStdout(t, func() error {
		return RunScriptsCmd.RunE(RunScriptsCmd, []string{"unit"})
	})
	if err != nil {
		t.Fatalf("RunScriptsCmd errored: %v", err)
	}
	testutil.WantLines(t, out, "run-scripts(dry-run): "+theHost.RepoRoot+"/install/unit")

	if _, err := testutil.CaptureStdout(t, func() error {
		return RunScriptsCmd.RunE(RunScriptsCmd, []string{"__nope__"})
	}); err == nil {
		t.Error("RunScriptsCmd must error when NAME matches no script")
	}
}

// [<] 🤖🤖
