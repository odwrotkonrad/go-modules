package host

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/spec"
	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// setupHost: mock che repo, returns dry-run Host, resolved spec, repo dir.
func setupHost(t *testing.T) (Host, spec.Resolved, string) {
	t.Helper()
	dir, home := testutil.CheRepo(t)
	h := New(dir, home, testutil.CheProfile, DryRunDelta)
	s, err := spec.Load(filepath.Join(dir, "che.yml"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Resolve(testutil.CheProfile, h.Root)
	if err != nil {
		t.Fatal(err)
	}
	return h, res, dir
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
			res, err := s.Resolve(testutil.CheProfile, filepath.Join(dir, "root"))
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

// each op dry-run: prints actions, mutates nothing.
func TestDryRunPasses(t *testing.T) {
	cases := []struct {
		name    string
		run     func(Host, spec.Resolved) error
		mustLog string // a verb the dry-run output must contain
	}{
		{"link", func(h Host, r spec.Resolved) error { return h.MkLinks(r.Links, r.Dirs) }, "ln(dry-run)"},
		{"copy", func(h Host, r spec.Resolved) error { return h.MkCopies(r.Copies, r.Dirs) }, "cp(dry-run)"},
		{"render-templates", func(h Host, r spec.Resolved) error { return h.RenderTemplates(r.Templates) }, "render(dry-run)"},
		{"mk-dirs", func(h Host, r spec.Resolved) error { return h.MkDirs(r.Dirs, r.ExtraDirs) }, "mkdir(dry-run)"},
		{"prune-links", func(h Host, r spec.Resolved) error { return h.PruneBrokenLinks(r.Dirs) }, "prune-links(dry-run)"},
		{"run-scripts", func(h Host, r spec.Resolved) error {
			scripts, err := h.ResolveScripts(r.Scripts)
			if err != nil {
				return err
			}
			return h.RunScripts(scripts)
		}, "run-scripts(dry-run)"},
		{"services bootout", func(h Host, r spec.Resolved) error {
			svcs, err := h.ResolveServices(r.Services)
			if err != nil {
				return err
			}
			return h.Bootout(svcs)
		}, "bootout(dry-run)"},
		{"services bootin", func(h Host, r spec.Resolved) error {
			svcs, err := h.ResolveServices(r.Services)
			if err != nil {
				return err
			}
			return h.Bootin(svcs)
		}, "bootstrap(dry-run)"},
		{"services ensure", func(h Host, r spec.Resolved) error {
			svcs, err := h.ResolveServices(r.Services)
			if err != nil {
				return err
			}
			return h.Ensure(svcs)
		}, "ensure(dry-run)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, res, dir := setupHost(t)
			before := snapshotTree(t, dir)

			out, err := testutil.CaptureStdout(t, func() error { return c.run(h, res) })
			if err != nil {
				t.Fatalf("%s dry-run errored: %v", c.name, err)
			}
			out = testutil.StripANSI(out)
			if !strings.Contains(out, c.mustLog) {
				t.Errorf("%s dry-run printed no %q action:\n%s", c.name, c.mustLog, out)
			}
			for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
				if line != "" && !strings.Contains(line, "(dry-run)") {
					t.Errorf("%s printed a non-dry-run line: %q", c.name, line)
				}
			}
			if after := snapshotTree(t, dir); after != before {
				t.Errorf("%s dry-run mutated the tree:\nBEFORE:\n%s\nAFTER:\n%s", c.name, before, after)
			}
		})
	}
}

// [<] 🤖🤖
