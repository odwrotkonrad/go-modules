package cli

// [>] 🤖🤖

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
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
	t.Setenv("CHE_SKIP_EXEC_IF", "")
	t.Setenv("CHE_PROFILE", "")
	t.Setenv("CHE_DRY_RUN", "")
	t.Setenv("CHE_SKIP_PLUGINS", "")
	dryRunMode, profileForce, skipPlugins = "", "", false

	t.Setenv("PLUGIN_GATE", "1")
	t.Setenv("CHE_DEBUG", "1")
	out, err := testutil.CaptureStdout(t, build)
	if err != nil {
		t.Fatalf("build() errored: %v\n%s", err, out)
	}
	if len(units) != 1 || strings.Contains(out, "plugin") {
		t.Fatalf("build() must defer plugins: units = %d\n%s", len(units), out)
	}
	var ran []unit
	out, err = testutil.CaptureStdout(t, func() error {
		return forEachUnit("test", func(u unit) error { ran = append(ran, u); return nil })
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
		"cloned file://"+pluginRepo,
		"execIf(pass): profile gitlabGroup: env:PLUGIN_GATE")

	t.Setenv("PLUGIN_GATE", "")
	t.Setenv("CHE_DEBUG", "")
	if err := build(); err != nil {
		t.Fatalf("build() (gate unset) errored: %v", err)
	}
	ran = nil
	out, err = testutil.CaptureStdout(t, func() error {
		return forEachUnit("test", func(u unit) error { ran = append(ran, u); return nil })
	})
	if err != nil {
		t.Fatalf("forEachUnit (gate unset) errored: %v\n%s", err, out)
	}
	if len(ran) != 1 {
		t.Fatalf("ran units = %d, want 1 (plugin skipped)\n%s", len(ran), out)
	}
	testutil.WantLines(t, out, "skip "+ref+" (execIf failed)")
	testutil.NotLine(t, out, "execIf(reject)")
	testutil.NotLine(t, out, "run "+ref)
	testutil.NotLine(t, out, "pull ")
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
	t.Setenv("CHE_SKIP_EXEC_IF", "")
	t.Setenv("CHE_PROFILE", "")
	t.Setenv("CHE_DRY_RUN", "")
	t.Setenv("PLUGIN_GATE", "")
	os.Unsetenv("PLUGIN_GATE")
	t.Setenv("CHE_SKIP_PLUGINS", "")
	dryRunMode, profileForce, skipPlugins = "", "", false

	if err := build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	var ran []unit
	var gateInOp string
	out, err := testutil.CaptureStdout(t, func() error {
		return forEachUnit("test", func(u unit) error {
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

// --skip-plugins (env CHE_SKIP_PLUGINS) drops plugins entries: only the local
// unit runs, no plugin pull or log line.
func TestBuildSkipPlugins(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("non-root path only; build resolves home from $HOME")
	}
	hostRepo := testutil.Repo(t, map[string]string{
		"che.yml":           "main:\n  options: {autoExec: true}\n  plugins: [\"@file:///nonexistent::main\"]\n  include:\n    runScripts: [scripts/local.zsh]\n",
		"scripts/local.zsh": "#!/bin/zsh\n",
	})
	t.Chdir(hostRepo)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CHE_SKIP_EXEC_IF", "")
	t.Setenv("CHE_PROFILE", "")
	t.Setenv("CHE_DRY_RUN", "")
	dryRunMode, profileForce = "", ""

	for name, set := range map[string]func(){
		"flag": func() { t.Setenv("CHE_SKIP_PLUGINS", ""); skipPlugins = true },
		"env":  func() { t.Setenv("CHE_SKIP_PLUGINS", "1"); skipPlugins = false },
	} {
		set()
		if err := build(); err != nil {
			t.Fatalf("[%s] build() errored: %v", name, err)
		}
		if len(pluginRefs) != 0 {
			t.Fatalf("[%s] pluginRefs = %v, want none", name, pluginRefs)
		}
		var ran []unit
		out, err := testutil.CaptureStdout(t, func() error {
			return forEachUnit("test", func(u unit) error { ran = append(ran, u); return nil })
		})
		if err != nil {
			t.Fatalf("[%s] forEachUnit errored: %v\n%s", name, err, out)
		}
		if len(ran) != 1 || strings.Contains(out, "plugin") {
			t.Fatalf("[%s] ran units = %d, want 1 (plugins skipped)\n%s", name, len(ran), out)
		}
	}
	skipPlugins = false
}

// a dir-path plugins ref (absolute and relative) anchors the unit at the dir
// itself: no git clone/pull, no cache checkout; its op lines carry the
// profile= subtype.
func TestBuildPluginUnitsDirRef(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("non-root path only; build resolves home from $HOME")
	}
	pluginDir := t.TempDir()
	testutil.WriteTree(t, pluginDir, map[string]string{
		"che.yml":               "gitlabGroup:\n  include:\n    runScripts: [scripts/bootstrap.zsh]\n",
		"scripts/bootstrap.zsh": "#!/bin/zsh\n",
	})
	for name, refDir := range map[string]string{
		"absolute": pluginDir,
		"relative": "./plugin",
		"envvar":   "$PLUGIN_DIR_REF",
	} {
		t.Run(name, func(t *testing.T) {
			ref := refDir + "::gitlabGroup"
			hostRepo := testutil.Repo(t, map[string]string{
				"che.yml":           "main:\n  options: {autoExec: true}\n  plugins: [\"" + ref + "\"]\n  include:\n    runScripts: [scripts/local.zsh]\n",
				"scripts/local.zsh": "#!/bin/zsh\n",
			})
			if name == "relative" {
				testutil.WriteTree(t, filepath.Join(hostRepo, "plugin"), map[string]string{
					"che.yml":               "gitlabGroup:\n  include:\n    runScripts: [scripts/bootstrap.zsh]\n",
					"scripts/bootstrap.zsh": "#!/bin/zsh\n",
				})
			}
			home := t.TempDir()
			t.Chdir(hostRepo)
			t.Setenv("HOME", home)
			t.Setenv("PLUGIN_DIR_REF", pluginDir)
			t.Setenv("CHE_SKIP_EXEC_IF", "")
			t.Setenv("CHE_PROFILE", "")
			t.Setenv("CHE_DRY_RUN", "")
			t.Setenv("CHE_SKIP_PLUGINS", "")
			t.Setenv("CHE_DEBUG", "")
			dryRunMode, profileForce, skipPlugins = "all", "", false
			t.Cleanup(func() { dryRunMode = "" })

			if err := build(); err != nil {
				t.Fatalf("build() errored: %v", err)
			}
			var ran []unit
			out, err := testutil.CaptureStdout(t, func() error {
				return forEachUnit("run-scripts", func(u unit) error {
					ran = append(ran, u)
					scripts, err := u.host.ResolveScripts(u.res.Scripts)
					if err != nil {
						return err
					}
					return u.host.RunScripts(scripts)
				})
			})
			if err != nil {
				t.Fatalf("forEachUnit errored: %v\n%s", err, out)
			}
			if len(ran) != 2 {
				t.Fatalf("ran units = %d, want 2\n%s", len(ran), out)
			}
			want := pluginDir
			if name == "relative" {
				want = filepath.Join(hostRepo, "plugin")
			}
			if got, _ := filepath.EvalSymlinks(ran[1].host.RepoRoot); got != mustEval(t, want) {
				t.Errorf("plugin RepoRoot = %q, want %q", ran[1].host.RepoRoot, want)
			}
			testutil.WantLines(t, out,
				"run-scripts(dry-run=all,profile=gitlabGroup): ",
				"scripts/bootstrap.zsh")
			for _, frag := range []string{"clone", "pull"} {
				testutil.NotLine(t, out, frag)
			}
		})
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	out, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// a failing unit does not stop the rest: both units run, each failure prints a
// "<name>(report): fail <ref>" line, and forEachUnit returns joined errors.
func TestForEachUnitContinuesOnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("non-root path only; build resolves home from $HOME")
	}
	pluginDir := t.TempDir()
	testutil.WriteTree(t, pluginDir, map[string]string{
		"che.yml":               "gitlabGroup:\n  include:\n    runScripts: [scripts/bootstrap.zsh]\n",
		"scripts/bootstrap.zsh": "#!/bin/zsh\n",
	})
	ref := pluginDir + "::gitlabGroup"
	hostRepo := testutil.Repo(t, map[string]string{
		"che.yml":           "main:\n  options: {autoExec: true}\n  plugins: [\"" + ref + "\"]\n  include:\n    runScripts: [scripts/local.zsh]\n",
		"scripts/local.zsh": "#!/bin/zsh\n",
	})
	t.Chdir(hostRepo)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CHE_SKIP_EXEC_IF", "")
	t.Setenv("CHE_PROFILE", "")
	t.Setenv("CHE_DRY_RUN", "")
	t.Setenv("CHE_SKIP_PLUGINS", "")
	t.Setenv("CHE_DEBUG", "")
	dryRunMode, profileForce, skipPlugins = "", "", false

	if err := build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	var ran []unit
	out, err := testutil.CaptureStdout(t, func() error {
		return forEachUnit("test", func(u unit) error {
			ran = append(ran, u)
			if u.ref == "" {
				return errors.New("local boom")
			}
			return nil
		})
	})
	if err == nil || !strings.Contains(err.Error(), "local boom") {
		t.Fatalf("err = %v, want joined local boom", err)
	}
	if len(ran) != 2 {
		t.Fatalf("ran units = %d, want 2 (continue past local failure)\n%s", len(ran), out)
	}
	testutil.WantLines(t, out, "test(report): fail local: local boom")
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
