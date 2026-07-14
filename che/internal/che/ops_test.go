package che

// [>] 🤖🤖🤖

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/database"
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

// testRunID is the fixed per-run stamp tests use (backup filenames + ledger run).
const testRunID = "00000000T000000"

// newProfile builds a *ProfileReady anchored at dir/home under cfg, real seams
// with a fresh on-disk ledger under home's state dir, so tests exercise the real
// recording/prune/uninstall path against a throwaway DB. A started ledger
// spec/profile row lets recording ops write.
func newProfile(dir, home string, cfg options.Options) *ProfileReady {
	seams := NewSeams(home)
	specRow, _ := seams.Ledger.StartSpec(testRunID, "", "test")
	profRow, _ := seams.Ledger.StartProfile(specRow, testutil.CheProfile, testutil.CheProfile, "", dir)
	return &ProfileReady{
		Source:      spec.ProfileSourceReady{ProfileName: testutil.CheProfile},
		ref:         testutil.CheProfile,
		workingDir:  filepath.Join(dir, "root"),
		opts:        cfg,
		home:        home,
		runID:       testRunID,
		specDone:    specRow,
		profileDone: profRow,
		Seams:       seams,
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
	"prune-links": func(p *ProfileReady, _ spec.OperationRecipes) error { return p.pruneBrokenLinks() },
	"run-scripts": func(p *ProfileReady, r spec.OperationRecipes) error {
		scripts, err := p.resolveScripts(r.RunScripts.Scripts)
		if err != nil {
			return err
		}
		return p.runScripts(scripts)
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
		case "brokenLink":
			if a.Bool(t, i) {
				seedBrokenLink(t, p)
			}
		default:
			t.Fatalf("unknown scenario arg %q", name)
		}
	}
}

// seedBrokenLink plants a symlink under HOME pointing at a missing root/ file
// and records the matching ledger link op (source gone), so ledger-driven
// prune-links classifies it as broken and removes it.
func seedBrokenLink(t *testing.T, p *ProfileReady) string {
	t.Helper()
	dir := filepath.Join(p.home, ".config/zsh")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	dead := filepath.Join(dir, "dead")
	src := p.resolveSrc("_home/.config/zsh/gone")
	require.NoError(t, os.Symlink(src, dead))
	require.NoError(t, p.Ledger.RecordOperation(p.profileDone, database.OperationDone{
		OpType: "create", Kind: "link", Dest: dead, Target: src,
		Prev: database.Object{Kind: "absent"},
		Next: database.Object{Kind: "link", Present: true, Target: src},
	}))
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
		applyScenario(t, c.Input.Args, m, p)
		out, runErr := testutil.CaptureStdout(t, func() error { return op(p, res) })
		c.Expected.Check(t, runErr)
		vars := map[string]string{"HOME": p.home, "REPO": dir, "ROOT": p.resolveRoot()}
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
