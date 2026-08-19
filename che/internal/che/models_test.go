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
	// [why] clear XDG/CHE base-dir vars so spec-side cache-home resolution falls back to $HOME
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
	WorkingDirs   map[string]string   `yaml:"workingDirs"`
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
	SkipRemoteRefs          bool     `yaml:"skipRemoteRefs"`
	ValidateSpec            string   `yaml:"validateSpec"`
	LogLevel                string   `yaml:"logLevel"`
	UnsetEnv                []string `yaml:"unsetEnv"`
	AutoDiscover            *bool    `yaml:"autoDiscover"`
	Profiles                []string `yaml:"profiles"`
	ProfileWorkingDirectory string   `yaml:"profileWorkingDirectory"`
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

			// [why] the launch env is built at the top edge exactly like the CLI does
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
			// [why] spec-side source resolution (URI $VAR expand, XDG cache home) reads the process env
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
				AutoDiscover:            flags.AutoDiscover == nil || *flags.AutoDiscover,
				Profiles:                flags.Profiles,
				ProfileWorkingDirectory: flags.ProfileWorkingDirectory,
				SkipRemoteRefs:          flags.SkipRemoteRefs,
				ValidateSpec:            vs,
				ValidateSpecCLI:         vs,
				LogLevel:                logLevel,
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
			if w.WorkingDirs != nil {
				by := profileByName(profiles)
				for name, wantDir := range w.WorkingDirs {
					pr := by[name]
					require.NotNilf(t, pr, "profile %q not prepared\n%s", name, out)
					wantEval, _ := filepath.EvalSymlinks(testyml.Expand(wantDir, vars))
					gotEval, _ := filepath.EvalSymlinks(pr.workingDir)
					assert.Equalf(t, wantEval, gotEval, "profile %q working dir", name)
				}
			}
			if w.SampleEnv != "" {
				// [why] the profile captured the launch env overlaid with its spec env
				assert.Equal(t, w.EnvInOverlay, sp.env[w.SampleEnv], w.SampleEnv+" in the captured env")
			}
		})
}

type prepareOptsGot struct {
	ValidateSpec      string   `yaml:"validateSpec,omitempty"`
	LogLevel          string   `yaml:"logLevel,omitempty"`
	DryRun            string   `yaml:"dryRun,omitempty"`
	SkipRemoteRefs    bool     `yaml:"skipRemoteRefs,omitempty"`
	RenderSkipSecrets bool     `yaml:"renderSkipSecrets,omitempty"`
	AutoDiscover      bool     `yaml:"autoDiscover,omitempty"`
	Profiles          []string `yaml:"profiles,omitempty"`
}

func TestPrepareOptions(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/prepare_options.test.spec.yml", func(t *testing.T, c testyml.Case[prepareOptsGot]) (prepareOptsGot, error) {
		repo := testutil.Repo(t, map[string]string{"che.yml": c.Input.Args.String(t, 0)})
		_, baseEnv := prepEnv(t)
		if configYml := c.Input.Args.String(t, 1); configYml != "" {
			// [why] XDG_CONFIG_HOME steers UserConfigPath, which resolves the user config layer
			cfgHome := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", cfgHome)
			t.Setenv("CHE_CONFIG_HOME", "")
			cfgDir := filepath.Join(cfgHome, "che")
			require.NoError(t, os.MkdirAll(cfgDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "config.yml"), []byte(configYml), 0o644))
		}
		var extraEnv map[string]string
		c.Input.Args.To(t, 2, &extraEnv)
		env := map[string]string{}
		maps.Copy(env, baseEnv)
		maps.Copy(env, extraEnv)
		var flags struct {
			ValidateSpec string   `yaml:"validateSpec"`
			Profiles     []string `yaml:"profiles"`
		}
		c.Input.Args.To(t, 3, &flags)
		cliOpts := options.Options{ValidateSpec: options.ValidateSpecMode(flags.ValidateSpec), Profiles: flags.Profiles}
		_, opts, err := PrepareApplicationOptions(newContext(env, repo), cliOpts)
		if err != nil {
			return prepareOptsGot{}, err
		}
		return prepareOptsGot{
			ValidateSpec:      string(opts.ValidateSpec),
			LogLevel:          opts.LogLevel,
			DryRun:            string(opts.DryRun),
			SkipRemoteRefs:    opts.SkipRemoteRefs,
			RenderSkipSecrets: opts.RenderSkipSecrets,
			AutoDiscover:      opts.AutoDiscover,
			Profiles:          opts.Profiles,
		}, nil
	})
}

func TestOverlayEnv(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/overlay_env.test.spec.yml", func(t *testing.T, c testyml.Case[map[string]string]) (map[string]string, error) {
		var base, overlay map[string]string
		c.Input.Args.To(t, 0, &base)
		c.Input.Args.To(t, 1, &overlay)
		snapshot := maps.Clone(base)
		got := overlayEnv(base, overlay)
		assert.Equal(t, snapshot, base, "base map unchanged")
		return got, nil
	})
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

type stubOpSpec struct {
	Name     string `yaml:"name"`
	Selected bool   `yaml:"selected"`
	Delta    int    `yaml:"delta"`
	Fail     string `yaml:"fail"`
}

type stubProfileSpec struct {
	Ref     string       `yaml:"ref"`
	SkipOps []string     `yaml:"skipOps"`
	Ops     []stubOpSpec `yaml:"ops"`
}

type execGot struct {
	Ran      []string `yaml:"ran,omitempty"`
	Executed []string `yaml:"executed,omitempty"`
}

func TestExecOperations(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/exec_operations.test.spec.yml", func(t *testing.T, c testyml.Case[execGot]) {
		var specs []stubProfileSpec
		c.Input.Args.To(t, 0, &specs)
		var ran []string
		profiles := make([]*ProfileReady, 0, len(specs))
		for _, ps := range specs {
			p := &ProfileReady{ref: ps.Ref, opts: options.Options{RunSkipOps: ps.SkipOps}}
			for _, op := range ps.Ops {
				var fail error
				if op.Fail != "" {
					fail = errors.New(op.Fail)
				}
				p.OperationsReady = append(p.OperationsReady,
					&stubOperation{name: op.Name, selected: op.Selected, delta: op.Delta, fail: fail, ran: &ran})
			}
			profiles = append(profiles, p)
		}
		t.Cleanup(log.SwapLevel(log.Levels.Debug))
		var executed []string
		var err error
		out, capErr := testutil.CaptureStdout(t, func() error {
			if c.Input.Args.Bool(t, 1) {
				s := &SpecReady{Profiles: profiles}
				err = s.ExecEach(context.Background(), "run", func(_ context.Context, p *ProfileReady) error {
					executed = append(executed, p.Ref())
					return nil
				})
				return nil
			}
			err = profiles[0].ExecOperations(context.Background())
			return nil
		})
		require.NoError(t, capErr)
		c.Expected.Check(t, err)
		assert.Equal(t, c.Expected.Output.Ran, ran, "op run order")
		assert.Equal(t, c.Expected.Output.Executed, executed, "executed profiles")
		stripped := testutil.StripANSI(out)
		for _, m := range c.Expected.StdOut {
			testyml.MustMatch(t, stripped, m)
		}
		for _, m := range c.NotExpected.StdOut {
			testyml.MustNotMatch(t, stripped, m)
		}
	})
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
