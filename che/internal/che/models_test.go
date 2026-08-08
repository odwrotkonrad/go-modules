package che

// [>] 🤖🤖

import (
	"cmp"
	"context"
	"embed"
	"errors"
	"maps"
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

func prepEnv(t *testing.T) (string, map[string]string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("non-root path only; build resolves home from $HOME")
	}
	home := t.TempDir()
	// [why] clear XDG/CHE base-dir vars so spec-side cache-home resolution
	for _, k := range []string{
		"CHE_CACHE_HOME", "CHE_STATE_HOME", "CHE_DATA_HOME", "CHE_CONFIG_HOME",
		"XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME", "XDG_CONFIG_HOME",
	} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	execx.Swap(t, testutil.NewCmdMockExecutor())
	return home, map[string]string{"HOME": home}
}

func newContext(env map[string]string, cwd string) Context {
	return Context{Env: env, Cwd: cwd, Euid: os.Geteuid()}
}

type prepWant struct {
	Profiles      []string            `yaml:"profiles"`
	Rejected      map[string]string   `yaml:"rejected"`
	RepoRoot      string              `yaml:"repoRoot"`
	RepoRootUnder string              `yaml:"repoRootUnder"`
	Script        string              `yaml:"script"`
	Env           map[string]string   `yaml:"env"`
	SampleEnv     string              `yaml:"sampleEnv"`
	EnvInOverlay  string              `yaml:"envInOverlay"`
	LinkDests     map[string][]string `yaml:"linkDests"`
}

func profileByName(ps []*ProfileReady) map[string]*ProfileReady {
	by := map[string]*ProfileReady{}
	for _, p := range ps {
		by[p.Source.GetProfileName()] = p
	}
	return by
}

func linkDests(t *testing.T, p *ProfileReady) []string {
	t.Helper()
	var dests []string
	for _, op := range p.OperationsReady {
		lo, ok := op.(*MakeLinksOperationReady)
		if !ok {
			continue
		}
		for _, l := range lo.Links {
			dests = append(dests, p.toDest(spec.DestRel(l)))
		}
	}
	sort.Strings(dests)
	return dests
}

func sourcedProfile(ps []*ProfileReady) *ProfileReady {
	for _, p := range ps {
		if p.Ref() != p.Source.GetProfileName() {
			return p
		}
	}
	return nil
}

type prepFlags struct {
	SkipRemoteRefs bool     `yaml:"skipRemoteRefs"`
	ValidateSpec   string   `yaml:"validateSpec"`
	LogLevel       string   `yaml:"logLevel"`
	UnsetEnv       []string `yaml:"unsetEnv"`
}

func TestPrepareSpecs(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/prepare_specs.test.spec.yml",
		func(t *testing.T, c testyml.Case[prepWant]) {
			var refFiles, hostFiles map[string]string
			var flags prepFlags
			c.Input.Args.To(t, 0, &refFiles)
			c.Input.Args.To(t, 1, &hostFiles)
			c.Input.Args.To(t, 2, &flags)

			vars := map[string]string{}
			if len(refFiles) > 0 {
				vars["REF_DIR"] = testutil.Repo(t, refFiles)
				vars["REF_NAME"] = filepath.Base(vars["REF_DIR"])
			}
			hostTree := map[string]string{}
			for rel, body := range hostFiles {
				hostTree[rel] = testyml.Expand(body, vars)
			}
			hostRepo := testutil.Repo(t, hostTree)
			home, baseEnv := prepEnv(t)
			vars["HOST_REPO"] = hostRepo
			vars["HOME"] = home
			vars["CACHE"] = filepath.Join(home, ".cache/che/remote-sources")

			// [why] the launch env is built at the top edge exactly like
			env := map[string]string{}
			maps.Copy(env, baseEnv)
			if d, ok := vars["REF_DIR"]; ok {
				env["REF_DIR_VAR"] = d
			}
			for k, v := range c.Context.Env {
				env[k] = v
			}
			for _, k := range flags.UnsetEnv {
				delete(env, k)
			}
			// [why] spec-side source resolution (URI $VAR expand, XDG cache home)
			t.Chdir(hostRepo)
			for _, k := range flags.UnsetEnv {
				t.Setenv(k, "")
				os.Unsetenv(k)
			}
			for k, v := range env {
				t.Setenv(k, v)
			}
			ctx := newContext(env, hostRepo)

			vs := options.ValidateSpecMode(testyml.Expand(flags.ValidateSpec, vars))
			logLevel := cmp.Or(flags.LogLevel, "info")
			opts := options.Options{
				AutoDiscover:    true,
				SkipRemoteRefs:  flags.SkipRemoteRefs,
				ValidateSpec:    vs,
				ValidateSpecCLI: vs,
				LogLevel:        logLevel,
			}
			level, err := log.ParseLevel(logLevel)
			require.NoError(t, err)
			t.Cleanup(log.SwapLevel(level))
			var root *SpecReady
			out, err := testutil.CaptureStdout(t, func() error {
				var e error
				root, e = PrepareSpecs(ctx, opts, spec.SpecSourceRecipe{})
				return e
			})
			if c.Expected.IsErrorWanted() {
				c.Expected.Check(t, err)
			} else {
				require.NoErrorf(t, err, "PrepareSpecs\n%s", out)
			}
			stripped := testutil.StripANSI(out)
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
			if w.Rejected != nil {
				want := map[string]string{}
				for ref, cond := range w.Rejected {
					want[testyml.Expand(ref, vars)] = cond
				}
				got := map[string]string{}
				for _, r := range root.Rejected {
					got[r.Ref] = r.Cond
				}
				assert.Equal(t, want, got, "rejected profiles\n%s", out)
				restore := log.SwapLevel(log.Levels.Debug)
				line, err := testutil.CaptureStdout(t, func() error { root.LogRejected(); return nil })
				restore()
				require.NoError(t, err)
				for ref, cond := range want {
					assert.Contains(t, testutil.StripANSI(line),
						"will not run profile "+ref+": runIf failed: "+cond, "per-profile rejected line")
				}
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
			if w.SampleEnv != "" {
				// [why] the profile captured the launch env overlaid with its
				assert.Equal(t, w.EnvInOverlay, sp.env[w.SampleEnv], w.SampleEnv+" in the captured env")
			}
		})
}

func TestPrepareOptionsPrecedence(t *testing.T) {
	repo := testutil.Repo(t, map[string]string{
		"che.yml": "options:\n  validateSpec: error\n  logLevel: debug\np:\n  options: {autoDiscover: true}\n",
	})
	_, baseEnv := prepEnv(t)
	ctx := func(extra map[string]string) Context {
		env := map[string]string{}
		maps.Copy(env, baseEnv)
		maps.Copy(env, extra)
		return newContext(env, repo)
	}

	_, opts, err := PrepareApplicationOptions(ctx(nil), options.Options{})
	require.NoError(t, err)
	assert.Equal(t, options.ValidateSpec.Error, opts.ValidateSpec, "yaml layer over default")
	assert.Equal(t, "debug", opts.LogLevel, "yaml logLevel over default")

	_, opts, err = PrepareApplicationOptions(ctx(map[string]string{"CHE_VALIDATE_SPEC": "warn"}), options.Options{})
	require.NoError(t, err)
	assert.Equal(t, options.ValidateSpec.Warn, opts.ValidateSpec, "env var over yaml")

	_, opts, err = PrepareApplicationOptions(ctx(map[string]string{"CHE_VALIDATE_SPEC": "warn"}), options.Options{ValidateSpec: options.ValidateSpec.Error})
	require.NoError(t, err)
	assert.Equal(t, options.ValidateSpec.Error, opts.ValidateSpec, "flag over env var")
}

func TestPrepareOptionsUserConfig(t *testing.T) {
	repo := testutil.Repo(t, map[string]string{
		"che.yml": "options:\n  validateSpec: warn\n  profiles: [spec/only]\np:\n  options: {autoDiscover: true}\n",
	})
	_, baseEnv := prepEnv(t)

	// [why] XDG_CONFIG_HOME steers UserConfigPath, which resolves the config
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("CHE_CONFIG_HOME", "")
	cfgDir := filepath.Join(cfgHome, "che")
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))
	config := "validateSpec: error\n" +
		"logLevel: debug\n" +
		"dryRun: delta\n" +
		"skipRemoteRefs: true\n" +
		"autoDiscover: true\n" +
		"profiles: [cfg/a, cfg/b]\n" +
		"renderTemplates:\n  skipSecrets: true\n"
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "config.yml"), []byte(config), 0o644))

	ctx := func(extra map[string]string) Context {
		env := map[string]string{}
		maps.Copy(env, baseEnv)
		maps.Copy(env, extra)
		return newContext(env, repo)
	}

	_, opts, err := PrepareApplicationOptions(ctx(nil), options.Options{})
	require.NoError(t, err)
	assert.Equal(t, options.ValidateSpec.Error, opts.ValidateSpec, "user-config validateSpec over spec")
	assert.Equal(t, "debug", opts.LogLevel, "user-config logLevel")
	assert.Equal(t, options.DryRun.Delta, opts.DryRun, "user-config dryRun")
	assert.True(t, opts.SkipRemoteRefs, "user-config skipRemoteRefs")
	assert.True(t, opts.RenderSkipSecrets, "user-config renderTemplates.skipSecrets")
	assert.True(t, opts.AutoDiscover, "user-config autoDiscover")
	assert.Equal(t, []string{"cfg/a", "cfg/b"}, opts.Profiles, "user-config profiles over spec")

	_, opts, err = PrepareApplicationOptions(ctx(map[string]string{"CHE_VALIDATE_SPEC": "warn", "CHE_PROFILE": "env/a,env/b"}), options.Options{})
	require.NoError(t, err)
	assert.Equal(t, options.ValidateSpec.Warn, opts.ValidateSpec, "env over user-config")
	assert.Equal(t, []string{"env/a", "env/b"}, opts.Profiles, "CHE_PROFILE over user-config")

	_, opts, err = PrepareApplicationOptions(ctx(map[string]string{"CHE_VALIDATE_SPEC": "warn", "CHE_PROFILE": "env/a,env/b"}), options.Options{Profiles: []string{"flag/a"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"flag/a"}, opts.Profiles, "flag over env + user-config")

	_, opts, err = PrepareApplicationOptions(ctx(map[string]string{"CHE_DRY_RUN": "true"}), options.Options{})
	require.NoError(t, err)
	assert.Equal(t, options.DryRun.Delta, opts.DryRun, "dry-run true aliases delta")
}

func TestWorkingDirectoryCascade(t *testing.T) {
	repo := testutil.Repo(t, map[string]string{
		"che.yml": "options:\n  profileWorkingDirectory: spectree\n" +
			"p:\n  options: {autoDiscover: true}\n  include:\n    makeLinks: [{source: _home/**, dest: 's:^_home:$HOME:'}]\n" +
			"q:\n  options: {autoDiscover: true, profileWorkingDirectory: proftree}\n  include:\n    makeLinks: [{source: _home/**, dest: 's:^_home:$HOME:'}]\n",
		"spectree/_home/.config/a": "a\n",
		"proftree/_home/.config/b": "b\n",
	})
	home, baseEnv := prepEnv(t)

	root, err := PrepareSpecs(newContext(baseEnv, repo), options.Options{SkipRunIf: true, AutoDiscover: true}, spec.SpecSourceRecipe{})
	require.NoError(t, err)
	byName := map[string]*ProfileReady{}
	for _, pr := range root.AllProfiles() {
		byName[pr.Source.GetProfileName()] = pr
	}

	eval := func(p string) string { r, _ := filepath.EvalSymlinks(p); return r }
	assert.Equal(t, eval(filepath.Join(repo, "spectree")), eval(byName["p"].workingDir))
	assert.Equal(t, eval(filepath.Join(repo, "proftree")), eval(byName["q"].workingDir))

	linkDest := func(pr *ProfileReady) string {
		for _, op := range pr.OperationsReady {
			if lo, ok := op.(*MakeLinksOperationReady); ok && len(lo.Links) > 0 {
				return pr.toDest(spec.DestRel(lo.Links[0]))
			}
		}
		return ""
	}
	assert.Equal(t, filepath.Join(home, ".config/a"), linkDest(byName["p"]), "p links from spectree")
	assert.Equal(t, filepath.Join(home, ".config/b"), linkDest(byName["q"]), "q links from proftree")

	repo2 := testutil.Repo(t, map[string]string{
		"che.yml":                 "r:\n  options: {autoDiscover: true}\n  include:\n    makeLinks: [_home/**]\n",
		"chetree/_home/.config/c": "c\n",
	})
	_, baseEnv2 := prepEnv(t)
	root2, err := PrepareSpecs(newContext(baseEnv2, repo2), options.Options{SkipRunIf: true, AutoDiscover: true, ProfileWorkingDirectory: "chetree"}, spec.SpecSourceRecipe{})
	require.NoError(t, err)
	wantWD, _ := filepath.EvalSymlinks(filepath.Join(repo2, "chetree"))
	gotWD, _ := filepath.EvalSymlinks(root2.AllProfiles()[0].workingDir)
	assert.Equal(t, wantWD, gotWD, "che-level flag default")
}

func TestCheLevelWorkingDirectoryDoesNotLeakIntoSourcedSpec(t *testing.T) {
	ref := testutil.Repo(t, map[string]string{
		"che.yml":         "s:\n  options: {autoDiscover: true}\n  include:\n    makeLinks: [_home/**]\n",
		"_home/.config/x": "x\n",
	})
	host := testutil.Repo(t, map[string]string{
		"che.yml": "main:\n  options: {autoDiscover: true}\n  include:\n    profiles:\n" +
			"      - source: \"" + ref + "/che.yml::s\"\n",
		"roottree/_home/.config/c": "c\n",
	})
	_, baseEnv := prepEnv(t)

	root, err := PrepareSpecs(newContext(baseEnv, host), options.Options{SkipRunIf: true, AutoDiscover: true, ProfileWorkingDirectory: "roottree"}, spec.SpecSourceRecipe{})
	require.NoError(t, err)

	byName := profileByName(root.AllProfiles())
	wantRefWD, _ := filepath.EvalSymlinks(ref)
	gotRefWD, _ := filepath.EvalSymlinks(byName["s"].workingDir)
	assert.Equal(t, wantRefWD, gotRefWD, "sourced spec anchors at its own checkout, not che-level roottree")
}

func TestAutoDiscoverSwitch(t *testing.T) {
	repo := testutil.Repo(t, map[string]string{
		"che.yml": "marked:\n  options: {autoDiscover: true}\n",
	})
	_, baseEnv := prepEnv(t)

	_, err := PrepareSpecs(newContext(baseEnv, repo), options.Options{SkipRunIf: true}, spec.SpecSourceRecipe{})
	require.ErrorContains(t, err, "auto-discovery is disabled", "default-false zero value models autoDiscover=false")

	root, err := PrepareSpecs(newContext(baseEnv, repo), options.Options{SkipRunIf: true, AutoDiscover: true}, spec.SpecSourceRecipe{})
	require.NoError(t, err)
	require.Len(t, root.AllProfiles(), 1, "autoDiscover=true discovers marked profiles")

	root, err = PrepareSpecs(newContext(baseEnv, repo), options.Options{SkipRunIf: true, Profiles: []string{"marked"}}, spec.SpecSourceRecipe{})
	require.NoError(t, err)
	require.Len(t, root.AllProfiles(), 1, "forced profiles bypass the disabled discovery")
}

func TestOverlayEnv(t *testing.T) {
	base := map[string]string{"SHADOWED": "host", "KEPT": "base"}
	got := overlayEnv(base, map[string]string{"SHADOWED": "ref", "ADDED": "x"})
	assert.Equal(t, map[string]string{"SHADOWED": "ref", "KEPT": "base", "ADDED": "x"}, got)
	assert.Equal(t, map[string]string{"SHADOWED": "host", "KEPT": "base"}, base, "base map unchanged")

	assert.Equal(t, base, overlayEnv(base, nil))
}

type stubOperation struct {
	OperationReady
	name     string
	selected bool
	delta    int
	fail     error
	ran      *[]string
}

func (o *stubOperation) Name() string                      { return o.name }
func (o *stubOperation) Selected() bool                    { return o.selected }
func (o *stubOperation) counts(_ *ProfileReady) (int, int) { return 1, o.delta }
func (o *stubOperation) execOperation(_ *ProfileReady) error {
	*o.ran = append(*o.ran, o.name)
	return o.fail
}

func TestExecOperations(t *testing.T) {
	var ran []string
	boom := errors.New("boom")
	p := &ProfileReady{
		ref: "p",
		OperationsReady: []operationReady{
			&stubOperation{name: "one", selected: true, delta: 1, ran: &ran},
			&stubOperation{name: "skipped", selected: false, ran: &ran},
			&stubOperation{name: "failing", selected: true, delta: 1, fail: boom, ran: &ran},
			&stubOperation{name: "last", selected: true, delta: 1, ran: &ran},
		},
	}
	t.Cleanup(log.SwapLevel(log.Levels.Debug))
	out, err := testutil.CaptureStdout(t, func() error { return p.ExecOperations(context.Background()) })
	require.ErrorIs(t, err, boom)
	assert.Equal(t, []string{"one", "failing", "last"}, ran, "run order, failure does not stop")
	assert.Contains(t, testutil.StripANSI(out), "# one")
	assert.NotContains(t, testutil.StripANSI(out), "# skipped")
}

func TestExecOperationsSkipOpsNoSweep(t *testing.T) {
	var ran []string
	p := &ProfileReady{
		ref:  "p",
		opts: options.Options{SkipOps: []string{"render-templates"}},
		OperationsReady: []operationReady{
			&stubOperation{name: "render-templates", selected: false, ran: &ran},
		},
	}
	t.Cleanup(log.SwapLevel(log.Levels.Debug))
	out, err := testutil.CaptureStdout(t, func() error { return p.ExecOperations(context.Background()) })
	require.NoError(t, err)
	assert.Empty(t, ran)
	stripped := testutil.StripANSI(out)
	assert.Contains(t, stripped, "will not run op render-templates: options.skipOps")
	assert.NotContains(t, stripped, "(nothing selected)", "skip wins over the deselected sweep branch")
}

func TestExecEachSkipsZeroDeltaProfile(t *testing.T) {
	var ran []string
	mk := func(ref string, delta int) *ProfileReady {
		return &ProfileReady{ref: ref, OperationsReady: []operationReady{
			&stubOperation{name: "make-links", selected: true, delta: delta, ran: &ran},
		}}
	}
	s := &SpecReady{Profiles: []*ProfileReady{mk("settled", 0), mk("drifted", 1)}}
	t.Cleanup(log.SwapLevel(log.Levels.Debug))
	var executed []string
	out, err := testutil.CaptureStdout(t, func() error {
		return s.ExecEach(context.Background(), "run", func(_ context.Context, p *ProfileReady) error {
			executed = append(executed, p.Ref())
			return nil
		})
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"drifted"}, executed, "zero-delta profile not executed")
	stripped := testutil.StripANSI(out)
	assert.Contains(t, stripped, "will not run profile settled: no changes")
	assert.Contains(t, stripped, "will run profile drifted: make-links (1 change)")
}

// [<] 🤖🤖

func TestFindSpecRoot(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "che.yml"), []byte("x"), 0o644))

	got, err := findSpecRoot(filepath.Join(dir, "a", "b"))
	require.NoError(t, err)
	want, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	gotEval, err := filepath.EvalSymlinks(got)
	require.NoError(t, err)
	require.Equal(t, want, gotEval)

	_, err = findSpecRoot(filepath.Join(os.TempDir(), "definitely-no-spec-here"))
	require.Error(t, err)
}
