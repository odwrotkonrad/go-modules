package che

// [>] 🤖🤖

import (
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

// prepEnv anchors a PrepareSpecs test: temp HOME, executor mocked. Returns HOME
// and the base launch env (HOME set, CHE_*/XDG_* absent) for building the case's
// Context.
func prepEnv(t *testing.T) (string, map[string]string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("non-root path only; build resolves home from $HOME")
	}
	home := t.TempDir()
	// [why] clear XDG/CHE base-dir vars so spec-side cache-home resolution
	// (ambient, out of scope) anchors under the temp HOME, not the real one.
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

// newContext builds the launch context a case feeds to the real
// PrepareApplicationOptions/PrepareSpecs: env (base + case), cwd = host repo, euid from
// the process (the same top-edge construction production does).
func newContext(env map[string]string, cwd string) Context {
	return Context{Env: env, Cwd: cwd, Euid: os.Geteuid()}
}

type prepWant struct {
	Profiles      []string            `yaml:"profiles"` // Ref() list, tree order
	Rejected      map[string]string   `yaml:"rejected"` // runIf-rejected ref -> rejecting condition
	RepoRoot      string              `yaml:"repoRoot"` // sourced profile's directory
	RepoRootUnder string              `yaml:"repoRootUnder"`
	Script        string              `yaml:"script"`    // rel suffix of a sourced profile's resolved script
	Env           map[string]string   `yaml:"env"`       // sourced profile's env overlay
	SampleEnv     string              `yaml:"sampleEnv"` // key sampled from the profile's captured env
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
			dests = append(dests, p.toDest(spec.DestRel(l)))
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

// prepFlags decodes the case's flags arg: the CLI/env inputs that used to be
// knobs, now driven through the real front door (options flags + Context env).
type prepFlags struct {
	SkipRemoteRefs bool     `yaml:"skipRemoteRefs"`
	ValidateSpec   string   `yaml:"validateSpec"`
	Debug          bool     `yaml:"debug"`
	UnsetEnv       []string `yaml:"unsetEnv"` // keys omitted from Context.Env
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
			// production: base (HOME) + case env, ref-dir var, unset keys omitted.
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
			// reads the process env ambiently, outside this refactor's scope; keep
			// it consistent with the launch ctx so those reads match production.
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
			opts := options.Options{
				AutoDiscover:    true,
				SkipRemoteRefs:  flags.SkipRemoteRefs,
				ValidateSpec:    vs,
				ValidateSpecCLI: vs, // models the flag/env override
				Debug:           flags.Debug,
			}
			log.SetDebug(flags.Debug)
			t.Cleanup(func() { log.SetDebug(false) })
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
				line, err := testutil.CaptureStdout(t, func() error { root.LogRejected(); return nil })
				require.NoError(t, err)
				for ref, cond := range want {
					assert.Contains(t, testutil.StripANSI(line),
						"discover-profiles(noMatchDue["+cond+"]): "+ref, "per-profile noMatch line")
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
				// env: block; the sampled key reads from that captured env.
				assert.Equal(t, w.EnvInOverlay, sp.env[w.SampleEnv], w.SampleEnv+" in the captured env")
			}
		})
}

// TestPrepareOptionsPrecedence: flags > env vars > local che.yml options: >
// defaults, driving Context.Env instead of the process env.
func TestPrepareOptionsPrecedence(t *testing.T) {
	repo := testutil.Repo(t, map[string]string{
		"che.yml": "options:\n  validateSpec: error\n  debug: true\np:\n  options: {autoDiscover: true}\n",
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
	assert.True(t, opts.Debug, "yaml debug over default")

	_, opts, err = PrepareApplicationOptions(ctx(map[string]string{"CHE_VALIDATE_SPEC": "warn"}), options.Options{})
	require.NoError(t, err)
	assert.Equal(t, options.ValidateSpec.Warn, opts.ValidateSpec, "env var over yaml")

	_, opts, err = PrepareApplicationOptions(ctx(map[string]string{"CHE_VALIDATE_SPEC": "warn"}), options.Options{ValidateSpec: options.ValidateSpec.Error})
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
	_, baseEnv := prepEnv(t)

	// [why] XDG_CONFIG_HOME steers UserConfigPath, which resolves the config
	// base ambiently (out of this refactor's scope); the CHE_* option env goes
	// through Context.Env like production.
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

	ctx := func(extra map[string]string) Context {
		env := map[string]string{}
		maps.Copy(env, baseEnv)
		maps.Copy(env, extra)
		return newContext(env, repo)
	}

	// user-config over the local spec + defaults.
	_, opts, err := PrepareApplicationOptions(ctx(nil), options.Options{})
	require.NoError(t, err)
	assert.Equal(t, options.ValidateSpec.Error, opts.ValidateSpec, "user-config validateSpec over spec")
	assert.True(t, opts.Debug, "user-config debug")
	assert.Equal(t, options.DryRun.Delta, opts.DryRun, "user-config dryRun")
	assert.True(t, opts.SkipRemoteRefs, "user-config skipRemoteRefs")
	assert.True(t, opts.RenderSkipSecrets, "user-config renderTemplates.skipSecrets")
	assert.True(t, opts.AutoDiscover, "user-config autoDiscover")
	assert.Equal(t, []string{"cfg/a", "cfg/b"}, opts.Profiles, "user-config profiles over spec")

	// env over user-config.
	_, opts, err = PrepareApplicationOptions(ctx(map[string]string{"CHE_VALIDATE_SPEC": "warn", "CHE_PROFILE": "env/a,env/b"}), options.Options{})
	require.NoError(t, err)
	assert.Equal(t, options.ValidateSpec.Warn, opts.ValidateSpec, "env over user-config")
	assert.Equal(t, []string{"env/a", "env/b"}, opts.Profiles, "CHE_PROFILE over user-config")

	// flags over env + user-config.
	_, opts, err = PrepareApplicationOptions(ctx(map[string]string{"CHE_VALIDATE_SPEC": "warn", "CHE_PROFILE": "env/a,env/b"}), options.Options{Profiles: []string{"flag/a"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"flag/a"}, opts.Profiles, "flag over env + user-config")

	// --dry-run=true aliases delta.
	_, opts, err = PrepareApplicationOptions(ctx(map[string]string{"CHE_DRY_RUN": "true"}), options.Options{})
	require.NoError(t, err)
	assert.Equal(t, options.DryRun.Delta, opts.DryRun, "dry-run true aliases delta")
}

// TestWorkingDirectoryCascade: profile > spec > che for options.profileWorkingDirectory,
// and the resolved tree feeds classification (links come from it).
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
	// p: inherits the spec-level workingDirectory.
	assert.Equal(t, eval(filepath.Join(repo, "spectree")), eval(byName["p"].workingDir))
	// q: its own workingDirectory wins.
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

	// che level (flag) seeds the default when the spec omits it.
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

// TestCheLevelWorkingDirectoryDoesNotLeakIntoSourcedSpec: the che-level
// workingDirectory (flag / root spec) seeds only the root spec; a sourced spec
// resolves against its own checkout, where the root's tree name does not exist.
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

	// che-level workingDirectory "roottree" exists in host, NOT in ref. Before the
	// fix it leaked into the sourced spec and failed resolving ref/roottree.
	root, err := PrepareSpecs(newContext(baseEnv, host), options.Options{SkipRunIf: true, AutoDiscover: true, ProfileWorkingDirectory: "roottree"}, spec.SpecSourceRecipe{})
	require.NoError(t, err)

	byName := profileByName(root.AllProfiles())
	wantRefWD, _ := filepath.EvalSymlinks(ref)
	gotRefWD, _ := filepath.EvalSymlinks(byName["s"].workingDir)
	assert.Equal(t, wantRefWD, gotRefWD, "sourced spec anchors at its own checkout, not che-level roottree")
}

// TestAutoDiscoverSwitch: autoDiscover=false disables discovery wholesale:
// bare runs error, forced profiles still work.
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

// TestOverlayEnv: the overlay wins over the base, adds absent keys, and leaves
// the base map untouched (pure derivation, no process mutation).
func TestOverlayEnv(t *testing.T) {
	base := map[string]string{"SHADOWED": "host", "KEPT": "base"}
	got := overlayEnv(base, map[string]string{"SHADOWED": "ref", "ADDED": "x"})
	assert.Equal(t, map[string]string{"SHADOWED": "ref", "KEPT": "base", "ADDED": "x"}, got)
	assert.Equal(t, map[string]string{"SHADOWED": "host", "KEPT": "base"}, base, "base map unchanged")

	// empty overlay returns the base unchanged.
	assert.Equal(t, base, overlayEnv(base, nil))
}

// stubOperation records execution order for ExecOperations tests.
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

// TestExecOperations: run order, Selected gating, error join without stopping.
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
	out, err := testutil.CaptureStdout(t, func() error { return p.ExecOperations(context.Background()) })
	require.ErrorIs(t, err, boom)
	assert.Equal(t, []string{"one", "failing", "last"}, ran, "run order, failure does not stop")
	assert.Contains(t, testutil.StripANSI(out), "run(runOp): one")
	assert.NotContains(t, testutil.StripANSI(out), "run(runOp): skipped")
}

// TestExecOperationsSkipOpsNoSweep: a skipped deselected op takes the skip
// branch, not the emptied-op sweep ([why] cli specs run ledger-off, so only
// this branch-order assertion pins the no-sweep guarantee; skip visibility
// itself is covered by the cmds specs).
func TestExecOperationsSkipOpsNoSweep(t *testing.T) {
	var ran []string
	p := &ProfileReady{
		ref:  "p",
		opts: options.Options{SkipOps: []string{"render-templates"}, Debug: true},
		OperationsReady: []operationReady{
			&stubOperation{name: "render-templates", selected: false, ran: &ran},
		},
	}
	out, err := testutil.CaptureStdout(t, func() error { return p.ExecOperations(context.Background()) })
	require.NoError(t, err)
	assert.Empty(t, ran)
	stripped := testutil.StripANSI(out)
	assert.Contains(t, stripped, "run(runOp, skippedDue[config.skipOps]): render-templates")
	assert.NotContains(t, stripped, "(nothing selected)", "skip wins over the deselected sweep branch")
}

// TestExecEachSkipsZeroDeltaProfile: a profile whose command ops carry no
// delta is skipped wholesale, announced skippedDueNoDelta; profiles with delta
// still run.
func TestExecEachSkipsZeroDeltaProfile(t *testing.T) {
	var ran []string
	mk := func(ref string, delta int) *ProfileReady {
		return &ProfileReady{ref: ref, OperationsReady: []operationReady{
			&stubOperation{name: "make-links", selected: true, delta: delta, ran: &ran},
		}}
	}
	s := &SpecReady{Profiles: []*ProfileReady{mk("settled", 0), mk("drifted", 1)}}
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
	assert.Contains(t, stripped, "run(runProfile, skippedDue[NoDelta]): settled: [make-links(0)]")
	assert.Contains(t, stripped, "run(runProfile): drifted: [make-links(1)]")
}

// [<] 🤖🤖
