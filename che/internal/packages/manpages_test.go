package packages

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

type manpageGot struct {
	Base    string `yaml:"base"`
	Section string `yaml:"section"`
}

func TestParseManpage(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/parse_manpage.test.spec.yml", func(t *testing.T, c testyml.Case[manpageGot]) (manpageGot, error) {
		base, section, err := ParseManpage(c.Input.Args.String(t, 0))
		if err != nil {
			return manpageGot{}, err
		}
		return manpageGot{Base: base, Section: section}, nil
	})
}

const fooManYaml = `packages:
  foo:
    - binariesRemoteArchive:
        extractBinaries: [foo]
        extractManpages: [man/foo.1]
        platformEligibility:
          - linux-amd64: sha256:goodsha
        version: "1.0"
        url: https://example.com/foo-{version}.tar.gz
`

func TestInstallBinaryTarManpages(t *testing.T) {
	body := tarGzBody(t, "foo", "man/foo.1")
	in, _ := newInstaller(t, withSha(fooManYaml, body), "linux", cmdMap(nil), Options{})
	home := tempHome(t, in)
	testFetch.Bodies["https://example.com/foo-1.0.tar.gz"] = body
	require.NoError(t, in.Install([]string{"foo"}))
	requireSymlink(t, filepath.Join(home, ".local", "share", "man", "man1", "foo.1"),
		filepath.Join(home, ".local", "opt", "foo", "man", "foo.1"))
}

func TestInstallBinaryTarManpageMemberMissing(t *testing.T) {
	body := tarGzBody(t, "foo")
	in, _ := newInstaller(t, withSha(fooManYaml, body), "linux", cmdMap(nil), Options{})
	tempHome(t, in)
	testFetch.Bodies["https://example.com/foo-1.0.tar.gz"] = body
	require.ErrorContains(t, in.Install([]string{"foo"}), "manpage member not in archive: man/foo.1")
}

func writeManpage(t *testing.T, dir, section, name string) string {
	t.Helper()
	p := filepath.Join(dir, "man"+section, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(""), 0o644))
	return p
}

func TestCheckManpagesWarnsMissing(t *testing.T) {
	in, _ := newInstaller(t, "packages:\n  kind:\n    manpages: [kind.1]\n    installers: [brew]", "darwin", cmdMap(nil), Options{})
	in.Host.ManpathDirs = func() []string { return []string{t.TempDir()} }
	out, err := captureStdout(t, func() error { return in.CheckManpages([]string{"kind"}) })
	require.NoError(t, err)
	wantLines(t, out, "kind: kind.1 resolves nowhere on the man search path")
}

func TestCheckManpagesWarnsDuplicates(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	pa := writeManpage(t, dirA, "1", "kind.1")
	pb := writeManpage(t, dirB, "1", "kind.1.gz")
	in, _ := newInstaller(t, "packages:\n  kind:\n    manpages: [kind.1]\n    installers: [brew]", "darwin", cmdMap(nil), Options{})
	in.Host.ManpathDirs = func() []string { return []string{dirA, dirB} }
	out, err := captureStdout(t, func() error { return in.CheckManpages([]string{"kind"}) })
	require.NoError(t, err)
	wantLines(t, out, "kind: manpage kind.1: "+pa+", "+pb)
}

func TestCheckManpagesItemOverride(t *testing.T) {
	in, _ := newInstaller(t, "packages:\n  kind:\n    manpages: [kind.1]\n    installers:\n      - brew:\n          manpages: []", "darwin", cmdMap(nil), Options{})
	in.Host.ManpathDirs = func() []string { return []string{t.TempDir()} }
	out, err := captureStdout(t, func() error { return in.CheckManpages([]string{"kind"}) })
	require.NoError(t, err)
	require.Empty(t, out)
}

// [<] 🤖🤖
