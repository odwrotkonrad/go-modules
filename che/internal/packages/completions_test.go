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
    installers:
      - binariesRemoteArchive:
          extractBinaries: [go/bin/go, go/bin/gofmt]
          platformEligibility:
            - linux-amd64: sha256:goodsha
          version: 1.26.4
          url: https://example.com/go{version}.{os}-{arch}.tar.gz
    completions:
      zsh:
        name: _golang
        url: https://example.com/_golang
        checksum: sha256:compsha
`

const (
	goArchiveURL = "https://example.com/go1.26.4.linux-amd64.tar.gz"
	goCompURL    = "https://example.com/_golang"
)

var goCompBody = []byte("#compdef go\n")

func completionsOpts(extra Options) Options {
	extra.CompletionsEnabled = true
	return extra
}

func goCompSetup(t *testing.T, opts Options) (*Installer, string) {
	t.Helper()
	archive := tarGzBody(t, "go/bin/go", "go/bin/gofmt")
	yml := strings.ReplaceAll(withSha(goCompYaml, archive), "compsha", shaHex(goCompBody))
	in, _ := newInstaller(t, yml, "linux", cmdMap(nil), opts)
	home := tempHome(t, in)
	testFetch.Bodies[goArchiveURL] = archive
	testFetch.Bodies[goCompURL] = goCompBody
	return in, home
}

func TestEntryObjectFormParsesManagersAndCompletions(t *testing.T) {
	in, _ := newInstaller(t, goCompYaml, "linux", cmdMap(nil), Options{})
	entry, err := in.File.Find("go", "packages.yml")
	require.NoError(t, err)
	require.Len(t, entry.Items, 1)
	require.Equal(t, "binariesRemoteArchive", entry.Items[0].Mgr)
	require.Equal(t, "_golang", entry.Completions.Zsh.Name)
	require.Equal(t, "https://example.com/_golang", entry.Completions.Zsh.URL)
}

func TestEntryObjectFormRequiresManagersOrCompletions(t *testing.T) {
	var f File
	err := yaml.Unmarshal([]byte("packages:\n  go: {}\n"), &f)
	require.ErrorContains(t, err, "installers or completions")
	err = yaml.Unmarshal([]byte("packages:\n  go:\n    completions:\n      zsh: {name: _go}\n"), &f)
	require.ErrorContains(t, err, "exactly one of cmd or url")
	err = yaml.Unmarshal([]byte("packages:\n  go:\n    completions:\n      zsh: {cmd: go env, url: https://example.com}\n"), &f)
	require.ErrorContains(t, err, "exactly one of cmd or url")
}

const dockerCompYaml = `packages:
  docker:
    completions:
      zsh: {cmd: docker completion zsh}
`

func TestManagerlessEntryInstallsCompletionsWhenPresent(t *testing.T) {
	in, m := newInstaller(t, dockerCompYaml, "linux", cmdMap([]string{"docker"}), completionsOpts(Options{}))
	home := tempHome(t, in)
	m.Stub = stubOutputs("sh ", "#compdef docker\n")
	require.NoError(t, in.Install([]string{"docker"}))
	requireCalls(t, m, "sh -ec docker completion zsh")
	dest := filepath.Join(home, ".local", "share", "zsh", "site-functions", "_docker")
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, "#compdef docker\n", string(got))
	fi, err := os.Stat(dest)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), fi.Mode().Perm())
}

func TestManagerlessEntrySkipsWhenCommandAbsent(t *testing.T) {
	in, m := newInstaller(t, dockerCompYaml, "linux", cmdMap(nil), completionsOpts(Options{}))
	require.NoError(t, in.Install([]string{"docker"}))
	require.Empty(t, m.Calls())
}

func TestInstallCompletionsCmdFailureAborts(t *testing.T) {
	in, m := newInstaller(t, dockerCompYaml, "linux", cmdMap([]string{"docker"}), completionsOpts(Options{}))
	m.Stub = failOn("sh -ec")
	err := in.Install([]string{"docker"})
	require.ErrorContains(t, err, "completions cmd failed")
}

func TestInstallCompletionsDisabledByDefault(t *testing.T) {
	in, _ := goCompSetup(t, Options{})
	require.NoError(t, in.Install([]string{"go"}))
	require.NotContains(t, testFetch.Calls(), goCompURL)
}

func TestInstallCompletionsFetchesVerifiesInstalls(t *testing.T) {
	in, home := goCompSetup(t, completionsOpts(Options{}))
	require.NoError(t, in.Install([]string{"go"}))
	require.Contains(t, testFetch.Calls(), goCompURL)
	dest := filepath.Join(home, ".local", "share", "zsh", "site-functions", "_golang")
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, goCompBody, got)
	fi, err := os.Stat(dest)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), fi.Mode().Perm())
}

func TestInstallCompletionsSkipsWhenPresent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "_golang"), []byte(""), 0o644))
	in, _ := goCompSetup(t, completionsOpts(Options{CompletionsDestinationCandidates: []string{dir}}))
	require.NoError(t, in.Install([]string{"go"}))
	require.NotContains(t, testFetch.Calls(), goCompURL)
}

func TestInstallCompletionsDryRunAnnounces(t *testing.T) {
	in, m := newInstaller(t, goCompYaml, "linux", cmdMap(nil), completionsOpts(Options{DryRun: true}))
	out, err := captureStdout(t, func() error { return in.Install([]string{"go"}) })
	require.NoError(t, err)
	wantLines(t, out, "install go zsh completions -> /home/u/.local/share/zsh/site-functions/_golang (dry run)")
	require.Empty(t, m.Calls())
}

func TestCompletionsDirPicksFpathCandidate(t *testing.T) {
	offDir, onDir := filepath.Join(t.TempDir(), "off"), filepath.Join(t.TempDir(), "on")
	in, _ := goCompSetup(t, completionsOpts(Options{
		CompletionsDestinationCandidates: []string{offDir, onDir},
		CompletionsCheckPresentOnFpath:   true,
	}))
	in.Host.FpathDirs = func() []string { return []string{onDir} }
	require.NoError(t, in.Install([]string{"go"}))
	require.FileExists(t, filepath.Join(onDir, "_golang"))
}

func TestCompletionsDirWarnsWhenNoneOnFpath(t *testing.T) {
	offDir := filepath.Join(t.TempDir(), "off")
	in, _ := goCompSetup(t, completionsOpts(Options{
		CompletionsDestinationCandidates: []string{offDir},
		CompletionsCheckPresentOnFpath:   true,
	}))
	in.Host.FpathDirs = func() []string { return []string{"/elsewhere"} }
	out, err := captureStdout(t, func() error { return in.Install([]string{"go"}) })
	require.NoError(t, err)
	wantLines(t, out, "not on fpath no packages.completions.zsh.installDestinationCandidates entry is on fpath ("+offDir+"), using "+offDir)
}

func TestInstallCompletionsShaMismatchAborts(t *testing.T) {
	archive := tarGzBody(t, "go/bin/go", "go/bin/gofmt")
	in, _ := newInstaller(t, withSha(goCompYaml, archive), "linux", cmdMap(nil), completionsOpts(Options{}))
	home := tempHome(t, in)
	testFetch.Bodies[goArchiveURL] = archive
	testFetch.Bodies[goCompURL] = goCompBody
	err := in.Install([]string{"go"})
	require.ErrorContains(t, err, "sha256 mismatch")
	require.NoFileExists(t, filepath.Join(home, ".local", "share", "zsh", "site-functions", "_golang"))
}

// [<] 🤖🤖
