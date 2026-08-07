package packages

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const goCompYaml = `packages:
  go:
    managers:
      - binary:
          version: 1.26.4
          url: https://example.com/go{version}.{os}-{arch}.tar.gz
          bin: go/bin/go go/bin/gofmt
          sha256:
            linux-amd64: goodsha
    completions:
      zsh:
        name: _golang
        url: https://example.com/_golang
        sha256: goodsha
`

func completionsOpts(extra Options) Options {
	extra.CompletionsEnabled = true
	return extra
}

func TestEntryObjectFormParsesManagersAndCompletions(t *testing.T) {
	in, _ := newInstaller(t, goCompYaml, "linux", cmdMap(nil), Options{})
	entry, err := in.File.Find("go", "packages.yml")
	require.NoError(t, err)
	require.Len(t, entry.Items, 1)
	require.Equal(t, "binary", entry.Items[0].Mgr)
	require.Equal(t, "_golang", entry.Completions.Zsh.Name)
	require.Equal(t, "https://example.com/_golang", entry.Completions.Zsh.URL)
}

func TestEntryObjectFormRequiresManagers(t *testing.T) {
	var f File
	err := yaml.Unmarshal([]byte("packages:\n  go:\n    completions:\n      zsh: {url: https://example.com/_golang}\n"), &f)
	require.ErrorContains(t, err, "managers")
}

func TestInstallCompletionsDisabledByDefault(t *testing.T) {
	in, m := newInstaller(t, goCompYaml, "linux", cmdMap([]string{"sha256sum"}), Options{})
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"go"}))
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "_golang")
}

func TestInstallCompletionsFetchesVerifiesInstalls(t *testing.T) {
	in, m := newInstaller(t, goCompYaml, "linux", cmdMap([]string{"sha256sum"}), completionsOpts(Options{}))
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"go"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "_golang https://example.com/_golang")
	require.Contains(t, calls, "mkdir -p /home/u/.local/share/zsh/site-functions")
	require.Contains(t, calls, "install -m 0644")
	require.Contains(t, calls, "/home/u/.local/share/zsh/site-functions/_golang")
}

func TestInstallCompletionsSkipsWhenPresent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "_golang"), []byte(""), 0o644))
	in, m := newInstaller(t, goCompYaml, "linux", cmdMap([]string{"sha256sum"}),
		completionsOpts(Options{CompletionsDestinationCandidates: []string{dir}}))
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"go"}))
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "_golang https://example.com/_golang")
}

func TestInstallCompletionsPackagesFilter(t *testing.T) {
	in, m := newInstaller(t, goCompYaml, "linux", cmdMap([]string{"sha256sum"}),
		completionsOpts(Options{CompletionsPackages: []string{"other"}}))
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"go"}))
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "_golang")
}

func TestInstallCompletionsDryRunAnnounces(t *testing.T) {
	in, m := newInstaller(t, goCompYaml, "linux", cmdMap(nil), completionsOpts(Options{DryRun: true}))
	out, err := captureStdout(t, func() error { return in.Install([]string{"go"}) })
	require.NoError(t, err)
	wantLines(t, out, "install go zsh completions -> /home/u/.local/share/zsh/site-functions/_golang (dry run)")
	require.Empty(t, m.Calls())
}

func TestCompletionsDirPicksFpathCandidate(t *testing.T) {
	in, m := newInstaller(t, goCompYaml, "linux", cmdMap([]string{"sha256sum"}),
		completionsOpts(Options{
			CompletionsDestinationCandidates: []string{"/off/fpath", "/on/fpath"},
			CompletionsCheckInFpath:          true,
		}))
	in.Host.FpathDirs = func() []string { return []string{"/on/fpath"} }
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"go"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "/on/fpath/_golang")
}

func TestCompletionsDirWarnsWhenNoneOnFpath(t *testing.T) {
	in, m := newInstaller(t, goCompYaml, "linux", cmdMap([]string{"sha256sum"}),
		completionsOpts(Options{
			CompletionsDestinationCandidates: []string{"/off/fpath"},
			CompletionsCheckInFpath:          true,
		}))
	in.Host.FpathDirs = func() []string { return []string{"/elsewhere"} }
	m.Stub = shaStub("goodsha")
	out, err := captureStdout(t, func() error { return in.Install([]string{"go"}) })
	require.NoError(t, err)
	wantLines(t, out, "not on fpath no packages.completions.installDestinationCandidates entry is on fpath (/off/fpath), using /off/fpath")
}

func TestInstallCompletionsShaMismatchAborts(t *testing.T) {
	in, m := newInstaller(t, goCompYaml, "linux", cmdMap([]string{"sha256sum"}), completionsOpts(Options{}))
	installed := false
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "sha256sum" && strings.HasSuffix(argv[1], "_golang") {
			return []byte("badsha  " + argv[1] + "\n"), nil
		}
		if argv[0] == "install" && argv[1] == "-m" && argv[2] == "0644" {
			installed = true
		}
		return shaStub("goodsha")(argv)
	}
	err := in.Install([]string{"go"})
	require.ErrorContains(t, err, "sha256 mismatch")
	require.False(t, installed)
}

// [<] 🤖🤖
