package che

// [>] 🤖🤖🤖

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// makeOpRecipes resolves one profile of dir's che.yml into its operation recipes.
func makeOpRecipes(t *testing.T, dir, profile string) spec.OperationRecipes {
	t.Helper()
	d, err := spec.Load(filepath.Join(dir, "che.yml"))
	require.NoError(t, err)
	for i := range d.ProfileRecipes {
		d.ProfileRecipes[i].Source.DirectoryPath = dir
	}
	rec, err := spec.FindRecipe(d.ProfileRecipes, profile)
	require.NoError(t, err)
	ops, _, err := rec.MakeProfile(d.ProfileRecipes, filepath.Join(dir, "root"))
	require.NoError(t, err)
	return ops
}

// newProfile builds a *ProfileReady anchored at dir/home under cfg, real seams
// (tests overlay mocks via withSeams).
func newProfile(dir, home string, cfg options.Options) *ProfileReady {
	return &ProfileReady{
		Source:     spec.ProfileSourceReady{ProfileName: testutil.CheProfile},
		ref:        testutil.CheProfile,
		workingDir: filepath.Join(dir, "root"),
		opts:       cfg,
		home:       home,
		Seams:      NewSeams(home),
	}
}

func (p *ProfileReady) withDir(dir string) *ProfileReady {
	p.Source.DirectoryPath = dir
	return p
}

// setupProfile: mock che repo, returns profile under cfg, resolved op recipes, repo dir.
func setupProfile(t *testing.T, cfg options.Options) (*ProfileReady, spec.OperationRecipes, string) {
	t.Helper()
	dir, home := testutil.CheRepo(t)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local/state"))
	p := newProfile(dir, home, cfg).withDir(dir)
	return p, makeOpRecipes(t, dir, testutil.CheProfile), dir
}

// ops maps a command word to the profile op it drives.
var ops = map[string]func(*ProfileReady, spec.OperationRecipes) error{
	"make-links": func(p *ProfileReady, r spec.OperationRecipes) error {
		return p.makeLinks(r.MakeLinks.Links, r.MakeLinks.Dirs)
	},
	"make-copies": func(p *ProfileReady, r spec.OperationRecipes) error {
		return p.makeCopies(r.MakeCopies.Copies, r.MakeCopies.Dirs)
	},
	"render-templates": func(p *ProfileReady, r spec.OperationRecipes) error {
		return p.renderTemplates(r.RenderTemplates.Templates, false)
	},
	"make-dirs":   func(p *ProfileReady, r spec.OperationRecipes) error { return p.makeDirs(r.MakeDirs.Dirs) },
	"prune-links": func(p *ProfileReady, r spec.OperationRecipes) error { return p.pruneBrokenLinks(r.PruneLinks.Dirs) },
	"run-scripts": func(p *ProfileReady, r spec.OperationRecipes) error {
		scripts, err := p.resolveScripts(r.RunScripts.Scripts)
		if err != nil {
			return err
		}
		return p.runScripts(scripts)
	},
	"services-bootout": func(p *ProfileReady, r spec.OperationRecipes) error {
		svcs, err := p.resolveServices(r.RunServices.Services)
		if err != nil {
			return err
		}
		return p.bootout(svcs)
	},
	"services-bootin": func(p *ProfileReady, r spec.OperationRecipes) error {
		svcs, err := p.resolveServices(r.RunServices.Services)
		if err != nil {
			return err
		}
		return p.bootin(svcs)
	},
	"services-ensure": func(p *ProfileReady, r spec.OperationRecipes) error {
		svcs, err := p.resolveServices(r.RunServices.Services)
		if err != nil {
			return err
		}
		return p.ensure(svcs)
	},
}

// applyScenario feeds the case's named scenario args to the generic mocks
// (executor knobs) or the fixture tree (brokenLink).
func applyScenario(t *testing.T, a testyml.Args, m *testutil.MockSet, p *ProfileReady) {
	t.Helper()
	for i := range a {
		switch name := a.Name(i); name {
		case "failCmds":
			a.To(t, i, &m.Exec.FailCmds)
		case "notLoaded":
			a.To(t, i, &m.Exec.NotLoaded)
		case "noPid":
			a.To(t, i, &m.Exec.NoPid)
		case "stubbornPrints":
			a.To(t, i, &m.Exec.StubbornPrints)
		case "brokenLink":
			if a.Bool(t, i) {
				seedBrokenLink(t, p)
			}
		default:
			t.Fatalf("unknown scenario arg %q", name)
		}
	}
}

// seedBrokenLink plants a symlink under HOME pointing at a missing root/ file.
func seedBrokenLink(t *testing.T, p *ProfileReady) string {
	t.Helper()
	dir := filepath.Join(p.home, ".config/zsh")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	dead := filepath.Join(dir, "dead")
	require.NoError(t, os.Symlink(p.src("HOME/.config/zsh/gone"), dead))
	return dead
}

// TestOps: dry-run off, record-only writer + mock executor, log lines assert
// the behavior.
func TestOps(t *testing.T) {
	specs, err := fs.Glob(td, "testdata/spec/cmds/che-*.test.spec.yml")
	require.NoError(t, err)
	require.NotEmpty(t, specs)
	run := func(t *testing.T, c testyml.Case[struct{}]) {
		op, ok := ops[strings.Join(c.Context.CommandArgs(), "-")]
		require.Truef(t, ok, "unknown command %q", c.Context.Command)
		p, res, dir := setupProfile(t, options.Options{})
		m := testutil.ApplyMocks(t, c.Context.MockedInterfaces)
		m.Reader.Roots = []string{dir}
		p.FS, p.Reader = m.FS, m.Reader
		testyml.Swap(t, &fsutil.Sleep, testutil.SleepMock)
		applyScenario(t, c.Input.Args, m, p)
		out, runErr := testutil.CaptureStdout(t, func() error { return op(p, res) })
		c.Expected.Check(t, runErr)
		vars := map[string]string{"HOME": p.home, "REPO": dir, "ROOT": p.root()}
		stripped := testutil.StripStamps(testutil.StripANSI(out))
		for _, f := range c.Expected.StdOut {
			testyml.MustMatch(t, stripped, testyml.Expand(f, vars))
		}
		for _, f := range c.NotExpected.StdOut {
			testyml.MustNotMatch(t, stripped, testyml.Expand(f, vars))
		}
	}
	for _, spec := range specs {
		testyml.Run(t, td, spec, run)
	}
}

// [<] 🤖🤖🤖
