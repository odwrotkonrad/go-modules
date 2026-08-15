package packages

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestPinMatches(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/pin_matches.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		return PinMatches(c.Input.Args.String(t, 0), c.Input.Args.String(t, 1)), nil
	})
}

const pinnedBatYaml = `packages:
  bat:
    version: "0.25.0"
    installers: [brew, apt]
`

func TestEntryVersionSkipsWhenMatching(t *testing.T) {
	in, m := newInstaller(t, pinnedBatYaml, "darwin", cmdMap([]string{"brew", "bat"}), Options{})
	m.Stub = stubOutputs("bat ", "bat 0.25.0 (871abd2)\n")
	require.NoError(t, in.Install([]string{"bat"}))
	refuteCalls(t, m, "brew install", "brew upgrade")
}

func TestEntryVersionDriftRunsManagerUpdate(t *testing.T) {
	in, m := newInstaller(t, pinnedBatYaml, "darwin", cmdMap([]string{"brew", "bat"}), Options{})
	m.Stub = stubOutputs("bat ", "bat 0.24.0\n", "brew outdated", "bat\n")
	require.NoError(t, in.Install([]string{"bat"}))
	require.Contains(t, m.Calls(), "brew upgrade bat@0.25.0")
}

func TestEntryVersionDriftSkipsWhenManagerHasNoNewer(t *testing.T) {
	in, m := newInstaller(t, pinnedBatYaml, "darwin", cmdMap([]string{"brew", "bat"}), Options{})
	m.Stub = stubOutputs("bat ", "bat 0.24.0\n")
	require.NoError(t, in.Install([]string{"bat"}))
	refuteCalls(t, m, "brew upgrade", "brew install bat")
}

func TestEntryVersionPinsArchiveVersion(t *testing.T) {
	const y = `packages:
  kind:
    version: "0.32.0"
    installers:
      - binariesRemoteArchive:
          platformEligibility: [{linux-amd64: sha256:goodsha}]
          url: https://example.com/kind-{version}
`
	body := []byte("kind-bin")
	in, _ := newInstaller(t, withSha(y, body), "linux", cmdMap(nil), Options{})
	tempHome(t, in)
	testFetch.Bodies["https://example.com/kind-0.32.0"] = body
	require.NoError(t, in.Install([]string{"kind"}))
	require.Contains(t, testFetch.Calls(), "https://example.com/kind-0.32.0")
}

func TestArchiveWithoutVersionNeedsNoPin(t *testing.T) {
	const y = `packages:
  aws:
    - binariesRemoteArchive:
        platformEligibility: [linux-amd64]
        extractBinaries: [aws/dist/aws]
        url: https://example.com/aws-latest.zip
`
	in, _ := newInstaller(t, y, "linux", cmdMap(nil), Options{})
	tempHome(t, in)
	testFetch.Bodies["https://example.com/aws-latest.zip"] = zipBody(t, "aws/dist/aws")
	require.NoError(t, in.Install([]string{"aws"}))
	require.Contains(t, testFetch.Calls(), "https://example.com/aws-latest.zip")
}

// [<] 🤖🤖

func TestBrewPinAppendsVersionedFormula(t *testing.T) {
	const y = `packages:
  node:
    version: "24"
    installers: [brew]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"brew"}), Options{})
	m.Stub = failOn("brew list")
	require.NoError(t, in.Install([]string{"node"}))
	requireCalls(t, m, "brew install node@24")
	refuteCalls(t, m, "node@24@24")
}

func TestCaskPinWarnsAndInstallsCurrent(t *testing.T) {
	const y = `packages:
  claude:
    version: "2.1.224"
    installers: [{brew/cask: {packageName: claude-code}}]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"brew"}), Options{})
	m.Stub = failOn("brew list")
	out, err := captureStdout(t, func() error { return in.Install([]string{"claude"}) })
	require.NoError(t, err)
	require.Contains(t, out, "cannot be enforced via brew/cask")
	require.Contains(t, m.Calls(), "brew install --cask claude-code")
}

func TestUnversionedLatestSentinelSkipsPinning(t *testing.T) {
	const y = `packages:
  jq:
    installers: [brew, apt]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"brew", "jq"}), Options{})
	m.Stub = stubOutputs("jq ", "jq-1.7.1\n")
	require.NoError(t, in.Install([]string{"jq"}))
	refuteCalls(t, m, "__rolling__", "brew install", "brew upgrade")
}

func TestUnversionedLatestSentinelNotUsedAsArchiveVersion(t *testing.T) {
	const y = `packages:
  kind:
    installers:
      - binariesRemoteArchive:
          platformEligibility: [{linux-amd64: sha256:goodsha}]
          url: https://example.com/kind-{version}
`
	in, _ := newInstaller(t, y, "linux", cmdMap([]string{"sha256sum"}), Options{})
	require.ErrorContains(t, in.Install([]string{"kind"}), "no version pinned")
}

func TestExplicitLatestSentinelResolvesLatestArchiveVersion(t *testing.T) {
	const y = `packages:
  kind:
    version: latest
    installers:
      - binariesRemoteArchive:
          platformEligibility: [linux-amd64]
          url: https://example.com/kind-{version}
`
	in, _ := newInstaller(t, y, "linux", cmdMap(nil), Options{})
	tempHome(t, in)
	require.NoError(t, in.Install([]string{"kind"}))
	require.Contains(t, testFetch.Calls(), "https://example.com/kind-latest")
}

func TestAliasBinaryLinksRenamedBinary(t *testing.T) {
	const y = `packages:
  bat:
    installers:
      - brew
      - apt:
          aliasBinary:
            batcat: bat
`
	cmds := cmdMap([]string{"apt-get"})
	in, m := newInstaller(t, y, "linux", cmds, Options{})
	home := tempHome(t, in)
	m.Stub = func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		if strings.HasPrefix(joined, "dpkg -s") {
			return nil, errNotInstalled
		}
		if strings.Contains(joined, "apt-get install") {
			cmds["batcat"] = "/usr/bin/batcat"
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"bat"}))
	requireCalls(t, m, "--no-install-recommends bat")
	resolved, err := os.Readlink(filepath.Join(home, ".local", "bin", "bat"))
	require.NoError(t, err)
	require.Equal(t, "/usr/bin/batcat", resolved)
}

func TestAliasBinarySkippedWhenSourceAbsent(t *testing.T) {
	const y = `packages:
  bat:
    installers:
      - apt:
          aliasBinary:
            batcat: bat
`
	in, _ := newInstaller(t, y, "darwin", cmdMap([]string{"brew", "bat"}), Options{})
	home := tempHome(t, in)
	require.NoError(t, in.Install([]string{"bat"}))
	require.NoFileExists(t, filepath.Join(home, ".local", "bin", "bat"))
}

func TestPostInstallRunsOnFreshInstallOnly(t *testing.T) {
	const y = `packages:
  kitty:
    installers:
      - script:
          url: https://example.com/installer.sh
          args: [launch=n]
    postInstall: ln -fs /Applications/kitty.app/Contents/MacOS/kitty "$HOME/.local/bin/kitty"
`
	in, m := newInstaller(t, y, "darwin", cmdMap(nil), Options{})
	m.Stub = stubOutputs("curl ", "echo installing\n")
	require.NoError(t, in.Install([]string{"kitty"}))
	requireCalls(t, m, "che-script-", " launch=n", `ln -fs /Applications/kitty.app/Contents/MacOS/kitty "$HOME/.local/bin/kitty"`)

	present, mp := newInstaller(t, y, "darwin", cmdMap([]string{"kitty"}), Options{})
	require.NoError(t, present.Install([]string{"kitty"}))
	refuteCalls(t, mp, "ln -fs")
}

func TestPostInstallShippedScriptResolves(t *testing.T) {
	const y = `packages:
  kitty:
    installers:
      - script:
          url: https://example.com/installer.sh
    postInstall:
      path: scripts/post-install-kitty.sh
`
	in, m := newInstaller(t, y, "darwin", cmdMap(nil), Options{})
	in.FilePath = BuiltinPath
	m.Stub = stubOutputs("curl ", "echo installing\n")
	require.NoError(t, in.Install([]string{"kitty"}))
	requireCalls(t, m, `ln -fs "$app/MacOS/kitten" "$bin/kitten"`, `ln -fs "$app/MacOS/kitty" "$bin/kitty"`)
}

// [>] 🤖🤖

type scriptPathsGot struct {
	Items       []string `yaml:"items"`
	PostInstall string   `yaml:"postInstall"`
}

func TestResolveScriptPaths(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_script_paths.test.spec.yml", func(t *testing.T, c testyml.Case[map[string]scriptPathsGot]) (map[string]scriptPathsGot, error) {
		var f File
		require.NoError(t, yaml.Unmarshal([]byte(c.Input.Args.String(t, 1)), &f))
		f.ResolveScriptPaths(c.Input.Args.String(t, 0))
		got := map[string]scriptPathsGot{}
		for name, e := range f.Packages {
			g := scriptPathsGot{}
			for _, it := range e.Items {
				path := ""
				if it.Script != nil {
					path = it.Script.Path
				}
				g.Items = append(g.Items, path)
			}
			if e.PostInstall != nil {
				g.PostInstall = e.PostInstall.Path
			}
			got[name] = g
		}
		return got, nil
	})
}

func TestUserFileScriptNeverFallsBackToBuiltin(t *testing.T) {
	yml := "packages:\n  kitty:\n    installers: [{script: {path: scripts/post-install-kitty.sh}}]\n"
	in, m := newInstaller(t, yml, "darwin", cmdMap(nil), Options{})
	in.FilePath = filepath.Join(t.TempDir(), "packages.yml")
	err := in.Install([]string{"kitty"})
	require.ErrorContains(t, err, "install script not found")
	refuteCalls(t, m, "kitty.app")
}

func TestOverrideAnchoredScriptRuns(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scripts", "my-post.sh"), []byte("echo user-post\n"), 0o755))
	yml := "packages:\n  mytool:\n    installers: [{script: {run: echo install}}]\n    postInstall: {path: scripts/my-post.sh}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "override.yml"), []byte(yml), 0o644))
	o, err := Load(filepath.Join(dir, "override.yml"))
	require.NoError(t, err)
	o.ResolveScriptPaths(dir)

	in, m := newInstaller(t, "packages: {}", "darwin", cmdMap(nil), Options{})
	base, err := LoadBuiltin()
	require.NoError(t, err)
	base.Merge(o)
	in.File, in.FilePath = base, BuiltinPath
	require.NoError(t, in.Install([]string{"mytool"}))
	requireCalls(t, m, filepath.Join(dir, "scripts", "my-post.sh"))
}

// [<] 🤖🤖

func TestGoModulePinsFromEntryVersion(t *testing.T) {
	const y = `packages:
  gopls:
    version: 0.23.0
    installers:
      - go:
          packageName: golang.org/x/tools/gopls
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"go"}), Options{})
	require.NoError(t, in.Install([]string{"gopls"}))
	requireCalls(t, m, "go install golang.org/x/tools/gopls@v0.23.0")
}

func TestVersionLatestTracksModuleHead(t *testing.T) {
	const y = `packages:
  mytool:
    version: latest
    installers:
      - go:
          packageName: example.com/mytool
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"go"}), Options{})
	require.NoError(t, in.Install([]string{"mytool"}))
	requireCalls(t, m, "go install example.com/mytool@latest")
	refuteCalls(t, m, "@vlatest")
}

func TestVersionLatestSkipsDriftChecks(t *testing.T) {
	const y = `packages:
  mytool:
    version: latest
    installers: [brew]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"brew", "mytool"}), Options{})
	require.NoError(t, in.Install([]string{"mytool"}))
	refuteCalls(t, m, "brew install", "brew upgrade")
}
