package cli

// [>] 🤖🤖

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

// pluginEnv anchors a plugin test in hostRepo: skip-if-root, chdir in, temp
// HOME, every CHE_* knob cleared, executor mocked (plugin git runs through the
// go-git model, nothing spawns). Returns HOME.
func pluginEnv(t *testing.T, hostRepo string) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("non-root path only; build resolves home from $HOME")
	}
	home := t.TempDir()
	t.Chdir(hostRepo)
	t.Setenv("HOME", home)
	for _, k := range []string{
		"CHE_SKIP_EXEC_IF", "CHE_PROFILE", "CHE_DRY_RUN",
		"CHE_SKIP_PLUGINS", "CHE_DEBUG", "CHE_VALIDATE_SPEC",
	} {
		t.Setenv(k, "")
	}
	execx.Swap(t, testutil.NewCmdMockExecutor())
	return home
}

// collectUnits runs forEachUnit("test"), collecting the units it visits.
func collectUnits(t *testing.T, a *CheApp) ([]unit, string, error) {
	t.Helper()
	var ran []unit
	out, err := testutil.CaptureStdout(t, func() error {
		return a.forEachUnit("test", func(u unit) error { ran = append(ran, u); return nil })
	})
	return ran, out, err
}

func TestBuildPluginValidateSpec(t *testing.T) {
	pluginRepo := testutil.Repo(t, map[string]string{
		"che.yml": "p:\n  includes:\n    link: [HOME/**]\n",
	})
	hostRepo := testutil.Repo(t, map[string]string{
		"che.yml": "main:\n  options: {autoExec: true}\n  plugins: [\"" + pluginRepo + "::p\"]\n",
	})
	pluginEnv(t, hostRepo)

	a := New()
	require.NoError(t, a.build())
	_, out, err := collectUnits(t, a)
	require.NoErrorf(t, err, "forEachUnit in warn mode\n%s", out)
	testutil.WantLines(t, out, "validate(che.yml)", "includes")

	t.Setenv("CHE_VALIDATE_SPEC", "error")
	a = New()
	require.NoError(t, a.build(), "local che.yml is valid")
	_, out, err = collectUnits(t, a)
	require.Errorf(t, err, "forEachUnit in error mode must fail on the plugin violation\n%s", out)
	assert.Contains(t, err.Error(), "includes", "error must name the violating key")
}

// a plugins `@url::profile` entry resolves into an extra unit anchored at
// the cache checkout; a failing remote execIf skips it (after the pull).
func TestBuildPluginUnits(t *testing.T) {
	pluginRepo := testutil.Repo(t, map[string]string{
		"che.yml":               "gitlabGroup:\n  options:\n    execIf: ['env:PLUGIN_GATE']\n  include:\n    runScripts: [scripts/bootstrap.zsh]\n",
		"scripts/bootstrap.zsh": "#!/bin/zsh\n",
	})
	ref := "@file://" + pluginRepo + "::gitlabGroup"
	hostRepo := testutil.Repo(t, map[string]string{
		"che.yml":           "main:\n  options: {autoExec: true}\n  plugins: [\"" + ref + "\"]\n  include:\n    runScripts: [scripts/local.zsh]\n",
		"scripts/local.zsh": "#!/bin/zsh\n",
	})
	home := pluginEnv(t, hostRepo)
	a := New()

	t.Setenv("PLUGIN_GATE", "1")
	t.Setenv("CHE_DEBUG", "1")
	out, err := testutil.CaptureStdout(t, a.build)
	require.NoErrorf(t, err, "build()\n%s", out)
	require.Len(t, a.units, 1, "build() must defer plugins")
	require.NotContains(t, out, "plugin", "build() must defer plugins")
	ran, out, err := collectUnits(t, a)
	require.NoErrorf(t, err, "forEachUnit\n%s", out)
	require.Lenf(t, ran, 2, "ran units\n%s", out)
	cache := filepath.Join(home, ".local/share/che/plugins")
	assert.Truef(t, strings.HasPrefix(ran[1].host.RepoRoot, cache+"/"), "plugin RepoRoot = %q, want under %q", ran[1].host.RepoRoot, cache)
	assert.Equal(t, ref, ran[1].ref)
	assert.Contains(t, ran[1].res.Scripts, "scripts/bootstrap.zsh")
	testutil.WantLines(t, out,
		"run "+ref,
		"cloned file://"+pluginRepo,
		"execIf(pass): profile gitlabGroup: env:PLUGIN_GATE")

	t.Setenv("PLUGIN_GATE", "")
	t.Setenv("CHE_DEBUG", "")
	require.NoError(t, a.build(), "build() (gate unset)")
	ran, out, err = collectUnits(t, a)
	require.NoErrorf(t, err, "forEachUnit (gate unset)\n%s", out)
	require.Lenf(t, ran, 1, "plugin must be skipped\n%s", out)
	testutil.WantLines(t, out, "skip "+ref+" (execIf failed)")
	testutil.NotLine(t, out, "execIf(reject)")
	testutil.NotLine(t, out, "run "+ref)
	testutil.NotLine(t, out, "pull ")
}

// an object-form plugins entry's env gates the remote execIf while the host
// env is unset, lands on the unit, and does not leak past build().
func TestBuildPluginUnitEnv(t *testing.T) {
	pluginRepo := testutil.Repo(t, map[string]string{
		"che.yml":               "gitlabGroup:\n  options:\n    execIf: ['env:PLUGIN_GATE']\n  include:\n    runScripts: [scripts/bootstrap.zsh]\n",
		"scripts/bootstrap.zsh": "#!/bin/zsh\n",
	})
	ref := "@file://" + pluginRepo + "::gitlabGroup"
	hostRepo := testutil.Repo(t, map[string]string{
		"che.yml":           "main:\n  options: {autoExec: true}\n  plugins:\n    - ref: \"" + ref + "\"\n      env:\n        PLUGIN_GATE: \"1\"\n  include:\n    runScripts: [scripts/local.zsh]\n",
		"scripts/local.zsh": "#!/bin/zsh\n",
	})
	pluginEnv(t, hostRepo)
	t.Setenv("PLUGIN_GATE", "")
	os.Unsetenv("PLUGIN_GATE")
	a := New()

	require.NoError(t, a.build())
	var ran []unit
	var gateInOp string
	out, err := testutil.CaptureStdout(t, func() error {
		return a.forEachUnit("test", func(u unit) error {
			ran = append(ran, u)
			gateInOp = os.Getenv("PLUGIN_GATE")
			return nil
		})
	})
	require.NoErrorf(t, err, "forEachUnit\n%s", out)
	require.Lenf(t, ran, 2, "ran units\n%s", out)
	assert.Equal(t, "1", ran[1].env["PLUGIN_GATE"], "unit env")
	assert.Equal(t, "1", gateInOp, "PLUGIN_GATE in plugin op")
	assert.Contains(t, ran[1].res.Scripts, "scripts/bootstrap.zsh")
	_, set := os.LookupEnv("PLUGIN_GATE")
	assert.False(t, set, "PLUGIN_GATE must not leak into the process env after forEachUnit")
	testutil.WantLines(t, out, "execIf(pass): profile gitlabGroup: env:PLUGIN_GATE")
}

// --skip-plugins (env CHE_SKIP_PLUGINS) drops plugins entries: only the local
// unit runs, no plugin pull or log line.
func TestBuildSkipPlugins(t *testing.T) {
	hostRepo := testutil.Repo(t, map[string]string{
		"che.yml":           "main:\n  options: {autoExec: true}\n  plugins: [\"@file:///nonexistent::main\"]\n  include:\n    runScripts: [scripts/local.zsh]\n",
		"scripts/local.zsh": "#!/bin/zsh\n",
	})
	pluginEnv(t, hostRepo)

	a := New()
	for name, set := range map[string]func(){
		"flag": func() { t.Setenv("CHE_SKIP_PLUGINS", ""); a.skipPlugins = true },
		"env":  func() { t.Setenv("CHE_SKIP_PLUGINS", "1"); a.skipPlugins = false },
	} {
		set()
		require.NoErrorf(t, a.build(), "[%s] build()", name)
		require.Emptyf(t, a.pluginRefs, "[%s] pluginRefs", name)
		ran, out, err := collectUnits(t, a)
		require.NoErrorf(t, err, "[%s] forEachUnit\n%s", name, out)
		require.Lenf(t, ran, 1, "[%s] plugins must be skipped\n%s", name, out)
		require.NotContainsf(t, out, "plugin", "[%s] plugins must be skipped", name)
	}
}

// a dir-path plugins ref (absolute and relative) anchors the unit at the dir
// itself: no git clone/pull, no cache checkout; its op lines carry the
// profile= subtype.
func TestBuildPluginUnitsDirRef(t *testing.T) {
	pluginDir := testutil.Tree(t, map[string]string{
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
			pluginEnv(t, hostRepo)
			t.Setenv("PLUGIN_DIR_REF", pluginDir)
			a := New()
			a.dryRunMode = "all"

			require.NoError(t, a.build())
			var ran []unit
			out, err := testutil.CaptureStdout(t, func() error {
				return a.forEachUnit("run-scripts", func(u unit) error {
					ran = append(ran, u)
					scripts, err := u.host.ResolveScripts(u.res.Scripts)
					if err != nil {
						return err
					}
					return u.host.RunScripts(scripts)
				})
			})
			require.NoErrorf(t, err, "forEachUnit\n%s", out)
			require.Lenf(t, ran, 2, "ran units\n%s", out)
			want := pluginDir
			if name == "relative" {
				want = filepath.Join(hostRepo, "plugin")
			}
			got, _ := filepath.EvalSymlinks(ran[1].host.RepoRoot)
			assert.Equal(t, mustEval(t, want), got, "plugin RepoRoot")
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
	require.NoError(t, err)
	return out
}

// a failing unit does not stop the rest: both units run, each failure prints a
// "<name>(report): fail <ref>" line, and forEachUnit returns joined errors.
func TestForEachUnitContinuesOnError(t *testing.T) {
	pluginDir := testutil.Tree(t, map[string]string{
		"che.yml":               "gitlabGroup:\n  include:\n    runScripts: [scripts/bootstrap.zsh]\n",
		"scripts/bootstrap.zsh": "#!/bin/zsh\n",
	})
	ref := pluginDir + "::gitlabGroup"
	hostRepo := testutil.Repo(t, map[string]string{
		"che.yml":           "main:\n  options: {autoExec: true}\n  plugins: [\"" + ref + "\"]\n  include:\n    runScripts: [scripts/local.zsh]\n",
		"scripts/local.zsh": "#!/bin/zsh\n",
	})
	pluginEnv(t, hostRepo)
	a := New()

	require.NoError(t, a.build())
	var ran []unit
	out, err := testutil.CaptureStdout(t, func() error {
		return a.forEachUnit("test", func(u unit) error {
			ran = append(ran, u)
			if u.ref == "" {
				return errors.New("local boom")
			}
			return nil
		})
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local boom", "joined errors must carry the local failure")
	require.Lenf(t, ran, 2, "continue past local failure\n%s", out)
	testutil.WantLines(t, out, "test(report): fail local: local boom")
}

// withEnv shadows an existing var, sets an absent one, restores both after.
func TestUnitWithEnv(t *testing.T) {
	t.Setenv("WITHENV_SHADOWED", "host")
	t.Setenv("WITHENV_ABSENT", "")
	os.Unsetenv("WITHENV_ABSENT")
	u := unit{env: map[string]string{"WITHENV_SHADOWED": "plugin", "WITHENV_ABSENT": "x"}}
	err := u.withEnv(func() error {
		assert.Equal(t, "plugin", os.Getenv("WITHENV_SHADOWED"), "shadowed")
		assert.Equal(t, "x", os.Getenv("WITHENV_ABSENT"), "absent")
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "host", os.Getenv("WITHENV_SHADOWED"), "shadowed after")
	_, set := os.LookupEnv("WITHENV_ABSENT")
	assert.False(t, set, "absent var must be unset after withEnv")
}

// [<] 🤖🤖
