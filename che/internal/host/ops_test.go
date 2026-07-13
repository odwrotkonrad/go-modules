package host

// [>] 🤖🤖🤖

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

// makeProfile resolves one profile of dir's che.yml into its operation recipes.
func makeProfile(t *testing.T, dir, profile string) spec.OperationRecipes {
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

// setupHost: mock che repo, returns Host under cfg, resolved op recipes, repo dir.
func setupHost(t *testing.T, cfg options.Options) (Host, spec.OperationRecipes, string) {
	t.Helper()
	dir, home := testutil.CheRepo(t)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	h := New(dir, filepath.Join(dir, "root"), home, testutil.CheProfile, cfg)
	return h, makeProfile(t, dir, testutil.CheProfile), dir
}

var ops = map[string]func(Host, spec.OperationRecipes) error{
	"make-links": func(h Host, r spec.OperationRecipes) error { return h.MakeLinks(r.MakeLinks.Links, r.MakeLinks.Dirs) },
	"make-copies": func(h Host, r spec.OperationRecipes) error {
		return h.MakeCopies(r.MakeCopies.Copies, r.MakeCopies.Dirs)
	},
	"render-templates": func(h Host, r spec.OperationRecipes) error {
		return h.RenderTemplates(r.RenderTemplates.Templates, false)
	},
	"make-dirs":   func(h Host, r spec.OperationRecipes) error { return h.MakeDirs(r.MakeDirs.Dirs) },
	"prune-links": func(h Host, r spec.OperationRecipes) error { return h.PruneBrokenLinks(r.PruneLinks.Dirs) },
	"run-scripts": func(h Host, r spec.OperationRecipes) error {
		scripts, err := h.ResolveScripts(r.RunScripts.Scripts)
		if err != nil {
			return err
		}
		return h.RunScripts(scripts)
	},
	"services-bootout": func(h Host, r spec.OperationRecipes) error {
		svcs, err := h.ResolveServices(r.RunServices.Services)
		if err != nil {
			return err
		}
		return h.Bootout(svcs)
	},
	"services-bootin": func(h Host, r spec.OperationRecipes) error {
		svcs, err := h.ResolveServices(r.RunServices.Services)
		if err != nil {
			return err
		}
		return h.Bootin(svcs)
	},
	"services-ensure": func(h Host, r spec.OperationRecipes) error {
		svcs, err := h.ResolveServices(r.RunServices.Services)
		if err != nil {
			return err
		}
		return h.Ensure(svcs)
	},
}

// applyScenario feeds the case's named scenario args to the generic mocks
// (executor knobs) or the fixture tree (brokenLink).
func applyScenario(t *testing.T, a testyml.Args, m *testutil.MockSet, h Host) {
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
				seedBrokenLink(t, h)
			}
		default:
			t.Fatalf("unknown scenario arg %q", name)
		}
	}
}

// seedBrokenLink plants a symlink under HOME pointing at a missing root/ file.
func seedBrokenLink(t *testing.T, h Host) string {
	t.Helper()
	dir := filepath.Join(h.Home, ".config/zsh")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	dead := filepath.Join(dir, "dead")
	require.NoError(t, os.Symlink(h.Src("HOME/.config/zsh/gone"), dead))
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
		h, res, dir := setupHost(t, options.Options{})
		m := testutil.ApplyMocks(t, c.Context.MockedInterfaces)
		m.Reader.Roots = []string{dir}
		h = h.WithFS(m.FS).WithFSReader(m.Reader)
		testyml.Swap(t, &Sleep, testutil.SleepMock)
		applyScenario(t, c.Input.Args, m, h)
		out, runErr := testutil.CaptureStdout(t, func() error { return op(h, res) })
		c.Expected.Check(t, runErr)
		vars := map[string]string{"HOME": h.Home, "REPO": dir, "ROOT": h.Root}
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
