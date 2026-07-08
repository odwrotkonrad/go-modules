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
	if len(units) != 1 || strings.Contains(out, "plugin") {
		t.Fatalf("build() must defer plugins: units = %d\n%s", len(units), out)
	}
	var ran []unit
	out, err = testutil.CaptureStdout(t, func() error {
		return forEachUnit(func(u unit) error { ran = append(ran, u); return nil })
	})
	if err != nil {
		t.Fatalf("forEachUnit errored: %v\n%s", err, out)
	}
	if len(ran) != 2 {
		t.Fatalf("ran units = %d, want 2\n%s", len(ran), out)
	}
	cache := filepath.Join(home, ".local/share/che/plugins")
	if !strings.HasPrefix(ran[1].host.RepoRoot, cache+"/") {
		t.Errorf("plugin RepoRoot = %q, want under %q", ran[1].host.RepoRoot, cache)
	}
	if ran[1].ref != ref {
		t.Errorf("plugin ref = %q, want %q", ran[1].ref, ref)
	}
	if !slices.Contains(ran[1].res.Scripts, "scripts/bootstrap.zsh") {
		t.Errorf("plugin Scripts = %v, want scripts/bootstrap.zsh", ran[1].res.Scripts)
	}
	testutil.WantLines(t, out,
		"run "+ref,
		"execIf(pass): profile gitlabGroup: env:PLUGIN_GATE")

	t.Setenv("PLUGIN_GATE", "")
	if err := build(); err != nil {
		t.Fatalf("build() (gate unset) errored: %v", err)
	}
	ran = nil
	out, err = testutil.CaptureStdout(t, func() error {
		return forEachUnit(func(u unit) error { ran = append(ran, u); return nil })
	})
	if err != nil {
		t.Fatalf("forEachUnit (gate unset) errored: %v\n%s", err, out)
	}
	if len(ran) != 1 {
		t.Fatalf("ran units = %d, want 1 (plugin skipped)\n%s", len(ran), out)
	}
	testutil.WantLines(t, out,
		"execIf(reject): profile gitlabGroup: env:PLUGIN_GATE",
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

	if err := build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	var ran []unit
	var gateInOp string
	out, err := testutil.CaptureStdout(t, func() error {
		return forEachUnit(func(u unit) error {
			ran = append(ran, u)
			gateInOp = os.Getenv("PLUGIN_GATE")
			return nil
		})
	})
	if err != nil {
		t.Fatalf("forEachUnit errored: %v\n%s", err, out)
	}
	if len(ran) != 2 {
		t.Fatalf("ran units = %d, want 2\n%s", len(ran), out)
	}
	if got := ran[1].env["PLUGIN_GATE"]; got != "1" {
		t.Errorf("unit env PLUGIN_GATE = %q, want %q", got, "1")
	}
	if gateInOp != "1" {
		t.Errorf("PLUGIN_GATE in plugin op = %q, want %q", gateInOp, "1")
	}
	if !slices.Contains(ran[1].res.Scripts, "scripts/bootstrap.zsh") {
		t.Errorf("plugin Scripts = %v, want scripts/bootstrap.zsh", ran[1].res.Scripts)
	}
	if _, set := os.LookupEnv("PLUGIN_GATE"); set {
		t.Error("PLUGIN_GATE leaked into the process env after forEachUnit")
	}
	testutil.WantLines(t, out, "execIf(pass): profile gitlabGroup: env:PLUGIN_GATE")
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
