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
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestE2EDryRun(t *testing.T) { runSpec(t, "dryrun.e2e.spec.yml") }

func TestE2ERun(t *testing.T) { runSpec(t, "e2e.spec.yml") }

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
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("E2E_BIN %s: %v", bin, err)
	}
	return abs
}

func runSpec(t *testing.T, path string) {
	bin := binPath(t)
	spec := loadSpec(t, path)
	w := setup(t, spec.Context.Env)
	w.bin = bin
	for _, s := range spec.Steps {
		printHeader(s.Name)
		out := w.act(t, s)
		w.assertWant(t, s.Name, out, s.Expected, true)
		w.assertWant(t, s.Name, out, s.NotExpected, false)
		if t.Failed() {
			t.Fatalf("step %s failed (workdir kept: %s)", s.Name, w.work)
		}
	}
	os.RemoveAll(w.work)
}

func loadSpec(t *testing.T, path string) specFile {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoErrorf(t, err, "read spec %s", path)
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var f specFile
	require.NoErrorf(t, dec.Decode(&f), "decode spec %s", path)
	require.NotEmptyf(t, f.Steps, "%s: no steps", path)
	for i, s := range f.Steps {
		require.NotEmptyf(t, s.Name, "%s: step %d: missing name", path, i)
		actions := 0
		for _, set := range []bool{s.Command != "", s.Write != nil, s.Remove != nil, s.GitRestore != nil, s.Extract != nil} {
			if set {
				actions++
			}
		}
		require.Equalf(t, 1, actions, "%s: step %q: exactly one action", path, s.Name)
		require.Emptyf(t, s.NotExpected.StdOutFull, "%s: step %q: stdOutFull is expected-only", path, s.Name)
		for _, f := range append(append([]fileAssert{}, s.Expected.Files...), s.NotExpected.Files...) {
			if f.CountGlob != "" {
				continue
			}
			require.Truef(t, f.Exists, "%s: step %q: files entries are positive-only (exists: true), assert absence under notExpected", path, s.Name)
		}
	}
	return f
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
	for _, repo := range []string{w.local, w.remote} {
		w.git(t, repo, "init", "-q", "-b", "main")
		w.git(t, repo, "add", "-A")
		w.git(t, repo, "-c", "user.email=e2e@invalid", "-c", "user.name=e2e", "commit", "-qm", "init")
	}
	return w
}

func (w *world) expand(s string) string { return testyml.Expand(s, w.vars) }

func (w *world) git(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = w.env
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v: %s", args, out)
}

func (w *world) act(t *testing.T, s step) string {
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
		w.git(t, w.expand(s.GitRestore.Repo), "checkout", "-q", "--", s.GitRestore.Path)
	case s.Extract != nil:
		w.extract(t, s)
	}
	return ""
}

func (w *world) runCommand(t *testing.T, s step) string {
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
	out = ansiRe.ReplaceAllString(out, "")
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("step %s: run che: %v", s.Name, err)
		}
		code = exitErr.ExitCode()
	}
	assert.Equalf(t, s.Expected.ExitCode, code, "step %s: exit code, output:\n%s", s.Name, out)
	return out
}

var (
	ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")
	holeRe = regexp.MustCompile(`\{\{/(.*?)/\}\}`)
)

func (w *world) assertWant(t *testing.T, name, out string, want want, positive bool) {
	t.Helper()
	if want.StdOutFull != "" {
		full := w.expand(want.StdOutFull)
		var b strings.Builder
		b.WriteString(`\A`)
		last := 0
		for _, loc := range holeRe.FindAllStringSubmatchIndex(full, -1) {
			b.WriteString(regexp.QuoteMeta(full[last:loc[0]]))
			b.WriteString(full[loc[2]:loc[3]])
			last = loc[1]
		}
		b.WriteString(regexp.QuoteMeta(full[last:]))
		b.WriteString(`\z`)
		if !regexp.MustCompile(b.String()).MatchString(out) {
			t.Errorf("step %s: full output mismatch\n--- want ---\n%s--- got ---\n%s", name, full, out)
		}
	}
	for _, m := range want.StdOut {
		if positive {
			testyml.MustMatch(t, out, w.expand(m))
		} else {
			testyml.MustNotMatch(t, out, w.expand(m))
		}
	}
	for _, f := range want.Files {
		w.assertFile(t, name, f, positive)
	}
}

func (w *world) assertFile(t *testing.T, name string, f fileAssert, positive bool) {
	t.Helper()
	if f.CountGlob != "" {
		matches, err := filepath.Glob(w.expand(f.CountGlob))
		require.NoErrorf(t, err, "step %s: glob %s", name, f.CountGlob)
		assert.Lenf(t, matches, f.Count, "step %s: glob %s", name, f.CountGlob)
		return
	}
	path := w.expand(f.Path)
	info, err := os.Lstat(path)
	if !positive {
		if err == nil {
			t.Errorf("step %s: present but must be absent: %s", name, path)
		}
		return
	}
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
		if !testyml.IsMatch(target, w.expand(f.SymlinkTo)) {
			t.Errorf("step %s: symlink %s -> %s, want %s", name, path, target, w.expand(f.SymlinkTo))
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
		target := filepath.Join(dest, strings.TrimPrefix(hdr.Name, "/"))
		switch hdr.Typeflag {
		case tar.TypeDir:
			require.NoError(t, os.MkdirAll(target, 0o755))
		case tar.TypeReg:
			require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
			out, err := os.Create(target)
			require.NoError(t, err)
			_, err = io.Copy(out, reader)
			require.NoError(t, err)
			require.NoError(t, out.Close())
		case tar.TypeSymlink:
			require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
			require.NoError(t, os.Symlink(hdr.Linkname, target))
		default:
			t.Fatalf("step %s: unsupported tar entry %q (type %v)", s.Name, hdr.Name, hdr.Typeflag)
		}
	}
}

func printHeader(name string) {
	rule := strings.Repeat("━", 64)
	fmt.Printf("\n\x1b[1;36m%s\n  %s\n%s\x1b[0m\n", rule, name, rule)
}

// [<] 🤖🤖
