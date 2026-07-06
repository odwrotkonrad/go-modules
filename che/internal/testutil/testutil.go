// Package testutil holds shared che test fixtures: file tree, committed git repo, stdout capture.
package testutil

// [>] 🤖🤖

import (
	"embed"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ansiRe matches SGR escape sequences (bold/reset) so assertions stay style-agnostic.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI removes SGR escape sequences, leaving plain text to assert against.
func StripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// specsFS holds the checked-in che.yml fixtures.
//
//go:embed specs/*.yml
var specsFS embed.FS

// Spec returns the named che.yml fixture (testutil/specs/<name>.yml).
func Spec(t *testing.T, name string) string {
	t.Helper()
	b, err := specsFS.ReadFile("specs/" + name + ".yml")
	if err != nil {
		t.Fatalf("read spec fixture %q: %v", name, err)
	}
	return string(b)
}

// WriteTree writes each rel->content file under dir, creating parent dirs.
func WriteTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// GitRepo inits dir as a git repo and commits everything in it.
func GitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "core.excludesfile=", "add", "-Af"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-qm", "x"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// Repo returns a temp dir of files, committed as a git repo.
func Repo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	WriteTree(t, dir, files)
	GitRepo(t, dir)
	return dir
}

// CheProfile is the profile specs/che.yml resolves under.
const CheProfile = "cli/macos"

// CheRepo builds a committed mock che repo (specs/che.yml + root/ tree covering every op)
// plus an on-disk HOME. Returns (repoDir, homeDir).
func CheRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := Repo(t, map[string]string{
		"che.yml":                                          Spec(t, "che"),
		"root/etc/zshrc":                                   "zshrc\n",
		"root/HOME/.config/zsh/.zshrc":                     "user zshrc\n",
		"root/HOME/.config/zsh/c.host.cp":                  "copyme\n",
		"root/HOME/.config/zsh/t.host.tpl":                 "plain template\n",
		"templates/r.repo.tpl":                             "repo body\n",
		"root/Library/LaunchDaemons/otelcol.plist.host.cp": "<plist/>\n",
		"install/unit":                                     "#!/bin/sh\necho ran\n",
	})
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir, home
}

// MockRepoEnv builds the mock che repo, chdirs in, exports CHE_FORCE_PROFILE + HOME so
// build() resolves against it. Returns HOME (for asserting ~/ dest paths). Skips as root
// (build resolves $HOME).
func MockRepoEnv(t *testing.T) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("non-root path only; build resolves home from $HOME")
	}
	dir, home := CheRepo(t)
	t.Chdir(dir)
	t.Setenv("CHE_FORCE_PROFILE", CheProfile)
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	return home
}

// RunDry runs a subcommand's RunE (caller already built dry-run state), captures stdout,
// asserts every printed line carries the dry-run=delta scope. dryRunLines=false skips that
// check (e.g. detect, prints bare profile).
func RunDry(t *testing.T, cmd *cobra.Command, dryRunLines bool) string {
	t.Helper()
	out, err := CaptureStdout(t, func() error { return cmd.RunE(cmd, nil) })
	if err != nil {
		t.Fatalf("%s errored: %v", cmd.Name(), err)
	}
	if dryRunLines {
		for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
			if line != "" && !strings.Contains(line, "dry-run=delta") {
				t.Errorf("non-dry-run line: %q\n--- got ---\n%s", line, out)
			}
		}
	}
	return out
}

// WantLines asserts every fragment appears in out (order-independent, style-agnostic).
func WantLines(t *testing.T, out string, fragments ...string) {
	t.Helper()
	out = StripANSI(out)
	for _, f := range fragments {
		if !strings.Contains(out, f) {
			t.Errorf("output missing %q\n--- got ---\n%s", f, out)
		}
	}
}

// NotLine asserts the fragment does not appear in out (style-agnostic).
func NotLine(t *testing.T, out, fragment string) {
	t.Helper()
	out = StripANSI(out)
	if strings.Contains(out, fragment) {
		t.Errorf("output unexpectedly contains %q\n--- got ---\n%s", fragment, out)
	}
}

// CaptureStdout runs fn with os.Stdout piped. Returns printed output plus fn's error.
func CaptureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	return string(out), runErr
}

// [<] 🤖🤖
