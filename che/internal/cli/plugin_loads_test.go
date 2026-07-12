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
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// pluginEnv anchors a plugin test in hostRepo: temp HOME, CHE_* cleared,
// executor mocked. Returns HOME.
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

func mustEval(t *testing.T, p string) string {
	t.Helper()
	out, err := filepath.EvalSymlinks(p)
	require.NoError(t, err)
	return out
}

type loadKnobs struct {
	Op          string   `yaml:"op"`
	SampleEnv   string   `yaml:"sampleEnv"`
	UnsetEnv    []string `yaml:"unsetEnv"`
	SkipPlugins bool     `yaml:"skipPlugins"`
}

type loadsWant struct {
	RanLoads      int               `yaml:"ranLoads"`
	PluginRefs    *int              `yaml:"pluginRefs"`
	Ref           string            `yaml:"ref"`
	RepoRoot      string            `yaml:"repoRoot"`
	RepoRootUnder string            `yaml:"repoRootUnder"`
	Script        string            `yaml:"script"`
	Env           map[string]string `yaml:"env"`
	EnvInOp       string            `yaml:"envInOp"`
}

func TestForEachLoad(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/for_each_load.test.spec.yml",
		func(t *testing.T, c testyml.Case[loadsWant]) {
			testutil.RequireRegistered(t, c.Context.MockedInterfaces)
			var pluginFiles, hostFiles map[string]string
			var ref string
			var knobs loadKnobs
			c.Input.Args.To(t, 0, &pluginFiles)
			c.Input.Args.To(t, 1, &hostFiles)
			c.Input.Args.To(t, 2, &ref)
			c.Input.Args.To(t, 3, &knobs)

			vars := map[string]string{}
			if len(pluginFiles) > 0 {
				vars["PLUGIN_DIR"] = testutil.Repo(t, pluginFiles)
			}
			ref = testyml.Expand(ref, vars)
			vars["REF"] = ref
			hostTree := map[string]string{}
			for rel, body := range hostFiles {
				hostTree[rel] = testyml.Expand(body, vars)
			}
			hostRepo := testutil.Repo(t, hostTree)
			home := pluginEnv(t, hostRepo)
			vars["HOST_REPO"] = hostRepo
			vars["CACHE"] = filepath.Join(home, ".local/share/che/plugins")
			for k, v := range c.Context.Env {
				t.Setenv(k, v)
			}
			for _, k := range knobs.UnsetEnv {
				t.Setenv(k, "")
				os.Unsetenv(k)
			}
			if d, ok := vars["PLUGIN_DIR"]; ok {
				t.Setenv("PLUGIN_DIR_REF", d)
			}

			a := New()
			a.config.SkipPlugins = knobs.SkipPlugins
			buildOut, err := testutil.CaptureStdout(t, a.init)
			require.NoErrorf(t, err, "init()\n%s", buildOut)
			require.Empty(t, a.plugins.built, "init() must defer plugins")
			require.NotContains(t, buildOut, "plugin", "init() must defer plugins")
			if c.Expected.Output.PluginRefs != nil {
				assert.Len(t, a.plugins.refs, *c.Expected.Output.PluginRefs, "pluginRefs")
			}

			envBefore := map[string]string{}
			for _, kv := range os.Environ() {
				k, v, _ := strings.Cut(kv, "=")
				envBefore[k] = v
			}
			var ran []load
			envInOp := ""
			op := func(u load) error {
				ran = append(ran, u)
				if knobs.SampleEnv != "" && u.pluginRef != "" {
					envInOp = os.Getenv(knobs.SampleEnv)
				}
				switch knobs.Op {
				case "failLocal":
					if u.pluginRef == "" {
						return errors.New("local boom")
					}
				case "runScripts":
					scripts, err := u.host.ResolveScripts(u.selection.Scripts)
					if err != nil {
						return err
					}
					return u.host.RunScripts(scripts)
				}
				return nil
			}
			opName := "test"
			if knobs.Op == "runScripts" {
				opName = "run-scripts"
			}
			out, ferr := testutil.CaptureStdout(t, func() error { return a.forEachLoad(opName, op) })
			for _, u := range ran {
				for k := range u.pluginEnv {
					cur, set := os.LookupEnv(k)
					prev, had := envBefore[k]
					assert.Falsef(t, set != had || cur != prev,
						"%s must not leak into the process env after forEachLoad", k)
				}
			}
			if c.Expected.IsErrorWanted() {
				c.Expected.Check(t, ferr)
			} else {
				require.NoErrorf(t, ferr, "forEachLoad\n%s", out)
			}
			stripped := testutil.StripStamps(testutil.StripANSI(out))
			for _, m := range c.Expected.StdOut {
				testyml.MustMatch(t, stripped, testyml.Expand(m, vars))
			}
			for _, m := range c.NotExpected.StdOut {
				testyml.MustNotMatch(t, stripped, testyml.Expand(m, vars))
			}

			w := c.Expected.Output
			require.Lenf(t, ran, w.RanLoads, "ran repos\n%s", out)
			var pu *load
			for i := range ran {
				if ran[i].pluginRef != "" {
					pu = &ran[i]
				}
			}
			if w.Ref != "" || w.RepoRoot != "" || w.RepoRootUnder != "" || w.Script != "" || w.Env != nil {
				require.NotNilf(t, pu, "no plugin repo ran\n%s", out)
			}
			if w.Ref != "" {
				assert.Equal(t, testyml.Expand(w.Ref, vars), pu.pluginRef, "plugin ref")
			}
			if w.RepoRootUnder != "" {
				prefix := testyml.Expand(w.RepoRootUnder, vars) + "/"
				assert.Truef(t, strings.HasPrefix(pu.host.RepoRoot, prefix),
					"plugin RepoRoot = %q, want under %q", pu.host.RepoRoot, prefix)
			}
			if w.RepoRoot != "" {
				got, _ := filepath.EvalSymlinks(pu.host.RepoRoot)
				assert.Equal(t, mustEval(t, testyml.Expand(w.RepoRoot, vars)), got, "plugin RepoRoot")
			}
			if w.Script != "" {
				assert.Contains(t, pu.selection.Scripts, w.Script)
			}
			if w.Env != nil {
				assert.Equal(t, w.Env, pu.pluginEnv, "repo env")
			}
			if knobs.SampleEnv != "" {
				assert.Equal(t, w.EnvInOp, envInOp, knobs.SampleEnv+" in plugin op")
			}
		})
}

type runWithPluginEnvWant struct {
	During     map[string]string `yaml:"during"`
	After      map[string]string `yaml:"after"`
	UnsetAfter []string          `yaml:"unsetAfter"`
}

func TestRunWithEnv(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/run_with_plugin_env.test.spec.yml",
		func(t *testing.T, c testyml.Case[runWithPluginEnvWant]) {
			for k, v := range c.Context.Env {
				t.Setenv(k, v)
			}
			var env map[string]string
			c.Input.Args.To(t, 0, &env)
			for _, k := range c.Input.Args.Strings(t, 1) {
				t.Setenv(k, "")
				os.Unsetenv(k)
			}
			u := load{pluginEnv: env}
			during := map[string]string{}
			err := u.runWithPluginEnv(func() error {
				for k := range c.Expected.Output.During {
					during[k] = os.Getenv(k)
				}
				return nil
			})
			require.NoError(t, err)
			assert.Equal(t, c.Expected.Output.During, during)
			for k, v := range c.Expected.Output.After {
				assert.Equal(t, v, os.Getenv(k), k+" after runWithPluginEnv")
			}
			for _, k := range c.Expected.Output.UnsetAfter {
				_, set := os.LookupEnv(k)
				assert.False(t, set, k+" must be unset after runWithPluginEnv")
			}
		})
}

// [<] 🤖🤖
