package packages

// [>] 🤖🤖

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func TestLoadBuiltinParsesShippedFile(t *testing.T) {
	f, err := LoadBuiltin()
	require.NoError(t, err)
	require.Contains(t, f.Packages, "bat")
	brew, err := f.Find("brew", BuiltinPath)
	require.NoError(t, err)
	require.Equal(t, "script", brew.Items[0].Mgr)
	require.Equal(t, "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh", brew.Items[0].Script.URL)
}

func TestLoadReadsFileAndErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "packages.yml")
	require.NoError(t, os.WriteFile(path, []byte("packages:\n  bat: [brew]\n"), 0o644))
	f, err := Load(path)
	require.NoError(t, err)
	require.Contains(t, f.Packages, "bat")

	_, err = Load(filepath.Join(dir, "absent.yml"))
	require.ErrorContains(t, err, "packages file not found")

	require.NoError(t, os.WriteFile(path, []byte(":not yaml ["), 0o644))
	_, err = Load(path)
	require.ErrorContains(t, err, "parse")
}

func TestMergeNilOverrideAndNilBase(t *testing.T) {
	f := &File{Packages: map[string]Entry{"bat": {Items: []Item{{Mgr: "brew"}}}}}
	f.Merge(nil)
	require.Len(t, f.Packages, 1)
	empty := &File{}
	empty.Merge(&File{Packages: map[string]Entry{"fd": {Items: []Item{{Mgr: "brew"}}}}})
	require.Contains(t, empty.Packages, "fd")
}

func TestNewHostCapturesRuntime(t *testing.T) {
	h := NewHost()
	require.NotEmpty(t, h.OS)
	require.NotEmpty(t, h.Arch)
	require.NotNil(t, h.LookPath)
	require.NotEmpty(t, h.PathDirs())
	require.Equal(t, os.Getenv("PATH"), h.Getenv("PATH"))
}

func TestApplicableGemAndCask(t *testing.T) {
	h := testHost("darwin", "amd64", cmdMap([]string{"gem", "brew"}))
	it, ok, err := h.pickPreferred("x", Entry{Items: []Item{{Mgr: "gem"}}}, nil, nil, nil, false)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "gem", it.Mgr)
	it, ok, err = h.pickPreferred("x", Entry{Items: []Item{{Mgr: "cask", Name: "x"}}}, nil, nil, nil, false)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "cask", it.Mgr)
}

func TestInstallGoModule(t *testing.T) {
	in, m := newInstaller(t, `packages:
  gopls: [{go: {packageName: "golang.org/x/tools/gopls@latest"}}]`, "darwin", cmdMap([]string{"go"}), Options{})
	require.NoError(t, in.Install([]string{"gopls"}))
	require.Equal(t, []string{"go install golang.org/x/tools/gopls@latest"}, m.Calls())
}

func TestInstallGemWithLinuxSudo(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  ruby-lsp: [{gem: {packageName: ruby-lsp}}]", "linux", cmdMap([]string{"gem"}), Options{})
	require.NoError(t, in.Install([]string{"ruby-lsp"}))
	require.Equal(t, []string{"sudo gem install ruby-lsp"}, m.Calls())
}

func TestInstallCaskWhenMissing(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  beekeeper-studio: [{brew/cask: {packageName: beekeeper-studio}}]", "darwin", cmdMap([]string{"brew"}), Options{})
	m.Stub = failOn("brew list")
	require.NoError(t, in.Install([]string{"beekeeper-studio"}))
	require.Contains(t, m.Calls(), "brew install --cask beekeeper-studio")
}

func TestUpdateBranches(t *testing.T) {
	cases := []struct {
		name, yaml, osname string
		cmds               []string
		want               []string
	}{
		{
			"cask", "packages:\n  beekeeper-studio: [{brew/cask: {packageName: beekeeper-studio}}]", "darwin",
			[]string{"brew"},
			[]string{"brew upgrade --cask beekeeper-studio"},
		},
		{
			"apt", "packages:\n  jq: [apt]", "linux",
			[]string{"apt-get"},
			[]string{"sudo apt-get update", "sudo apt-get install --only-upgrade --yes jq"},
		},
		{
			"npm", "packages:\n  corepack: [npm]", "darwin",
			[]string{"npm", "corepack"},
			[]string{"npm update --global corepack"},
		},
		{
			"gem", "packages:\n  ruby-lsp: [{gem: {packageName: ruby-lsp}}]", "linux",
			[]string{"gem", "ruby-lsp"},
			[]string{"sudo gem update ruby-lsp"},
		},
		{
			"go", "packages:\n  gopls: [{go: {packageName: \"golang.org/x/tools/gopls@latest\"}}]", "darwin",
			[]string{"go", "gopls"},
			[]string{"go install golang.org/x/tools/gopls@latest"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in, m := newInstaller(t, c.yaml, c.osname, cmdMap(c.cmds), Options{Update: true})
			pkg := strings.Split(strings.Split(c.yaml, "\n")[1], ":")[0]
			require.NoError(t, in.Install([]string{strings.TrimSpace(pkg)}))
			for _, w := range c.want {
				require.Contains(t, m.Calls(), w)
			}
		})
	}
}

func TestUpdateDryRunAnnounces(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  bat: [brew]", "darwin", cmdMap([]string{"brew"}), Options{Update: true, DryRun: true})
	out, err := captureStdout(t, func() error { return in.Install([]string{"bat"}) })
	require.NoError(t, err)
	wantLines(t, out, "update bat via brew (dry run)")
	require.NotContains(t, m.Calls(), "brew upgrade bat")
}

func TestInstalledVersionAptAndNpmMisses(t *testing.T) {
	in, m := newInstaller(t, "packages: {}", "linux", cmdMap(nil), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "dpkg-query" {
			return []byte("1.6-2\n"), nil
		}
		return []byte("other@1.0\n"), nil
	}
	require.Equal(t, "1.6-2", in.installedVersion("apt", "jq"))
	require.Equal(t, "", in.installedVersion("npm", "jq"))
	require.Equal(t, "", in.installedVersion("brew", "jq"))
	m.Stub = failOn("dpkg-query", "npm")
	require.Equal(t, "", in.installedVersion("apt", "jq"))
	require.Equal(t, "", in.installedVersion("npm", "jq"))
}

func TestInstallAptPinReinstallsOnDrift(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  jq:\n    version: 1.7-1\n    installers: [apt]", "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = stubOutputs("dpkg-query", "1.6-2")
	require.NoError(t, in.Install([]string{"jq"}))
	require.Contains(t, m.Calls(), "sudo apt-get install --yes --no-install-recommends jq=1.7-1")
}

func TestCheckUpgradableAptAndNpmOutdated(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  jq: [apt]\n  corepack: [npm]\n  fd: [apt]", "linux",
		cmdMap([]string{"apt-get", "npm", "corepack", "jq", "fd"}), Options{})
	m.Stub = stubOutputs(
		"apt list", "Listing...\njq/stable 1.7 amd64 [upgradable from: 1.6]\n",
		"npm outdated", "/g:corepack@0.30.0:corepack@0.29.0:\n")
	out, err := captureStdout(t, func() error { return in.CheckUpgradable([]string{"jq", "corepack", "fd"}) })
	require.NoError(t, err)
	wantLines(t, out, "upgradable jq via apt", "upgradable corepack via npm")
	notLine(t, out, "upgradable fd")
}

func TestManagerBinDirs(t *testing.T) {
	in, m := newInstaller(t, "packages: {}", "linux", cmdMap(nil), Options{})
	m.Stub = stubOutputs("brew ", "/opt/homebrew\n", "npm ", "/usr/local\n")
	env := map[string]string{}
	in.Host.Getenv = func(k string) string { return env[k] }
	require.Equal(t, "/opt/homebrew/bin", in.managerBinDir("brew"))
	require.Equal(t, "/usr/bin", in.managerBinDir("apt"))
	require.Equal(t, "/usr/local/bin", in.managerBinDir("npm"))
	require.Equal(t, "", in.managerBinDir("cask"))
	require.Equal(t, "", in.managerBinDir("gem"))
	require.Equal(t, "", in.managerBinDir("script"))
	require.Equal(t, "", in.managerBinDir("code"))
	env["HOME"] = "/home/u"
	require.Equal(t, "/home/u/.local/bin", in.managerBinDir("binariesRemoteArchive"))
	require.Equal(t, "/home/u/go/bin", in.managerBinDir("go"))
	env["GOPATH"] = "/gopath"
	require.Equal(t, "/gopath/bin", in.managerBinDir("go"))
	env["GOBIN"] = "/gobin"
	require.Equal(t, "/gobin", in.managerBinDir("go"))
}

func TestInstallBinaryZipAsset(t *testing.T) {
	const zipYaml = `packages:
  terraform:
    - binariesRemoteArchive:
        platformEligibility:
          - linux-amd64: sha256:goodsha
        version: 1.15.0
        url: https://example.com/terraform_{version}_{os}_{arch}.zip
`
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	zf, err := zw.CreateHeader(&zip.FileHeader{Name: "terraform"})
	require.NoError(t, err)
	_, err = zf.Write([]byte("#!/bin/sh\n"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	in, _ := newInstaller(t, withSha(zipYaml, zbuf.Bytes()), "linux", cmdMap(nil), Options{})
	home := tempHome(t, in)
	testFetch.Bodies["https://example.com/terraform_1.15.0_linux_amd64.zip"] = zbuf.Bytes()
	require.NoError(t, in.Install([]string{"terraform"}))
	require.FileExists(t, filepath.Join(home, ".local", "opt", "terraform", "terraform"))
	requireSymlink(t, filepath.Join(home, ".local", "bin", "terraform"), filepath.Join(home, ".local", "opt", "terraform", "terraform"))
}

func TestDefaultEmittersWriteStdout(t *testing.T) {
	in, _ := newInstaller(t, "packages:\n  bat: [brew]", "linux", cmdMap(nil), Options{DryRun: true})
	out, err := captureStdout(t, func() error {
		in.emit(log.Levels.Info, "installed", "x via brew")
		in.emitSkip(log.Levels.Info, "x", "already installed via brew")
		in.emitDryRun("install", "x via brew")
		return nil
	})
	require.NoError(t, err)
	wantLines(t, out, "installed x via brew", "will not install x: already installed via brew", "install x via brew (dry run)")
}

// [<] 🤖🤖
