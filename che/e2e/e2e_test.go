package e2e

// [>] 🤖🤖

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestE2EDryRun(t *testing.T) { runCase(t, "dry-run-e2e") }

func TestE2ERun(t *testing.T) { runCase(t, "e2e") }

type world struct {
	bin                       string
	work, home, local, remote string
	vars                      map[string]string
	env                       []string
}

// binPath resolves the shell-built binary from E2E_BIN (set by the e2e-*
// make targets, which build it with -cover); unset skips the test.
func binPath(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("E2E_BIN")
	if bin == "" {
		t.Skip("E2E_BIN not set; run via make e2e-dryrun / e2e-run")
	}
	abs, err := filepath.Abs(bin)
	require.NoError(t, err)
	_, err = os.Stat(abs)
	require.NoErrorf(t, err, "E2E_BIN %s", bin)
	return abs
}

func runCase(t *testing.T, name string) {
	bin := binPath(t)
	c := loadCase(t, "e2e.spec.yml", name)
	w := setup(t, c.Context.Env)
	w.bin = bin
	for _, s := range c.Steps {
		printHeader(s.Name)
		out, code := w.act(t, s)
		if s.Command != "" {
			assert.Equalf(t, s.Expected.ExitCode, code, "step %s: exit code, output:\n%s", s.Name, out)
		}
		w.assertExpected(t, s.Name, out, s.Expected)
		w.assertNotExpected(t, s.Name, out, s.NotExpected)
		if t.Failed() {
			t.Fatalf("step %s failed (workdir kept: %s)", s.Name, w.work)
		}
	}
	os.RemoveAll(w.work)
}

func loadCase(t *testing.T, path, name string) testCase {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoErrorf(t, err, "read spec %s", path)
	var f specFile
	require.NoErrorf(t, testyml.StrictDecode(raw, &f), "decode spec %s", path)
	require.NotEmptyf(t, f.TestCases, "%s: no testCases", path)
	for _, c := range f.TestCases {
		require.NotEmptyf(t, c.Name, "%s: testCase missing name", path)
		require.NotEmptyf(t, c.Steps, "%s: testCase %q: no steps", path, c.Name)
		for i, s := range c.Steps {
			require.NotEmptyf(t, s.Name, "%s: %s: step %d: missing name", path, c.Name, i)
			require.Equalf(t, 1, s.countActions(), "%s: %s: step %q: exactly one action", path, c.Name, s.Name)
			for _, e := range s.Expected.Files {
				require.Truef(t, e.CountGlob != "" || e.Exists, "%s: %s: step %q: files entries are positive (exists: true)", path, c.Name, s.Name)
			}
			for _, e := range s.NotExpected.Files {
				require.Truef(t, e.Exists, "%s: %s: step %q: notExpected files entries assert absence of exists: true", path, c.Name, s.Name)
			}
		}
	}
	for _, c := range f.TestCases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("%s: no testCase named %q", path, name)
	return testCase{}
}

func setup(t *testing.T, specEnv map[string]string) *world {
	t.Helper()
	work, err := os.MkdirTemp("", "che-e2e-")
	require.NoError(t, err)
	work, err = filepath.EvalSymlinks(work)
	require.NoError(t, err)
	w := &world{
		work:   work,
		home:   filepath.Join(work, "home"),
		local:  filepath.Join(work, "local"),
		remote: filepath.Join(work, "remote"),
	}
	require.NoError(t, os.CopyFS(w.local, os.DirFS("local")))
	require.NoError(t, os.CopyFS(w.remote, os.DirFS("remote")))
	cheYml := filepath.Join(w.local, "che.yml")
	raw, err := os.ReadFile(cheYml)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cheYml, bytes.ReplaceAll(raw, []byte("__REMOTE_DIR__"), []byte(w.remote)), 0o644))
	require.NoError(t, os.MkdirAll(w.home, 0o755))
	w.vars = map[string]string{
		"WORK":           w.work,
		"HOME":           w.home,
		"LOCAL":          w.local,
		"REMOTE":         w.remote,
		"XDG_STATE_HOME": filepath.Join(w.home, ".local/state"),
		"XDG_CACHE_HOME": filepath.Join(w.home, ".cache"),
	}
	w.env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + w.home,
		"XDG_STATE_HOME=" + w.vars["XDG_STATE_HOME"],
		"XDG_CACHE_HOME=" + w.vars["XDG_CACHE_HOME"],
		"XDG_CONFIG_HOME=" + filepath.Join(w.home, ".config"),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"CHE_E2E=1",
		"CHE_VALIDATE_SPEC=error",
	}
	for k, v := range specEnv {
		w.env = append(w.env, k+"="+v)
	}
	if dir := os.Getenv("E2E_GOCOVERDIR"); dir != "" {
		w.env = append(w.env, "GOCOVERDIR="+dir)
	}
	testutil.GitRepo(t, w.local)
	testutil.GitRepo(t, w.remote)
	return w
}

func (w *world) expand(s string) string { return testyml.Expand(s, w.vars) }

func (w *world) act(t *testing.T, s step) (out string, exitCode int) {
	t.Helper()
	switch {
	case s.Command != "":
		return w.runCommand(t, s)
	case s.Write != nil:
		path := w.expand(s.Write.Path)
		require.NoErrorf(t, os.WriteFile(path, []byte(s.Write.Content+"\n"), 0o644), "step %s: write %s", s.Name, path)
	case s.Remove != nil:
		for _, p := range s.Remove.Paths {
			require.NoErrorf(t, os.RemoveAll(w.expand(p)), "step %s: remove %s", s.Name, p)
		}
	case s.GitRestore != nil:
		cmd := exec.Command("git", "-C", w.expand(s.GitRestore.Repo), "checkout", "-q", "--", s.GitRestore.Path)
		cmd.Env = w.env
		gitOut, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "step %s: git restore %s: %s", s.Name, s.GitRestore.Path, gitOut)
	case s.Extract != nil:
		w.extract(t, s)
	}
	return "", 0
}

func (w *world) runCommand(t *testing.T, s step) (string, int) {
	t.Helper()
	fields := strings.Fields(w.expand(s.Command))
	require.Equalf(t, "che", fields[0], "step %s: command must invoke che", s.Name)
	fmt.Printf("$ che %s\n", strings.Join(fields[1:], " "))
	cmd := exec.Command(w.bin, append([]string{"-C", w.local}, fields[1:]...)...)
	cmd.Env = w.env
	raw, err := cmd.CombinedOutput()
	out := string(raw)
	fmt.Print(out)
	if out != "" && !strings.HasSuffix(out, "\n") {
		fmt.Println()
	}
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("step %s: run che: %v", s.Name, err)
		}
		code = exitErr.ExitCode()
	}
	return testutil.StripANSI(out), code
}

func (w *world) assertExpected(t *testing.T, name, out string, want want) {
	t.Helper()
	if want.StdOutFull != "" {
		full := w.expand(want.StdOutFull)
		if !testyml.IsMatchFull(out, full) {
			t.Errorf("step %s: full output mismatch\n--- want ---\n%s--- got ---\n%s", name, full, out)
		}
	}
	for _, m := range want.StdOut {
		testyml.MustMatch(t, out, w.expand(m))
	}
	for _, f := range want.Files {
		w.assertFile(t, name, f)
	}
}

func (w *world) assertNotExpected(t *testing.T, name, out string, want notWant) {
	t.Helper()
	for _, m := range want.StdOut {
		testyml.MustNotMatch(t, out, w.expand(m))
	}
	for _, f := range want.Files {
		path := w.expand(f.Path)
		if _, err := os.Lstat(path); err == nil {
			t.Errorf("step %s: present but must be absent: %s", name, path)
		}
	}
}

func (w *world) assertFile(t *testing.T, name string, f fileAssert) {
	t.Helper()
	if f.CountGlob != "" {
		matches, err := filepath.Glob(w.expand(f.CountGlob))
		require.NoErrorf(t, err, "step %s: glob %s", name, f.CountGlob)
		assert.Lenf(t, matches, f.Count, "step %s: glob %s", name, f.CountGlob)
		return
	}
	path := w.expand(f.Path)
	info, err := os.Lstat(path)
	if err != nil {
		t.Errorf("step %s: missing %s", name, path)
		return
	}
	switch f.Type {
	case "file":
		if !info.Mode().IsRegular() {
			t.Errorf("step %s: not a regular file: %s (%v)", name, path, info.Mode())
		}
	case "dir":
		if !info.IsDir() {
			t.Errorf("step %s: not a dir: %s (%v)", name, path, info.Mode())
		}
	case "symlink":
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("step %s: not a symlink: %s (%v)", name, path, info.Mode())
		}
	case "":
	default:
		t.Fatalf("step %s: unknown file type %q", name, f.Type)
	}
	if f.SymlinkTo != "" {
		target, err := os.Readlink(path)
		require.NoErrorf(t, err, "step %s: readlink %s", name, path)
		if want := w.expand(f.SymlinkTo); !testyml.IsMatch(target, want) {
			t.Errorf("step %s: symlink %s -> %s, want %s", name, path, target, want)
		}
	}
	if f.Content != "" {
		raw, err := os.ReadFile(path)
		require.NoErrorf(t, err, "step %s: read %s", name, path)
		assert.Equalf(t, f.Content, strings.TrimRight(string(raw), "\n"), "step %s: content of %s", name, path)
	}
}

func (w *world) extract(t *testing.T, s step) {
	t.Helper()
	matches, err := filepath.Glob(w.expand(s.Extract.ArchiveGlob))
	require.NoErrorf(t, err, "step %s: glob %s", s.Name, s.Extract.ArchiveGlob)
	require.Lenf(t, matches, 1, "step %s: archives matching %s", s.Name, s.Extract.ArchiveGlob)
	dest := w.expand(s.Extract.Dest)
	require.NoError(t, os.MkdirAll(dest, 0o755))
	archive, err := os.Open(matches[0])
	require.NoError(t, err)
	defer archive.Close()
	reader := tar.NewReader(bzip2.NewReader(archive))
	for {
		hdr, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoErrorf(t, err, "step %s: read %s", s.Name, matches[0])
		// [why] che backups (fsutil.ArchiveDestinations) emit regular files only.
		require.Equalf(t, byte(tar.TypeReg), hdr.Typeflag, "step %s: tar entry %q", s.Name, hdr.Name)
		target := filepath.Join(dest, hdr.Name)
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
		out, err := os.Create(target)
		require.NoError(t, err)
		_, err = io.Copy(out, reader)
		require.NoError(t, err)
		require.NoError(t, out.Close())
	}
}

var headerRule = strings.Repeat("━", 64)

func printHeader(name string) {
	fmt.Printf("\n\x1b[1;36m%s\n  %s\n%s\x1b[0m\n", headerRule, name, headerRule)
}

// [<] 🤖🤖
