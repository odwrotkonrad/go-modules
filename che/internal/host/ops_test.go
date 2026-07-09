package host

// [>] 🤖🤖🤖

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/config"
	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

// setupHost: mock che repo, returns Host under cfg, resolved spec, repo dir.
func setupHost(t *testing.T, cfg config.Config) (Host, spec.Resolved, string) {
	t.Helper()
	dir, home := testutil.CheRepo(t)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	h := New(dir, home, testutil.CheProfile, cfg)
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

// opsIn is one case's input: the op to run plus stub knobs (failCmds fails
// matching executor commands, notLoaded/noPid drive the launchctl print stub,
// brokenLink seeds a broken repo symlink under HOME).
type opsIn struct {
	Op         string
	BrokenLink bool     `yaml:"brokenLink"`
	FailCmds   []string `yaml:"failCmds"`
	NotLoaded  bool     `yaml:"notLoaded"`
	NoPid      bool     `yaml:"noPid"`
}

// launchdStub models launchd state across the executor calls of one case:
// bootout unloads, bootstrap loads, print reports per loaded/noPid.
func launchdStub(in opsIn) func([]string) ([]byte, error) {
	loaded := !in.NotLoaded
	return func(argv []string) ([]byte, error) {
		cmd := strings.Join(argv, " ")
		for _, f := range in.FailCmds {
			if strings.Contains(cmd, f) {
				return nil, errors.New("stub: fail " + f)
			}
		}
		switch {
		case strings.Contains(cmd, "launchctl bootout"):
			loaded = false
		case strings.Contains(cmd, "launchctl bootstrap"):
			loaded = true
		case strings.Contains(cmd, "launchctl print"):
			if !loaded {
				return nil, errors.New("stub: not loaded")
			}
			if in.NoPid {
				return []byte("state = running\n"), nil
			}
			return []byte("\tpid = 4242\n"), nil
		}
		return nil, nil
	}
}

// stubSleep no-ops the services settle/poll pacing for this test.
func stubSleep(t *testing.T) {
	t.Helper()
	prev := Sleep
	Sleep = func(time.Duration) {}
	t.Cleanup(func() { Sleep = prev })
}

// seedBrokenLink plants a symlink under HOME pointing at a missing root/ file.
func seedBrokenLink(t *testing.T, h Host) string {
	t.Helper()
	dir := filepath.Join(h.Home, ".config/zsh")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dead := filepath.Join(dir, "dead")
	if err := os.Symlink(h.Src("HOME/.config/zsh/gone"), dead); err != nil {
		t.Fatal(err)
	}
	return dead
}

// TestOps runs each op with dry-run off against a record-only MockFS and a
// mock command executor: log lines assert the behavior, nothing touches the
// host.
func TestOps(t *testing.T) {
	type c struct {
		Name    string
		In      opsIn
		Want    testyml.Want
		NotWant testyml.Want `yaml:"notWant"`
	}
	specs, err := fs.Glob(td, "testdata/spec/*.spec.yml")
	if err != nil || len(specs) == 0 {
		t.Fatalf("glob spec files: %v (%d found)", err, len(specs))
	}
	run := func(t *testing.T, c c) {
		run, ok := ops[c.In.Op]
		if !ok {
			t.Fatalf("unknown op %q", c.In.Op)
		}
		h, res, dir := setupHost(t, config.Config{})
		h = h.WithFS(&testutil.MockFS{})
		execx.Swap(t, &execx.Mock{Stub: launchdStub(c.In)})
		stubSleep(t)
		if c.In.BrokenLink {
			seedBrokenLink(t, h)
		}
		out, err := testutil.CaptureStdout(t, func() error { return run(h, res) })
		if c.Want.IsErrorWanted() {
			c.Want.CheckErr(t, err)
		} else if err != nil {
			t.Fatalf("%s errored: %v\n%s", c.In.Op, err, out)
		}
		vars := map[string]string{"HOME": h.Home, "REPO": dir, "ROOT": h.Root}
		stripped := testutil.StripStamps(testutil.StripANSI(out))
		for _, f := range c.Want.StdOut {
			testyml.MustMatch(t, stripped, testyml.Expand(f, vars))
		}
		for _, f := range c.NotWant.StdOut {
			testyml.MustNotMatch(t, stripped, testyml.Expand(f, vars))
		}
	}
	for _, spec := range specs {
		testyml.Run(t, td, spec, run)
	}
}

// [<] 🤖🤖🤖
