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
    installMethods: [brew, apt]
`

func TestEntryVersionSkipsWhenMatching(t *testing.T) {
	in, m := newInstaller(t, pinnedBatYaml, "darwin", cmdMap([]string{"brew", "bat"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "bat" {
			return []byte("bat 0.25.0 (871abd2)\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"bat"}))
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "brew install")
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "brew upgrade")
}

func TestEntryVersionDriftRunsManagerUpdate(t *testing.T) {
	in, m := newInstaller(t, pinnedBatYaml, "darwin", cmdMap([]string{"brew", "bat"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "bat" {
			return []byte("bat 0.24.0\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"bat"}))
	require.Contains(t, m.Calls(), "brew upgrade bat")
}

func TestEntryVersionPinsArchiveVersion(t *testing.T) {
	const y = `packages:
  kind:
    version: "0.32.0"
    installMethods:
      - prebuiltArchive:
          url: https://example.com/kind-{version}
          sha256: {linux-amd64: goodsha}
`
	in, m := newInstaller(t, y, "linux", cmdMap([]string{"sha256sum"}), Options{})
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"kind"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "https://example.com/kind-0.32.0")
}

func TestArchiveWithoutVersionNeedsNoPin(t *testing.T) {
	const y = `packages:
  aws:
    - prebuiltArchive:
        url: https://example.com/aws-latest.zip
        platforms: [linux-amd64]
        extractBinaries: [aws/dist/aws]
`
	in, m := newInstaller(t, y, "linux", cmdMap(nil), Options{})
	require.NoError(t, in.Install([]string{"aws"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "aws-latest.zip")
}

// [<] 🤖🤖

func TestVscodeExtensionPinInstallsVersionedAndSkips(t *testing.T) {
	const y = `packages:
  golang.go:
    version: "0.50.0"
    installMethods: [{brew: {vscode: golang.go}}]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("")
	require.NoError(t, in.Install([]string{"golang.go"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "code --install-extension golang.go@0.50.0")

	in2, m2 := newInstaller(t, y, "darwin", cmdMap([]string{"code"}), Options{})
	m2.Stub = codeListStub("golang.go@0.50.0\n")
	require.NoError(t, in2.Install([]string{"golang.go"}))
	require.NotContains(t, strings.Join(m2.Calls(), "\n"), "--install-extension")
}

func TestVscodeExtensionPinDriftReinstalls(t *testing.T) {
	const y = `packages:
  golang.go:
    version: "0.50.0"
    installMethods: [{brew: {vscode: golang.go}}]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("golang.go@0.49.0\n")
	require.NoError(t, in.Install([]string{"golang.go"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "code --install-extension golang.go@0.50.0")
}

func TestBrewVersionedFormulaIsNamedDirectly(t *testing.T) {
	const y = `packages:
  node:
    version: "24"
    installMethods: [{brew: node@24}]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"brew"}), Options{})
	m.Stub = failOn("brew list")
	require.NoError(t, in.Install([]string{"node"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "brew install node@24")
	require.NotContains(t, calls, "node@24@24")
}

func TestUnversionedLatestSentinelSkipsPinning(t *testing.T) {
	const y = `packages:
  jq:
    version: __the_one_published__
    installMethods: [brew, apt]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"brew", "jq"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "jq" {
			return []byte("jq-1.7.1\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"jq"}))
	calls := strings.Join(m.Calls(), "\n")
	require.NotContains(t, calls, "__the_one_published__")
	require.NotContains(t, calls, "brew install")
	require.NotContains(t, calls, "brew upgrade")
}

func TestUnversionedLatestSentinelNotUsedAsArchiveVersion(t *testing.T) {
	const y = `packages:
  kind:
    version: __the_one_published__
    installMethods:
      - prebuiltArchive:
          url: https://example.com/kind-{version}
          sha256: {linux-amd64: goodsha}
`
	in, _ := newInstaller(t, y, "linux", cmdMap([]string{"sha256sum"}), Options{})
	require.ErrorContains(t, in.Install([]string{"kind"}), "no version pinned")
}

func TestAliasBinaryLinksRenamedBinary(t *testing.T) {
	const y = `packages:
  bat:
    version: __the_one_published__
    installMethods:
      - brew
      - apt:
          packages:
            bat:
              aliasBinary:
                batcat: bat
`
	cmds := cmdMap([]string{"apt-get"})
	in, m := newInstaller(t, y, "linux", cmds, Options{})
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
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "--no-install-recommends bat")
	require.Contains(t, calls, "ln -sf /usr/bin/batcat /home/u/.local/bin/bat")
}

func TestAliasBinarySkippedWhenSourceAbsent(t *testing.T) {
	const y = `packages:
  bat:
    version: __the_one_published__
    installMethods:
      - apt:
          packages:
            bat:
              aliasBinary:
                batcat: bat
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"brew", "bat"}), Options{})
	require.NoError(t, in.Install([]string{"bat"}))
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "ln -sf")
}

func TestPostInstallRunsOnFreshInstallOnly(t *testing.T) {
	const y = `packages:
  kitty:
    installMethods:
      - script:
          remoteUrl: https://example.com/installer.sh
          args: [launch=n]
    postInstall: ln -fs /Applications/kitty.app/Contents/MacOS/kitty "$HOME/.local/bin/kitty"
`
	in, m := newInstaller(t, y, "darwin", cmdMap(nil), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "curl" {
			return []byte("echo installing\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"kitty"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "che-script-")
	require.Contains(t, calls, " launch=n")
	require.Contains(t, calls, `ln -fs /Applications/kitty.app/Contents/MacOS/kitty "$HOME/.local/bin/kitty"`)

	present, mp := newInstaller(t, y, "darwin", cmdMap([]string{"kitty"}), Options{})
	require.NoError(t, present.Install([]string{"kitty"}))
	require.NotContains(t, strings.Join(mp.Calls(), "\n"), "ln -fs")
}

func TestPostInstallShippedScriptResolves(t *testing.T) {
	const y = `packages:
  kitty:
    installMethods:
      - script:
          remoteUrl: https://example.com/installer.sh
    postInstall:
      path: scripts/post-install-kitty.sh
`
	in, m := newInstaller(t, y, "darwin", cmdMap(nil), Options{})
	in.FilePath = BuiltinPath
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "curl" {
			return []byte("echo installing\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"kitty"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, `ln -fs "$app/MacOS/kitten" "$bin/kitten"`)
	require.Contains(t, calls, `ln -fs "$app/MacOS/kitty" "$bin/kitty"`)
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
	yml := "packages:\n  kitty:\n    installMethods: [{script: {path: scripts/post-install-kitty.sh}}]\n"
	in, m := newInstaller(t, yml, "darwin", cmdMap(nil), Options{})
	in.FilePath = filepath.Join(t.TempDir(), "packages.yml")
	err := in.Install([]string{"kitty"})
	require.ErrorContains(t, err, "install script not found")
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "kitty.app")
}

func TestOverrideAnchoredScriptRuns(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scripts", "my-post.sh"), []byte("echo user-post\n"), 0o755))
	yml := "packages:\n  mytool:\n    installMethods: [{script: {run: echo install}}]\n    postInstall: {path: scripts/my-post.sh}\n"
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
	require.Contains(t, strings.Join(m.Calls(), "\n"), filepath.Join(dir, "scripts", "my-post.sh"))
}

// [<] 🤖🤖

func TestGoModulePinsFromEntryVersion(t *testing.T) {
	const y = `packages:
  gopls:
    version: 0.23.0
    installMethods:
      - go: golang.org/x/tools/gopls
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"go"}), Options{})
	require.NoError(t, in.Install([]string{"gopls"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "go install golang.org/x/tools/gopls@v0.23.0")
}

func TestAptPackagesAcceptListAndMapForms(t *testing.T) {
	const y = `packages:
  listform:
    installMethods: [{apt: {packages: [a, b]}}]
  mapform:
    installMethods:
      - apt:
          packages:
            a: {}
            b:
              aliasBinary: {bcat: b}
`
	var f File
	require.NoError(t, yaml.Unmarshal([]byte(y), &f))
	require.Equal(t, []string{"a", "b"}, f.Packages["listform"].Items[0].Apt.Packages.Names)
	require.Empty(t, f.Packages["listform"].Items[0].AliasBinary)

	m := f.Packages["mapform"].Items[0]
	require.Equal(t, []string{"a", "b"}, m.Apt.Packages.Names)
	require.Equal(t, map[string]string{"bcat": "b"}, m.AliasBinary)
}

func TestVersionLatestTracksModuleHead(t *testing.T) {
	const y = `packages:
  mytool:
    version: latest
    installMethods:
      - go: example.com/mytool
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"go"}), Options{})
	require.NoError(t, in.Install([]string{"mytool"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "go install example.com/mytool@latest")
	require.NotContains(t, calls, "@vlatest")
}

func TestVersionLatestSkipsDriftChecks(t *testing.T) {
	const y = `packages:
  mytool:
    version: latest
    installMethods: [brew]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"brew", "mytool"}), Options{})
	require.NoError(t, in.Install([]string{"mytool"}))
	calls := strings.Join(m.Calls(), "\n")
	require.NotContains(t, calls, "brew install")
	require.NotContains(t, calls, "brew upgrade")
}
