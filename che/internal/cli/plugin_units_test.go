package cli

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// a plugins `@url::profile` entry resolves into an extra unit anchored at
// the cache checkout; a failing remote execIf skips it (after the pull).
func TestBuildPluginUnits(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("non-root path only; build resolves home from $HOME")
	}
	pluginRepo := testutil.Repo(t, map[string]string{
		"che.yml":               "gitlabGroup:\n  options:\n    execIf: ['env:PLUGIN_GATE']\n  include:\n    runScripts: [scripts/bootstrap.zsh]\n",
		"scripts/bootstrap.zsh": "#!/bin/zsh\n",
	})
	ref := "@file://" + pluginRepo + "::gitlabGroup"
	hostRepo := testutil.Repo(t, map[string]string{
		"che.yml":           "main:\n  options: {autoExec: true}\n  plugins: [\"" + ref + "\"]\n  include:\n    runScripts: [scripts/local.zsh]\n",
		"scripts/local.zsh": "#!/bin/zsh\n",
	})
	home := t.TempDir()
	t.Chdir(hostRepo)
	t.Setenv("HOME", home)
	t.Setenv("CHE_OMIT_EXEC_IF", "")
	t.Setenv("CHE_PROFILE", "")
	t.Setenv("CHE_DRY_RUN", "")
	dryRunMode, profileForce = "", ""

	t.Setenv("PLUGIN_GATE", "1")
	out, err := testutil.CaptureStdout(t, build)
	if err != nil {
		t.Fatalf("build() errored: %v\n%s", err, out)
	}
	if len(units) != 2 {
		t.Fatalf("units = %d, want 2\n%s", len(units), out)
	}
	cache := filepath.Join(home, ".local/share/che/plugins")
	if !strings.HasPrefix(units[1].host.RepoRoot, cache+"/") {
		t.Errorf("plugin RepoRoot = %q, want under %q", units[1].host.RepoRoot, cache)
	}
	if units[1].ref != ref {
		t.Errorf("plugin ref = %q, want %q", units[1].ref, ref)
	}
	if !slices.Contains(units[1].res.Scripts, "scripts/bootstrap.zsh") {
		t.Errorf("plugin Scripts = %v, want scripts/bootstrap.zsh", units[1].res.Scripts)
	}
	testutil.WantLines(t, out, "execIf: profile gitlabGroup: env:PLUGIN_GATE -> true")

	t.Setenv("PLUGIN_GATE", "")
	out, err = testutil.CaptureStdout(t, build)
	if err != nil {
		t.Fatalf("build() (gate unset) errored: %v\n%s", err, out)
	}
	if len(units) != 1 {
		t.Fatalf("units = %d, want 1 (plugin skipped)\n%s", len(units), out)
	}
	testutil.WantLines(t, out,
		"execIf: profile gitlabGroup: env:PLUGIN_GATE -> false",
		"skip "+ref+" (execIf failed)")
}

// an object-form plugins entry's env gates the remote execIf while the host
// env is unset, lands on the unit, and does not leak past build().
func TestBuildPluginUnitEnv(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("non-root path only; build resolves home from $HOME")
	}
	pluginRepo := testutil.Repo(t, map[string]string{
		"che.yml":               "gitlabGroup:\n  options:\n    execIf: ['env:PLUGIN_GATE']\n  include:\n    runScripts: [scripts/bootstrap.zsh]\n",
		"scripts/bootstrap.zsh": "#!/bin/zsh\n",
	})
	ref := "@file://" + pluginRepo + "::gitlabGroup"
	hostRepo := testutil.Repo(t, map[string]string{
		"che.yml":           "main:\n  options: {autoExec: true}\n  plugins:\n    - ref: \"" + ref + "\"\n      env:\n        PLUGIN_GATE: \"1\"\n  include:\n    runScripts: [scripts/local.zsh]\n",
		"scripts/local.zsh": "#!/bin/zsh\n",
	})
	home := t.TempDir()
	t.Chdir(hostRepo)
	t.Setenv("HOME", home)
	t.Setenv("CHE_OMIT_EXEC_IF", "")
	t.Setenv("CHE_PROFILE", "")
	t.Setenv("CHE_DRY_RUN", "")
	t.Setenv("PLUGIN_GATE", "")
	os.Unsetenv("PLUGIN_GATE")
	dryRunMode, profileForce = "", ""

	out, err := testutil.CaptureStdout(t, build)
	if err != nil {
		t.Fatalf("build() errored: %v\n%s", err, out)
	}
	if len(units) != 2 {
		t.Fatalf("units = %d, want 2\n%s", len(units), out)
	}
	if got := units[1].env["PLUGIN_GATE"]; got != "1" {
		t.Errorf("unit env PLUGIN_GATE = %q, want %q", got, "1")
	}
	if !slices.Contains(units[1].res.Scripts, "scripts/bootstrap.zsh") {
		t.Errorf("plugin Scripts = %v, want scripts/bootstrap.zsh", units[1].res.Scripts)
	}
	if _, set := os.LookupEnv("PLUGIN_GATE"); set {
		t.Error("PLUGIN_GATE leaked into the process env after build()")
	}
	testutil.WantLines(t, out, "execIf: profile gitlabGroup: env:PLUGIN_GATE -> true")
}

// withEnv shadows an existing var, sets an absent one, restores both after.
func TestUnitWithEnv(t *testing.T) {
	t.Setenv("WITHENV_SHADOWED", "host")
	t.Setenv("WITHENV_ABSENT", "")
	os.Unsetenv("WITHENV_ABSENT")
	u := unit{env: map[string]string{"WITHENV_SHADOWED": "plugin", "WITHENV_ABSENT": "x"}}
	err := u.withEnv(func() error {
		if got := os.Getenv("WITHENV_SHADOWED"); got != "plugin" {
			t.Errorf("shadowed = %q, want plugin", got)
		}
		if got := os.Getenv("WITHENV_ABSENT"); got != "x" {
			t.Errorf("absent = %q, want x", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("WITHENV_SHADOWED"); got != "host" {
		t.Errorf("shadowed after = %q, want host", got)
	}
	if _, set := os.LookupEnv("WITHENV_ABSENT"); set {
		t.Error("absent var not unset after withEnv")
	}
}

// [<] 🤖🤖
