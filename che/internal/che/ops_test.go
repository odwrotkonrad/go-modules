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

func makeOpRecipes(t *testing.T, dir string) spec.OperationRecipes {
	t.Helper()
	d, err := spec.Load(filepath.Join(dir, "che.yml"))
	require.NoError(t, err)
	for i := range d.ProfileRecipes {
		d.ProfileRecipes[i].Source.DirectoryPath = dir
	}
	rec, err := spec.FindRecipe(d.ProfileRecipes, testutil.CheProfile)
	require.NoError(t, err)
	ops, _, err := rec.MakeProfile(d.ProfileRecipes, filepath.Join(dir, "root"))
	require.NoError(t, err)
	return ops
}

const testRunID = "00000000T000000"

func newProfile(dir, home string, cfg options.Options) *ProfileReady {
	deps := NewDeps(home)
	specRow, _ := deps.Ledger.StartSpec(testRunID, "", "test")
	profRow, _ := deps.Ledger.StartProfile(specRow, testutil.CheProfile, testutil.CheProfile, "", dir)
	return &ProfileReady{
		Source:      spec.ProfileSourceReady{ProfileName: testutil.CheProfile},
		ref:         testutil.CheProfile,
		env:         map[string]string{"invokingSpecGitRoot": dir},
		workingDir:  filepath.Join(dir, "root"),
		opts:        cfg,
		home:        home,
		runID:       testRunID,
		runTs:       testRunID,
		specDone:    specRow,
		profileDone: profRow,
		Deps:        deps,
	}
}

func (p *ProfileReady) withDir(dir string) *ProfileReady {
	p.Source.DirectoryPath = dir
	return p
}

func setupProfile(t *testing.T, cfg options.Options) (*ProfileReady, spec.OperationRecipes, string) {
	t.Helper()
	dir, home := testutil.CheRepo(t)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local/state"))
	p := newProfile(dir, home, cfg).withDir(dir)
	return p, makeOpRecipes(t, dir), dir
}

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
	"make-dirs":          func(p *ProfileReady, r spec.OperationRecipes) error { return p.makeDirs(r.MakeDirs.Dirs) },
	"prune-broken-links": func(p *ProfileReady, _ spec.OperationRecipes) error { return p.pruneBrokenLinks() },
	"backup-create": func(p *ProfileReady, r spec.OperationRecipes) error {
		ops, err := p.prepareOperations(r)
		if err != nil {
			return err
		}
		p.OperationsReady = ops
		return p.ExecBackup()
	},
	"run-scripts": func(p *ProfileReady, r spec.OperationRecipes) error {
		scripts, err := p.resolveScripts(r.RunScripts.Scripts)
		if err != nil {
			return err
		}
		return p.runScripts(scripts)
	},
}

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
		case "settleLink":
			if a.Bool(t, i) {
				res := makeOpRecipes(t, p.Source.DirectoryPath)
				settlers["make-links"](t, p, res)
			}
		case "driftLink":
			if a.Bool(t, i) {
				res := makeOpRecipes(t, p.Source.DirectoryPath)
				item := res.MakeLinks.Links[0]
				dest := p.toDest(spec.DestRel(item))
				require.NoError(t, os.MkdirAll(filepath.Dir(dest), 0o755))
				require.NoError(t, os.WriteFile(dest, []byte("drifted\n"), 0o644))
			}
		default:
			t.Fatalf("unknown scenario arg %q", name)
		}
	}
}

func seedBrokenLink(t *testing.T, p *ProfileReady) {
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
}

func TestOps(t *testing.T) {
	specs, err := fs.Glob(td, "testdata/spec/cmds/che-*.test.spec.yml")
	require.NoError(t, err)
	require.NotEmpty(t, specs)
	run := func(t *testing.T, c testyml.Case[struct{}]) {
		op, ok := ops[strings.Join(c.Context.CommandArgs(), "-")]
		require.Truef(t, ok, "unknown command %q", c.Context.Command)
		p, res, dir := setupProfile(t, options.Options{})
		m := testutil.ApplyMocks(t, c.Context.MockedInterfaces)
		m.Reader.Roots = []string{dir, p.home}
		p.FS, p.Reader = m.FS, m.Reader
		applyScenario(t, c.Input.Args, m, p)
		out, runErr := testutil.CaptureStdout(t, func() error { return op(p, res) })
		c.Expected.Check(t, runErr)
		vars := map[string]string{"HOME": p.home, "REPO": dir, "ROOT": p.resolveRoot()}
		stripped := testutil.StripANSI(out)
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
