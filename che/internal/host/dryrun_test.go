package host

// [>] 🤖🤖

import (
	"embed"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

// setupHost: mock che repo, returns dry-run Host, resolved spec, repo dir.
func setupHost(t *testing.T) (Host, spec.Resolved, string) {
	t.Helper()
	dir, home := testutil.CheRepo(t)
	h := New(dir, home, testutil.CheProfile, DryRunDelta)
	s, err := spec.Load(filepath.Join(dir, "che.yml"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Resolve([]string{testutil.CheProfile}, h.Root)
	if err != nil {
		t.Fatal(err)
	}
	return h, res, dir
}

var ops = map[string]func(Host, spec.Resolved) error{
	"link":             func(h Host, r spec.Resolved) error { return h.MkLinks(r.Links, r.Dirs) },
	"copy":             func(h Host, r spec.Resolved) error { return h.MkCopies(r.Copies, r.Dirs) },
	"render-templates": func(h Host, r spec.Resolved) error { return h.RenderTemplates(r.Templates, false) },
	"mk-dirs":          func(h Host, r spec.Resolved) error { return h.MkDirs(r.Dirs, r.ExtraDirs) },
	"prune-links":      func(h Host, r spec.Resolved) error { return h.PruneBrokenLinks(r.Dirs) },
	"run-scripts": func(h Host, r spec.Resolved) error {
		scripts, err := h.ResolveScripts(r.Scripts)
		if err != nil {
			return err
		}
		return h.RunScripts(scripts)
	},
	"services-bootout": func(h Host, r spec.Resolved) error {
		svcs, err := h.ResolveServices(r.Services)
		if err != nil {
			return err
		}
		return h.Bootout(svcs)
	},
	"services-bootin": func(h Host, r spec.Resolved) error {
		svcs, err := h.ResolveServices(r.Services)
		if err != nil {
			return err
		}
		return h.Bootin(svcs)
	},
	"services-ensure": func(h Host, r spec.Resolved) error {
		svcs, err := h.ResolveServices(r.Services)
		if err != nil {
			return err
		}
		return h.Ensure(svcs)
	},
}

func TestOpsDryRun(t *testing.T) {
	type in struct {
		Op string
	}
	type c struct {
		Name    string
		In      in
		Want    testyml.Want
		NotWant testyml.Want `yaml:"notWant"`
	}
	testyml.Run(t, td, "testdata/spec/ops.spec.yml", func(t *testing.T, c c) {
		run, ok := ops[c.In.Op]
		if !ok {
			t.Fatalf("unknown op %q", c.In.Op)
		}
		h, res, _ := setupHost(t)
		out, err := testutil.CaptureStdout(t, func() error { return run(h, res) })
		if c.Want.WantsError() {
			c.Want.CheckErr(t, err)
			return
		}
		if err != nil {
			t.Fatalf("%s dry-run errored: %v", c.In.Op, err)
		}
		stripped := testutil.StripANSI(out)
		for _, f := range c.Want.StdOut {
			testyml.MustMatch(t, stripped, f)
		}
		for _, f := range c.NotWant.StdOut {
			testyml.MustNotMatch(t, stripped, f)
		}
	})
}

// snapshotTree: sorted path + content under dir. [why] prove dry-run mutates nothing.
func snapshotTree(t *testing.T, dir string) string {
	t.Helper()
	var lines []string
	err := filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		switch {
		case fi.IsDir():
			lines = append(lines, "d "+rel)
		case fi.Mode()&os.ModeSymlink != 0:
			target, _ := os.Readlink(p)
			lines = append(lines, "l "+rel+" -> "+target)
		default:
			b, _ := os.ReadFile(p)
			lines = append(lines, "f "+rel+" = "+string(b))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

// dry-run=all reports every dest even when already in desired state, where
// dry-run=delta skips it.
func TestDryRunAllReportsSettledDests(t *testing.T) {
	cases := []struct {
		name string
		// settle puts a dest into desired state (link points into repo / copy matches).
		settle func(t *testing.T, h Host, r spec.Resolved) string
		run    func(Host, spec.Resolved) error
	}{
		{
			"link",
			func(t *testing.T, h Host, r spec.Resolved) string {
				item := r.Links[0]
				dest := h.ToDest(item.Rel)
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(h.Src(item.Rel), dest); err != nil {
					t.Fatal(err)
				}
				return dest
			},
			func(h Host, r spec.Resolved) error { return h.MkLinks(r.Links, r.Dirs) },
		},
		{
			"copy",
			func(t *testing.T, h Host, r spec.Resolved) string {
				item := r.Copies[0]
				dest := h.copyDests(item)[0]
				src, err := os.ReadFile(h.Src(item.Rel))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dest, src, 0o644); err != nil {
					t.Fatal(err)
				}
				return dest
			},
			func(h Host, r spec.Resolved) error { return h.MkCopies(r.Copies, r.Dirs) },
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, home := testutil.CheRepo(t)
			s, err := spec.Load(filepath.Join(dir, "che.yml"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := s.Resolve([]string{testutil.CheProfile}, filepath.Join(dir, "root"))
			if err != nil {
				t.Fatal(err)
			}

			dest := c.settle(t, New(dir, home, testutil.CheProfile, DryRunDelta), res)

			delta := New(dir, home, testutil.CheProfile, DryRunDelta)
			deltaOut, err := testutil.CaptureStdout(t, func() error { return c.run(delta, res) })
			if err != nil {
				t.Fatal(err)
			}
			testutil.NotLine(t, deltaOut, dest)

			all := New(dir, home, testutil.CheProfile, DryRunAll)
			allOut, err := testutil.CaptureStdout(t, func() error { return c.run(all, res) })
			if err != nil {
				t.Fatal(err)
			}
			testutil.WantLines(t, allOut, dest)
		})
	}
}

// each op dry-run: prints only dry-run lines, mutates nothing.
func TestDryRunMutatesNothing(t *testing.T) {
	for name, run := range ops {
		t.Run(name, func(t *testing.T) {
			h, res, dir := setupHost(t)
			before := snapshotTree(t, dir)

			out, err := testutil.CaptureStdout(t, func() error { return run(h, res) })
			if err != nil {
				t.Fatalf("%s dry-run errored: %v", name, err)
			}
			out = testutil.StripANSI(out)
			for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
				if line != "" && !strings.Contains(line, "dry-run=delta") {
					t.Errorf("%s printed a non-dry-run line: %q", name, line)
				}
			}
			if after := snapshotTree(t, dir); after != before {
				t.Errorf("%s dry-run mutated the tree:\nBEFORE:\n%s\nAFTER:\n%s", name, before, after)
			}
		})
	}
}

// [<] 🤖🤖
