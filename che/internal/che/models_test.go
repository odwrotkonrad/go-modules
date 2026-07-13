package che

// [>] 🤖🤖

import (
	"embed"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
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
		"CHE_CACHE_HOME", "CHE_STATE_HOME", "CHE_DATA_HOME",
		"XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME",
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
	Profiles      []string            `yaml:"profiles"` // Ref() list, tree order
	RepoRoot      string              `yaml:"repoRoot"` // sourced profile's directory
	RepoRootUnder string              `yaml:"repoRootUnder"`
	Script        string              `yaml:"script"` // rel suffix of a sourced profile's resolved script
	Env           map[string]string   `yaml:"env"`    // sourced profile's env overlay
	EnvInOverlay  string              `yaml:"envInOverlay"`
	LinkDests     map[string][]string `yaml:"linkDests"` // profile name -> its resolved link dests (workingDir + host mapping)
}

// profileByName indexes prepared profiles by their bare (unqualified) name.
func profileByName(ps []*ProfileReady) map[string]*ProfileReady {
	by := map[string]*ProfileReady{}
	for _, p := range ps {
		by[p.Source.GetProfileName()] = p
	}
	return by
}

// linkDests resolves a profile's link items through its own toDest (workingDir +
// HOME/system-root mapping), sorted for stable comparison.
func linkDests(t *testing.T, p *ProfileReady) []string {
	t.Helper()
	var dests []string
	for _, op := range p.OperationsReady {
		lo, ok := op.(*MakeLinksOperationReady)
		if !ok {
			continue
		}
		for _, l := range lo.Links {
			dests = append(dests, p.toDest(l.Rel))
		}
	}
	sort.Strings(dests)
	return dests
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
			vars["HOME"] = home
			vars["CACHE"] = filepath.Join(home, ".cache/che/sources")
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
			vs := options.ValidateSpecMode(testyml.Expand(knobs.ValidateSpec, vars))
			opts := options.Options{
				SkipRemoteRefs:  knobs.SkipRemoteRefs,
				ValidateSpec:    vs,
				ValidateSpecCLI: vs, // knob models the flag/env override
				Debug:           knobs.Debug,
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
			if w.LinkDests != nil {
				by := profileByName(profiles)
				for name, wantDests := range w.LinkDests {
					pr := by[name]
					require.NotNilf(t, pr, "profile %q not prepared\n%s", name, out)
					var want []string
					for _, d := range wantDests {
						want = append(want, testyml.Expand(d, vars))
					}
					sort.Strings(want)
					assert.Equalf(t, want, linkDests(t, pr), "profile %q link dests\n%s", name, out)
				}
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

	opts, err := PrepareOptions(options.Options{})
	require.NoError(t, err)
	assert.Equal(t, options.ValidateSpec.Error, opts.ValidateSpec, "yaml layer over default")
	assert.True(t, opts.Debug, "yaml debug over default")

	t.Setenv("CHE_VALIDATE_SPEC", "warn")
	opts, err = PrepareOptions(options.Options{})
	require.NoError(t, err)
	assert.Equal(t, options.ValidateSpec.Warn, opts.ValidateSpec, "env var over yaml")

	opts, err = PrepareOptions(options.Options{ValidateSpec: options.ValidateSpec.Error})
	require.NoError(t, err)
	assert.Equal(t, options.ValidateSpec.Error, opts.ValidateSpec, "flag over env var")
}

// TestPrepareOptionsUserConfig: the user-config file ($XDG_CONFIG_HOME/che/
// config.yml) resolves under env and flags, over the local che.yml options:
// block. A real config file drives every field.
func TestPrepareOptionsUserConfig(t *testing.T) {
	repo := testutil.Repo(t, map[string]string{
		"che.yml": "options:\n  validateSpec: warn\n  profiles: [spec/only]\np:\n  options: {autoDiscover: true}\n",
	})
	prepEnv(t, repo)

	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("CHE_CONFIG_HOME", "")
	cfgDir := filepath.Join(cfgHome, "che")
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))
	config := "validateSpec: error\n" +
		"debug: true\n" +
		"dryRun: delta\n" +
		"skipRemoteRefs: true\n" +
		"autoDiscover: true\n" +
		"profiles: [cfg/a, cfg/b]\n" +
		"renderTemplates:\n  skipSecrets: true\n"
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "config.yml"), []byte(config), 0o644))

	// user-config over the local spec + defaults.
	opts, err := PrepareOptions(options.Options{})
	require.NoError(t, err)
	assert.Equal(t, options.ValidateSpec.Error, opts.ValidateSpec, "user-config validateSpec over spec")
	assert.True(t, opts.Debug, "user-config debug")
	assert.Equal(t, options.DryRun.Delta, opts.DryRun, "user-config dryRun")
	assert.True(t, opts.SkipRemoteRefs, "user-config skipRemoteRefs")
	assert.True(t, opts.RenderSkipSecrets, "user-config renderTemplates.skipSecrets")
	require.NotNil(t, opts.AutoDiscover)
	assert.True(t, *opts.AutoDiscover, "user-config autoDiscover")
	assert.Equal(t, []string{"cfg/a", "cfg/b"}, opts.Profiles, "user-config profiles over spec")

	// env over user-config.
	t.Setenv("CHE_VALIDATE_SPEC", "warn")
	t.Setenv("CHE_PROFILE", "env/a,env/b")
	opts, err = PrepareOptions(options.Options{})
	require.NoError(t, err)
	assert.Equal(t, options.ValidateSpec.Warn, opts.ValidateSpec, "env over user-config")
	assert.Equal(t, []string{"env/a", "env/b"}, opts.Profiles, "CHE_PROFILE over user-config")

	// flags over env + user-config.
	opts, err = PrepareOptions(options.Options{Profiles: []string{"flag/a"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"flag/a"}, opts.Profiles, "flag over env + user-config")
}

// TestWorkingDirectoryCascade: profile > spec > che for options.workingDirectory,
// and the resolved tree feeds classification (links come from it).
func TestWorkingDirectoryCascade(t *testing.T) {
	repo := testutil.Repo(t, map[string]string{
		"che.yml": "options:\n  workingDirectory: spectree\n" +
			"p:\n  options: {autoDiscover: true}\n  include:\n    makeLinks: [HOME/**]\n" +
			"q:\n  options: {autoDiscover: true, workingDirectory: proftree}\n  include:\n    makeLinks: [HOME/**]\n",
		"spectree/HOME/.config/a": "a\n",
		"proftree/HOME/.config/b": "b\n",
	})
	home := prepEnv(t, repo)

	root, err := PrepareSpecs(options.Options{SkipExecIf: true}, spec.SpecSourceRecipe{})
	require.NoError(t, err)
	byName := map[string]*ProfileReady{}
	for _, pr := range root.AllProfiles() {
		byName[pr.Source.GetProfileName()] = pr
	}

	eval := func(p string) string { r, _ := filepath.EvalSymlinks(p); return r }
	// p: inherits the spec-level workingDirectory.
	assert.Equal(t, eval(filepath.Join(repo, "spectree")), eval(byName["p"].workingDir))
	// q: its own workingDirectory wins.
	assert.Equal(t, eval(filepath.Join(repo, "proftree")), eval(byName["q"].workingDir))

	linkDest := func(pr *ProfileReady) string {
		for _, op := range pr.OperationsReady {
			if lo, ok := op.(*MakeLinksOperationReady); ok && len(lo.Links) > 0 {
				return pr.toDest(lo.Links[0].Rel)
			}
		}
		return ""
	}
	assert.Equal(t, filepath.Join(home, ".config/a"), linkDest(byName["p"]), "p links from spectree")
	assert.Equal(t, filepath.Join(home, ".config/b"), linkDest(byName["q"]), "q links from proftree")

	// che level (flag) seeds the default when the spec omits it.
	repo2 := testutil.Repo(t, map[string]string{
		"che.yml":                "r:\n  options: {autoDiscover: true}\n  include:\n    makeLinks: [HOME/**]\n",
		"chetree/HOME/.config/c": "c\n",
	})
	prepEnv(t, repo2)
	root2, err := PrepareSpecs(options.Options{SkipExecIf: true, WorkingDirectory: "chetree"}, spec.SpecSourceRecipe{})
	require.NoError(t, err)
	wantWD, _ := filepath.EvalSymlinks(filepath.Join(repo2, "chetree"))
	gotWD, _ := filepath.EvalSymlinks(root2.AllProfiles()[0].workingDir)
	assert.Equal(t, wantWD, gotWD, "che-level flag default")
}

// TestAutoDiscoverGlobal: the user-config global autoDiscover discovers every
// profile that leaves it unset, but a profile's own autoDiscover: false wins.
func TestAutoDiscoverGlobal(t *testing.T) {
	repo := testutil.Repo(t, map[string]string{
		"che.yml": "unset:\n  options: {}\n" +
			"optout:\n  options: {autoDiscover: false}\n",
	})
	prepEnv(t, repo)

	yes := true
	root, err := PrepareSpecs(options.Options{SkipExecIf: true, AutoDiscover: &yes}, spec.SpecSourceRecipe{})
	require.NoError(t, err)
	var names []string
	for _, pr := range root.AllProfiles() {
		names = append(names, pr.Source.GetProfileName())
	}
	assert.Equal(t, []string{"unset"}, names, "global autoDiscover picks unset, profile opt-out wins")
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
func (o *stubOperation) execOperation(_ *ProfileReady) error {
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
