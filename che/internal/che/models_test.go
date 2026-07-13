package che

// [>] 🤖🤖

import (
	"embed"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/config"
	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/host"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

// prepEnv anchors a PrepareSpecs test in hostRepo: temp HOME, CHE_* cleared,
// executor mocked. Returns HOME.
func prepEnv(t *testing.T, hostRepo string) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("non-root path only; build resolves home from $HOME")
	}
	home := t.TempDir()
	t.Chdir(hostRepo)
	t.Setenv("HOME", home)
	for _, k := range []string{
		"CHE_SKIP_EXEC_IF", "CHE_PROFILE", "CHE_DRY_RUN",
		"CHE_SKIP_REMOTE_REFS", "CHE_DEBUG", "CHE_VALIDATE_SPEC", "CHE_DIR",
	} {
		t.Setenv(k, "")
	}
	execx.Swap(t, testutil.NewCmdMockExecutor())
	return home
}

type prepKnobs struct {
	SkipRemoteRefs bool     `yaml:"skipRemoteRefs"`
	SampleEnv      string   `yaml:"sampleEnv"`
	UnsetEnv       []string `yaml:"unsetEnv"`
	ValidateSpec   string   `yaml:"validateSpec"`
	Debug          bool     `yaml:"debug"`
}

type prepWant struct {
	Profiles      []string          `yaml:"profiles"` // Ref() list, tree order
	RepoRoot      string            `yaml:"repoRoot"` // sourced profile's directory
	RepoRootUnder string            `yaml:"repoRootUnder"`
	Script        string            `yaml:"script"` // rel suffix of a sourced profile's resolved script
	Env           map[string]string `yaml:"env"`    // sourced profile's env overlay
	EnvInOverlay  string            `yaml:"envInOverlay"`
}

// sourcedProfile: the first profile whose ref differs from its bare name.
func sourcedProfile(ps []*ProfileReady) *ProfileReady {
	for _, p := range ps {
		if p.Ref() != p.Source.GetProfileName() {
			return p
		}
	}
	return nil
}

func TestPrepareSpecs(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/prepare_specs.test.spec.yml",
		func(t *testing.T, c testyml.Case[prepWant]) {
			var refFiles, hostFiles map[string]string
			var knobs prepKnobs
			c.Input.Args.To(t, 0, &refFiles)
			c.Input.Args.To(t, 1, &hostFiles)
			c.Input.Args.To(t, 2, &knobs)

			vars := map[string]string{}
			if len(refFiles) > 0 {
				vars["REF_DIR"] = testutil.Repo(t, refFiles)
			}
			hostTree := map[string]string{}
			for rel, body := range hostFiles {
				hostTree[rel] = testyml.Expand(body, vars)
			}
			hostRepo := testutil.Repo(t, hostTree)
			home := prepEnv(t, hostRepo)
			vars["HOST_REPO"] = hostRepo
			vars["CACHE"] = filepath.Join(home, ".local/share/che/plugins")
			for k, v := range c.Context.Env {
				t.Setenv(k, v)
			}
			for _, k := range knobs.UnsetEnv {
				t.Setenv(k, "")
				os.Unsetenv(k)
			}
			if d, ok := vars["REF_DIR"]; ok {
				t.Setenv("REF_DIR_VAR", d)
			}

			envBefore := map[string]string{}
			for _, kv := range os.Environ() {
				k, v, _ := strings.Cut(kv, "=")
				envBefore[k] = v
			}
			opts := config.Options{
				SkipRemoteRefs: knobs.SkipRemoteRefs,
				ValidateSpec:   config.ValidateSpecMode(testyml.Expand(knobs.ValidateSpec, vars)),
				Debug:          knobs.Debug,
			}
			log.SetDebug(knobs.Debug)
			t.Cleanup(func() { log.SetDebug(false) })
			var root *SpecReady
			out, err := testutil.CaptureStdout(t, func() error {
				var e error
				root, e = PrepareSpecs(opts, spec.SpecSourceRecipe{})
				return e
			})
			if c.Expected.IsErrorWanted() {
				c.Expected.Check(t, err)
			} else {
				require.NoErrorf(t, err, "PrepareSpecs\n%s", out)
			}
			stripped := testutil.StripStamps(testutil.StripANSI(out))
			for _, m := range c.Expected.StdOut {
				testyml.MustMatch(t, stripped, testyml.Expand(m, vars))
			}
			for _, m := range c.NotExpected.StdOut {
				testyml.MustNotMatch(t, stripped, testyml.Expand(m, vars))
			}
			if err != nil {
				return
			}

			profiles := root.AllProfiles()
			w := c.Expected.Output
			if w.Profiles != nil {
				var refs []string
				for _, p := range profiles {
					refs = append(refs, testyml.Expand(p.Ref(), map[string]string{}))
				}
				var want []string
				for _, x := range w.Profiles {
					want = append(want, testyml.Expand(x, vars))
				}
				assert.Equal(t, want, refs, "profile refs\n%s", out)
			}
			sp := sourcedProfile(profiles)
			if w.RepoRoot != "" || w.RepoRootUnder != "" || w.Script != "" || w.Env != nil || w.EnvInOverlay != "" {
				require.NotNilf(t, sp, "no sourced profile prepared\n%s", out)
			}
			if w.RepoRoot != "" {
				got, _ := filepath.EvalSymlinks(sp.Source.DirectoryPath)
				want, e := filepath.EvalSymlinks(testyml.Expand(w.RepoRoot, vars))
				require.NoError(t, e)
				assert.Equal(t, want, got, "sourced profile dir")
			}
			if w.RepoRootUnder != "" {
				prefix := testyml.Expand(w.RepoRootUnder, vars) + "/"
				assert.Truef(t, strings.HasPrefix(sp.Source.DirectoryPath, prefix),
					"sourced dir = %q, want under %q", sp.Source.DirectoryPath, prefix)
			}
			if w.Script != "" {
				var scripts []string
				for _, op := range sp.OperationsReady {
					if rs, ok := op.(*RunScriptsOperationReady); ok {
						scripts = rs.Scripts
					}
				}
				assert.Truef(t, len(scripts) > 0 && strings.HasSuffix(scripts[0], w.Script),
					"scripts %v must end with %q", scripts, w.Script)
			}
			if w.Env != nil {
				assert.Equal(t, w.Env, sp.Env, "sourced profile env overlay")
			}
			if knobs.SampleEnv != "" {
				sampled := ""
				require.NoError(t, sp.withEnv(func() error {
					sampled = os.Getenv(knobs.SampleEnv)
					return nil
				}))
				assert.Equal(t, w.EnvInOverlay, sampled, knobs.SampleEnv+" inside the overlay")
				cur, set := os.LookupEnv(knobs.SampleEnv)
				prev, had := envBefore[knobs.SampleEnv]
				assert.Falsef(t, set != had || cur != prev,
					"%s must not leak into the process env after withEnv", knobs.SampleEnv)
			}
		})
}

// TestPrepareOptionsPrecedence: flags > env vars > local che.yml options: > defaults.
func TestPrepareOptionsPrecedence(t *testing.T) {
	repo := testutil.Repo(t, map[string]string{
		"che.yml": "options:\n  validateSpec: error\n  debug: true\np:\n  options: {autoDiscover: true}\n",
	})
	prepEnv(t, repo)

	opts, err := PrepareOptions(config.Options{})
	require.NoError(t, err)
	assert.Equal(t, config.ValidateSpec.Error, opts.ValidateSpec, "yaml layer over default")
	assert.True(t, opts.Debug, "yaml debug over default")

	t.Setenv("CHE_VALIDATE_SPEC", "warn")
	opts, err = PrepareOptions(config.Options{})
	require.NoError(t, err)
	assert.Equal(t, config.ValidateSpec.Warn, opts.ValidateSpec, "env var over yaml")

	opts, err = PrepareOptions(config.Options{ValidateSpec: config.ValidateSpec.Error})
	require.NoError(t, err)
	assert.Equal(t, config.ValidateSpec.Error, opts.ValidateSpec, "flag over env var")
}

// TestWithEnv: overlay shadows, sets, and restores the process env.
func TestWithEnv(t *testing.T) {
	t.Setenv("WITHENV_SHADOWED", "host")
	t.Setenv("WITHENV_ABSENT", "")
	os.Unsetenv("WITHENV_ABSENT")
	during := map[string]string{}
	err := withEnv(map[string]string{"WITHENV_SHADOWED": "ref", "WITHENV_ABSENT": "x"}, func() error {
		during["WITHENV_SHADOWED"] = os.Getenv("WITHENV_SHADOWED")
		during["WITHENV_ABSENT"] = os.Getenv("WITHENV_ABSENT")
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"WITHENV_SHADOWED": "ref", "WITHENV_ABSENT": "x"}, during)
	assert.Equal(t, "host", os.Getenv("WITHENV_SHADOWED"), "shadowed restored")
	_, set := os.LookupEnv("WITHENV_ABSENT")
	assert.False(t, set, "absent key unset after withEnv")
}

// stubOperation records execution order for ExecOperations tests.
type stubOperation struct {
	OperationReady
	name     string
	selected bool
	fail     error
	ran      *[]string
}

func (o *stubOperation) Name() string   { return o.name }
func (o *stubOperation) Selected() bool { return o.selected }
func (o *stubOperation) execOperation(_ host.Host) error {
	*o.ran = append(*o.ran, o.name)
	return o.fail
}

// TestExecOperations: run order, Selected gating, error join without stopping.
func TestExecOperations(t *testing.T) {
	var ran []string
	boom := errors.New("boom")
	p := &ProfileReady{
		ref: "p",
		OperationsReady: []operationReady{
			&stubOperation{name: "one", selected: true, ran: &ran},
			&stubOperation{name: "skipped", selected: false, ran: &ran},
			&stubOperation{name: "failing", selected: true, fail: boom, ran: &ran},
			&stubOperation{name: "last", selected: true, ran: &ran},
		},
	}
	out, err := testutil.CaptureStdout(t, p.ExecOperations)
	require.ErrorIs(t, err, boom)
	assert.Equal(t, []string{"one", "failing", "last"}, ran, "run order, failure does not stop")
	assert.Contains(t, testutil.StripANSI(out), "all(run): one")
	assert.NotContains(t, testutil.StripANSI(out), "all(run): skipped")
}

// [<] 🤖🤖
