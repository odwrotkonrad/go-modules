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
	cases := []struct {
		out, pin string
		want     bool
	}{
		{"kind version 0.32.0", "0.32.0", true},
		{"kind version 0.32.0", "0.31.0", false},
		{"Terraform v1.15.8", "1.15.8", true},
		{"go version go1.26.4 darwin/arm64", "1.26.4", true},
		{"Python 3.14.5", "3.14.5", true},
		{"Python 3.14.5", "3.14.4", false},
		{"prometheus, version 3.5.5 (branch: HEAD, revision: dcd3d551ced8)", "3.5.5", true},
		{"glab 1.108.0 (5de20850)", "1.107.0", false},
		{"jq-1.7.1-3", "1.7.1-3", true},
		{"1:1.7.1-3", "1.7.1-3", true},
		{"anything", "", true},
		{"no version here", "1.0.0", false},
	}
	for _, c := range cases {
		require.Equalf(t, c.want, PinMatches(c.out, c.pin), "out=%q pin=%q", c.out, c.pin)
	}
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

func TestVscodeExtensionPinInstallsVersionedAndSkips(t *testing.T) {
	const y = `packages:
  golang.go:
    version: "0.50.0"
    installers: [vscode]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("")
	require.NoError(t, in.Install([]string{"golang.go"}))
	requireCalls(t, m, "code --install-extension golang.go@0.50.0")

	in2, m2 := newInstaller(t, y, "darwin", cmdMap([]string{"code"}), Options{})
	m2.Stub = codeListStub("golang.go@0.50.0\n")
	require.NoError(t, in2.Install([]string{"golang.go"}))
	refuteCalls(t, m2, "--install-extension")
}

func TestVscodeExtensionPinDriftReinstalls(t *testing.T) {
	const y = `packages:
  golang.go:
    version: "0.50.0"
    installers: [vscode]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("golang.go@0.49.0\n")
	require.NoError(t, in.Install([]string{"golang.go"}))
	requireCalls(t, m, "code --install-extension golang.go@0.50.0")
}

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

func TestRollingSentinelIsParseError(t *testing.T) {
	err := yaml.Unmarshal([]byte("packages:\n  node:\n    version: __rolling__\n    installers: [brew]\n"), &File{})
	require.ErrorContains(t, err, "__rolling__ sentinel is gone")
	err = yaml.Unmarshal([]byte("packages:\n  node:\n    installers: [{brew: {version: __rolling__}}]\n"), &File{})
	require.ErrorContains(t, err, "__rolling__ sentinel is gone")
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

func TestBrewLiteralVersionSuffixIsParseError(t *testing.T) {
	err := yaml.Unmarshal([]byte(`packages:
  node:
    version: "24"
    installers: [{brew: {packageName: node@24}}]
`), &File{})
	require.ErrorContains(t, err, "appended automatically")
}

func TestBrewObjectFormVersionSuffixIsParseError(t *testing.T) {
	err := yaml.Unmarshal([]byte(`packages:
  node:
    version: "24"
    installers: [{brew: {formula: node@24}}]
`), &File{})
	require.ErrorContains(t, err, "is gone")
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

func TestAptVocabularyCarriesNameAndAliases(t *testing.T) {
	const y = `packages:
  named:
    installers:
      - apt:
          packageName: a
          aliasBinary: {acat: a}
  legacy:
    installers: [{apt: {installPackages: [a, b]}}]
`
	var f File
	require.ErrorContains(t, yaml.Unmarshal([]byte(y), &f), "installPackages is gone")

	const ok = `packages:
  named:
    installers:
      - apt:
          packageName: a
          aliasBinary: {acat: a}
`
	var g File
	require.NoError(t, yaml.Unmarshal([]byte(ok), &g))
	m := g.Packages["named"].Items[0]
	require.Equal(t, "a", m.Apt.PackageName)
	require.Equal(t, map[string]string{"acat": "a"}, m.AliasBinary)
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
