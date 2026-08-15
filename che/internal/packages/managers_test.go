package packages

// [>] 🤖🤖

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/fetchx"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

var testFetch *fetchx.Mock

func newInstaller(t *testing.T, filesYaml, osname string, cmds map[string]string, opts Options) (*Installer, *execx.Mock) {
	t.Helper()
	var f File
	require.NoError(t, yaml.Unmarshal([]byte(filesYaml), &f))
	m := &execx.Mock{}
	execx.Swap(t, m)
	testFetch = &fetchx.Mock{Bodies: map[string][]byte{}}
	fetchx.Swap(t, testFetch)
	return &Installer{File: &f, FilePath: "packages.yml", Host: testHost(osname, "amd64", cmds), Opts: opts}, m
}

func failOn(prefixes ...string) func(argv []string) ([]byte, error) {
	return func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		for _, p := range prefixes {
			if strings.HasPrefix(joined, p) {
				return nil, fmt.Errorf("fail: %s", joined)
			}
		}
		return nil, nil
	}
}

func stubOutputs(pairs ...string) func(argv []string) ([]byte, error) {
	return func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		for i := 0; i+1 < len(pairs); i += 2 {
			if strings.HasPrefix(joined, pairs[i]) {
				return []byte(pairs[i+1]), nil
			}
		}
		return nil, nil
	}
}

func chainStubs(stubs ...func(argv []string) ([]byte, error)) func(argv []string) ([]byte, error) {
	return func(argv []string) ([]byte, error) {
		for _, s := range stubs {
			if out, err := s(argv); out != nil || err != nil {
				return out, err
			}
		}
		return nil, nil
	}
}

func tempHome(t *testing.T, in *Installer) string {
	t.Helper()
	home := t.TempDir()
	in.Host.Getenv = func(k string) string {
		if k == "HOME" {
			return home
		}
		return ""
	}
	return home
}

func shaHex(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

func withSha(yml string, body []byte) string { return strings.ReplaceAll(yml, "goodsha", shaHex(body)) }

func tarBytes(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, name := range names {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: 3}))
		_, err := tw.Write([]byte("bin"))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	return buf.Bytes()
}

func tarGzBody(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(tarBytes(t, names...))
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func tarXzBody(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	xw, err := xz.NewWriter(&buf)
	require.NoError(t, err)
	_, err = xw.Write(tarBytes(t, names...))
	require.NoError(t, err)
	require.NoError(t, xw.Close())
	return buf.Bytes()
}

func zipBody(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range names {
		f, err := zw.CreateHeader(&zip.FileHeader{Name: name})
		require.NoError(t, err)
		_, err = f.Write([]byte("bin"))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func requireCalls(t *testing.T, m *execx.Mock, fragments ...string) {
	t.Helper()
	joined := strings.Join(m.Calls(), "\n")
	for _, f := range fragments {
		require.Contains(t, joined, f)
	}
}

func refuteCalls(t *testing.T, m *execx.Mock, fragments ...string) {
	t.Helper()
	joined := strings.Join(m.Calls(), "\n")
	for _, f := range fragments {
		require.NotContains(t, joined, f)
	}
}

func TestInstallBrewWhenMissing(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  bat: [brew, apt]", "darwin", cmdMap([]string{"brew"}), Options{})
	m.Stub = failOn("brew list")
	require.NoError(t, in.Install([]string{"bat"}))
	require.Contains(t, m.Calls(), "brew install bat")
}

func TestInstallBrewSkipsWhenInstalled(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  bat: [brew, apt]", "darwin", cmdMap([]string{"brew"}), Options{})
	require.NoError(t, in.Install([]string{"bat"}))
	require.NotContains(t, m.Calls(), "brew install bat")
}

const tapYaml = `installerRegistries:
  brew:
    - konradodwrot/tap
packages:
  che: [{brew: {fromRegistry: konradodwrot/tap}}]
`

func TestOnlyMethodsSpareDependencies(t *testing.T) {
	const y = `packages:
  nvm: [{script: {url: https://example.com/nvm.sh}}]
  node:
    requires: [nvm]
    installers: [{nvm: {versions: ["24.0.0"], global: "24.0.0"}}]
`
	in, m := newInstaller(t, y, "linux", cmdMap(nil), Options{OnlyMethods: []string{"nvm"}})
	require.NoError(t, in.Install([]string{"node"}))
	requireCalls(t, m, "che-script-", "nvm install 24.0.0")
}

func TestInstallBrewUpdatesIndexOncePerRun(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  bat: [brew]\n  fd: [brew]", "darwin", cmdMap([]string{"brew"}), Options{})
	m.Stub = failOn("brew list")
	require.NoError(t, in.Install([]string{"bat", "fd"}))
	requireCalls(t, m, "brew update --quiet", "brew install bat", "brew install fd")
	require.Equal(t, 1, strings.Count(strings.Join(m.Calls(), "\n"), "brew update --quiet"))
}

func TestInstallBrewTapQualified(t *testing.T) {
	in, m := newInstaller(t, tapYaml, "darwin", cmdMap([]string{"brew"}), Options{})
	m.Stub = failOn("brew list")
	require.NoError(t, in.Install([]string{"che"}))
	require.Equal(t, []string{
		"brew list che",
		"brew update --quiet",
		"brew tap konradodwrot/tap",
		"brew trust konradodwrot/tap",
		"brew install konradodwrot/tap/che",
	}, m.Calls())
}

func TestInstallBrewUnknownRegistryErrors(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  che: [{brew: {fromRegistry: nowhere/tap}}]", "darwin", cmdMap([]string{"brew"}), Options{})
	m.Stub = failOn("brew list")
	require.ErrorContains(t, in.Install([]string{"che"}), `unknown brew registry "nowhere/tap"`)
}

func TestAptIneligibleOnNonDebianLinux(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  jq: [apt]", "linux", cmdMap([]string{"apt-get"}), Options{})
	in.Host.Distro = ""
	_, err := captureStdout(t, func() error { return in.Install([]string{"jq"}) })
	require.ErrorContains(t, err, "no applicable installation method for jq")
	require.Empty(t, m.Calls())
}

func TestEligibleInstallersMostSpecificKeyWins(t *testing.T) {
	const y = `osInstallers:
  linux: [npm]
  linux-debian: [apt]
packages:
  jq: [apt]
`
	in, m := newInstaller(t, y, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = failOn("dpkg -s")
	require.NoError(t, in.Install([]string{"jq"}))
	require.Contains(t, m.Calls(), "sudo apt-get install --yes --no-install-recommends jq")
}

func TestInstallAptSudoAndUpdateOnce(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  fd: [brew, {apt: {packageName: fd-find}}]\n  jq: [apt]", "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = failOn("dpkg -s")
	require.NoError(t, in.Install([]string{"fd", "jq"}))
	calls := m.Calls()
	require.Contains(t, calls, "sudo apt-get update")
	require.Contains(t, calls, "sudo apt-get install --yes --no-install-recommends fd-find")
	require.Contains(t, calls, "sudo apt-get install --yes --no-install-recommends jq")
	require.Equal(t, 1, strings.Count(strings.Join(calls, "\n"), "apt-get update"))
}

func TestInstallNpmLinksNvmGlobalBins(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  tsc: [npm]", "linux", cmdMap([]string{"npm"}), Options{})
	home := tempHome(t, in)
	bin := filepath.Join(home, ".nvm", "versions", "node", "v24.0.0", "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "tsc"), []byte("#!/bin/sh\n"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".nvm", "alias"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".nvm", "alias", "default"), []byte("v24.0.0\n"), 0o644))
	m.Stub = failOn("npm ls")
	require.NoError(t, in.Install([]string{"tsc"}))
	requireCalls(t, m, "npm install --global tsc")
	target, err := os.Readlink(filepath.Join(home, ".local", "bin", "tsc"))
	require.NoError(t, err)
	require.Equal(t, filepath.Join(bin, "tsc"), target)
}

func TestInstallNpmPinReinstallsOnDrift(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  ccstatusline:\n    version: 2.2.22\n    installers: [npm]", "darwin",
		cmdMap([]string{"npm", "ccstatusline"}), Options{})
	m.Stub = stubOutputs("npm ls ", "ccstatusline@2.2.0\n")
	require.NoError(t, in.Install([]string{"ccstatusline"}))
	require.Contains(t, m.Calls(), "npm install --global ccstatusline@2.2.22")
}

func TestInstallNpmPinSkipsWhenMatching(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  ccstatusline:\n    version: 2.2.22\n    installers: [npm]", "darwin",
		cmdMap([]string{"npm", "ccstatusline"}), Options{})
	m.Stub = stubOutputs("npm ls ", "ccstatusline@2.2.22\n")
	require.NoError(t, in.Install([]string{"ccstatusline"}))
	require.NotContains(t, m.Calls(), "npm install --global ccstatusline@2.2.22")
}

func TestInstallGemPinPassesVersionRequirement(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  ruby-lsp:\n    version: 0.26.10\n    installers: [{gem: {packageName: ruby-lsp}}]", "linux",
		cmdMap([]string{"gem"}), Options{})
	require.NoError(t, in.Install([]string{"ruby-lsp"}))
	require.Contains(t, m.Calls(), "sudo gem install ruby-lsp -v 0.26.10")
}

func TestInstallGoPinResolvesModuleVersion(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  gopls:\n    version: 0.23.0\n    installers: [{go: {packageName: golang.org/x/tools/gopls@latest}}]", "darwin",
		cmdMap([]string{"go"}), Options{})
	require.NoError(t, in.Install([]string{"gopls"}))
	require.Contains(t, m.Calls(), "go install golang.org/x/tools/gopls@v0.23.0")
}

func TestPinnedName(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/pinned_name.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		return pinnedName(c.Input.Args.String(t, 0), c.Input.Args.String(t, 1), c.Input.Args.String(t, 2)), nil
	})
}

func TestInstallUpdateUpgradesInstalledBrew(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  bat: [brew]", "darwin", cmdMap([]string{"brew"}), Options{Update: true})
	require.NoError(t, in.Install([]string{"bat"}))
	require.Contains(t, m.Calls(), "brew upgrade bat")
}

func TestInstallIfMissingSkipsPresentCommand(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  bat: [brew]", "darwin", cmdMap([]string{"brew", "bat"}), Options{IfMissing: true})
	require.NoError(t, in.Install([]string{"bat"}))
	require.Empty(t, m.Calls())
}

func TestInstallDryRunAnnouncesWithoutInstalling(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  bat: [brew]", "darwin", cmdMap([]string{"brew"}), Options{DryRun: true})
	m.Stub = failOn("brew list")
	out, err := captureStdout(t, func() error { return in.Install([]string{"bat"}) })
	require.NoError(t, err)
	wantLines(t, out, "install bat via brew (dry run)")
	require.NotContains(t, m.Calls(), "brew install bat")
}

func TestInstallRoundsServeLaterPackages(t *testing.T) {
	cmds := cmdMap([]string{"apt-get"})
	in, m := newInstaller(t, "packages:\n  x: [{npm: {packageName: x}}]\n  npm: [{apt: {packageName: npm}}]", "linux", cmds, Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		if strings.HasPrefix(joined, "dpkg -s") || strings.HasPrefix(joined, "npm ls") {
			return nil, fmt.Errorf("not installed")
		}
		if joined == "sudo apt-get install --yes --no-install-recommends npm" {
			cmds["npm"] = "/usr/bin/npm"
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"x", "npm"}))
	require.Contains(t, m.Calls(), "sudo npm install --global x")
}

func TestInstallMissingMethodWarnDowngrades(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  bat: [brew]", "linux", cmdMap(nil), Options{MissingMethodWarn: true})
	out, err := captureStdout(t, func() error { return in.Install([]string{"bat"}) })
	require.NoError(t, err)
	wantLines(t, out, "will not install bat: no applicable installation method")
	require.Empty(t, m.Calls())
}

func TestInstallSkipsNoApplicableManager(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  bat: [brew]", "linux", cmdMap(nil), Options{})
	_, err := captureStdout(t, func() error { return in.Install([]string{"bat"}) })
	require.ErrorContains(t, err, "no applicable installation method for bat")
	require.Empty(t, m.Calls())
}

const brewScriptYaml = `packages:
  brew:
    - script:
        os: darwin
        run: |
          export NONINTERACTIVE=1
          /bin/bash -c "$(curl -fsSL https://example.com/install.sh)"
`

func TestInstallScriptRunsShWhenMissing(t *testing.T) {
	in, m := newInstaller(t, brewScriptYaml, "darwin", cmdMap(nil), Options{})
	require.NoError(t, in.Install([]string{"brew"}))
	calls := m.Calls()
	require.Len(t, calls, 1)
	require.True(t, strings.HasPrefix(calls[0], "/bin/sh -ec export NONINTERACTIVE=1"))
}

func TestInstallScriptSkipsWhenCommandPresent(t *testing.T) {
	in, m := newInstaller(t, brewScriptYaml, "darwin", cmdMap([]string{"brew"}), Options{})
	require.NoError(t, in.Install([]string{"brew"}))
	require.Empty(t, m.Calls())
}

func TestInstallScriptSkipsOnForeignOs(t *testing.T) {
	in, m := newInstaller(t, brewScriptYaml, "linux", cmdMap(nil), Options{})
	_, err := captureStdout(t, func() error { return in.Install([]string{"brew"}) })
	require.ErrorContains(t, err, "no applicable installation method for brew")
	require.Empty(t, m.Calls())
}

func TestInstallScriptDryRunAnnounces(t *testing.T) {
	in, m := newInstaller(t, brewScriptYaml, "darwin", cmdMap(nil), Options{DryRun: true})
	out, err := captureStdout(t, func() error { return in.Install([]string{"brew"}) })
	require.NoError(t, err)
	wantLines(t, out, "install brew via script (dry run)")
	require.Empty(t, m.Calls())
}

func TestInstallScriptPathRunsUserFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scripts", "install-x.sh"), []byte("#!/bin/bash\n"), 0o755))
	in, m := newInstaller(t, "packages:\n  x: [{script: {path: scripts/install-x.sh}}]", "darwin", cmdMap(nil), Options{})
	in.FilePath = filepath.Join(dir, "packages.yml")
	require.NoError(t, in.Install([]string{"x"}))
	require.Equal(t, []string{"/bin/sh -e " + filepath.Join(dir, "scripts", "install-x.sh")}, m.Calls())
}

func TestInstallScriptPathMissingEverywhereErrors(t *testing.T) {
	in, _ := newInstaller(t, "packages:\n  x: [{script: {path: scripts/nope.sh}}]", "linux", cmdMap(nil), Options{})
	in.FilePath = BuiltinPath
	require.ErrorContains(t, in.Install([]string{"x"}), "install script not found: scripts/nope.sh")
}

func TestInstallScriptRemoteUrlFetchesAndRuns(t *testing.T) {
	const yml = `packages:
  brew:
    - script:
        os: darwin
        url: https://example.com/install.sh
        env: {NONINTERACTIVE: "1"}
`
	in, m := newInstaller(t, yml, "darwin", cmdMap(nil), Options{})
	testFetch.Bodies["https://example.com/install.sh"] = []byte("echo installing\n")
	require.NoError(t, in.Install([]string{"brew"}))
	require.Contains(t, testFetch.Calls(), "https://example.com/install.sh")
	require.Contains(t, m.Calls()[0], "che-script-")
	require.Contains(t, m.Envs()[0], "NONINTERACTIVE=1")
}

func TestInstallScriptRemoteUrlFetchFailureErrors(t *testing.T) {
	const yml = `packages:
  x: [{script: {url: https://example.com/install.sh}}]
`
	in, _ := newInstaller(t, yml, "darwin", cmdMap(nil), Options{})
	testFetch.Err = fmt.Errorf("boom")
	require.ErrorContains(t, in.Install([]string{"x"}), "install script fetch failed")
}

const gcloudYaml = `installerRegistries:
  apt:
    - url: https://packages.cloud.google.com/apt
      verificationKey: https://packages.cloud.google.com/apt/doc/apt-key.gpg
      suites: cloud-sdk
      components: main
packages:
  gcloud:
    - apt:
        packageName: google-cloud-cli
        fromRegistry: packages.cloud.google.com/apt
    - script:
        platformEligibility:
          - darwin-arm64: sha256:gsha
        os: darwin
        version: 572.0.0
        run: echo install-gcloud
`

func TestInstallScriptPinReinstallsOnDrift(t *testing.T) {
	in, m := newInstaller(t, gcloudYaml, "darwin", cmdMap([]string{"gcloud"}), Options{})
	in.Host.Arch = "arm64"
	in.FilePath = BuiltinPath
	m.Stub = stubOutputs("gcloud ", "Google Cloud SDK 570.0.0\n")
	require.NoError(t, in.Install([]string{"gcloud"}))
	requireCalls(t, m, "/bin/sh -ec")
}

func TestInstallScriptPinSkipsWhenMatching(t *testing.T) {
	in, m := newInstaller(t, gcloudYaml, "darwin", cmdMap([]string{"gcloud"}), Options{})
	in.Host.Arch = "arm64"
	m.Stub = stubOutputs("gcloud ", "Google Cloud SDK 572.0.0\n")
	require.NoError(t, in.Install([]string{"gcloud"}))
	require.Equal(t, []string{"gcloud --version"}, m.Calls())
}

func TestInstallScriptShaGatesApplicability(t *testing.T) {
	in, m := newInstaller(t, gcloudYaml, "darwin", cmdMap(nil), Options{})
	in.Host.Arch = "amd64"
	_, err := captureStdout(t, func() error { return in.Install([]string{"gcloud"}) })
	require.ErrorContains(t, err, "no applicable installation method for gcloud")
	require.Empty(t, m.Calls())
}

func TestInstallGcloudPicksLinuxAptRepo(t *testing.T) {
	in, m := newInstaller(t, gcloudYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	in.FilePath = BuiltinPath
	m.Stub = aptStub(t)
	require.NoError(t, in.Install([]string{"gcloud"}))
	require.Contains(t, testFetch.Calls(), "https://packages.cloud.google.com/apt/doc/apt-key.gpg")
	requireCalls(t, m,
		"/etc/apt/sources.list.d/packages.cloud.google.com-apt-cloud-sdk-main.sources",
		"--no-install-recommends -t cloud-sdk google-cloud-cli")
}

func TestScriptEnv(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/script_env.test.spec.yml", func(t *testing.T, c testyml.Case[map[string]string]) (map[string]string, error) {
		in, _ := newInstaller(t, c.Input.Args.String(t, 0), c.Input.Args.String(t, 1), cmdMap(nil), Options{})
		in.Host.Arch = c.Input.Args.String(t, 2)
		pkg := c.Input.Args.String(t, 3)
		var s *ScriptSpec
		for _, it := range in.File.Packages[pkg].Items {
			if it.Script != nil {
				s = it.Script
			}
		}
		require.NotNil(t, s)
		got := map[string]string{}
		for _, kv := range in.scriptEnv(pkg, s) {
			k, v, _ := strings.Cut(kv, "=")
			if strings.HasPrefix(k, "CHE_PKG_") {
				got[k] = v
			}
		}
		return got, nil
	})
}

func TestArchForUnknownConventionErrors(t *testing.T) {
	in, _ := newInstaller(t, "packages:\n  x:\n    - binariesRemoteArchive:\n        archScheme: gnu\n        platformEligibility: [{linux-amd64: sha256:s}]\n        version: \"1\"\n        url: https://example.com/x-{arch}", "linux", cmdMap(nil), Options{})
	require.ErrorContains(t, in.Install([]string{"x"}), `unknown archScheme "gnu"`)
}

func TestCheckUpgradableScriptPinDrift(t *testing.T) {
	in, m := newInstaller(t, gcloudYaml, "darwin", cmdMap([]string{"gcloud"}), Options{})
	in.Host.Arch = "arm64"
	m.Stub = stubOutputs("gcloud ", "Google Cloud SDK 570.0.0\n")
	out, err := captureStdout(t, func() error { return in.CheckUpgradable([]string{"gcloud"}) })
	require.NoError(t, err)
	wantLines(t, out, "upgradable gcloud via script: yaml pins 572.0.0")
}

func TestInstallPythonPyenvPinFromBuiltin(t *testing.T) {
	f, err := LoadBuiltin()
	require.NoError(t, err)
	m := &execx.Mock{}
	execx.Swap(t, m)
	in := &Installer{File: f, FilePath: BuiltinPath, Host: testHost("darwin", "arm64", cmdMap([]string{"python3", "pyenv", "brew"})), Opts: Options{}}
	m.Stub = func(argv []string) ([]byte, error) {
		if strings.Join(argv, " ") == "pyenv versions --bare" {
			return []byte("3.9.6\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"python3"}))
	requireCalls(t, m, "pyenv install --skip-existing 3.14.6", "pyenv global 3.14.6")

	m.Stub = func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		if joined == "pyenv versions --bare" {
			return []byte("3.14.6\n3.13.14\n"), nil
		}
		if joined == "pyenv global" {
			return []byte("3.14.6\n"), nil
		}
		return nil, nil
	}
	before := len(m.Calls())
	require.NoError(t, in.Install([]string{"python3"}))
	require.NotContains(t, strings.Join(m.Calls()[before:], "\n"), "pyenv install --skip-existing")
}

const kubectlYaml = `packages:
  kubectl:
    installers:
      - binariesRemoteArchive:
          platformEligibility:
            - linux-amd64: sha256:goodsha
          version: 1.36.3
          url: https://example.com/v{version}/kubectl
    versionCommand: kubectl version --client
`

func TestVersionCommandOverridesProbe(t *testing.T) {
	in, m := newInstaller(t, kubectlYaml, "linux", cmdMap([]string{"kubectl", "sha256sum"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "kubectl" && argv[1] == "version" && argv[2] == "--client" {
			return []byte("Client Version: v1.36.3\n"), nil
		}
		return nil, fmt.Errorf("unexpected probe: %v", argv)
	}
	require.NoError(t, in.Install([]string{"kubectl"}))
	require.Equal(t, []string{"kubectl version --client"}, m.Calls())
}

func TestVersionCommandDriftReinstalls(t *testing.T) {
	body := []byte("kubectl-bin")
	in, m := newInstaller(t, withSha(kubectlYaml, body), "linux", cmdMap([]string{"kubectl"}), Options{})
	tempHome(t, in)
	m.Stub = stubOutputs("kubectl ", "Client Version: v1.36.0\n")
	testFetch.Bodies["https://example.com/v1.36.3/kubectl"] = body
	require.NoError(t, in.Install([]string{"kubectl"}))
	require.NotEmpty(t, testFetch.Calls())
}

const codeExtYaml = "packages:\n  golang.go: [vscode]\n  redhat.vscode-yaml: [vscode]\n  code: [{brew/cask: {packageName: visual-studio-code}}]"

func codeListStub(installed string) func(argv []string) ([]byte, error) {
	return func(argv []string) ([]byte, error) {
		if argv[0] == "code" && argv[1] == "--list-extensions" {
			return []byte(installed), nil
		}
		return nil, nil
	}
}

func TestInstallCodeExtensionWhenMissing(t *testing.T) {
	in, m := newInstaller(t, codeExtYaml, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("redhat.vscode-yaml\n")
	require.NoError(t, in.Install([]string{"golang.go"}))
	require.Contains(t, m.Calls(), "code --install-extension golang.go")
}

func TestInstallCodeExtensionSkipsInstalledAndListsOnce(t *testing.T) {
	in, m := newInstaller(t, codeExtYaml, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("Golang.Go\nredhat.vscode-yaml\n")
	require.NoError(t, in.Install([]string{"golang.go", "redhat.vscode-yaml"}))
	require.Equal(t, []string{"code --list-extensions --show-versions"}, m.Calls())
}

func TestInstallCodeExtensionAttemptsWithoutCodeCommand(t *testing.T) {
	in, m := newInstaller(t, codeExtYaml, "linux", cmdMap(nil), Options{})
	require.NoError(t, in.Install([]string{"golang.go"}))
	requireCalls(t, m, "code --install-extension golang.go")
}

func TestInstallCodeExtensionUpdateForces(t *testing.T) {
	in, m := newInstaller(t, codeExtYaml, "darwin", cmdMap([]string{"code"}), Options{Update: true})
	m.Stub = codeListStub("golang.go\n")
	require.NoError(t, in.Install([]string{"golang.go"}))
	require.Contains(t, m.Calls(), "code --install-extension golang.go --force")
}

func TestInstallCodeExtensionRoundsAfterCask(t *testing.T) {
	cmds := cmdMap([]string{"brew"})
	in, m := newInstaller(t, codeExtYaml, "darwin", cmds, Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		if strings.HasPrefix(joined, "brew list") {
			return nil, fmt.Errorf("not installed")
		}
		if joined == "brew install --cask visual-studio-code" {
			cmds["code"] = "/usr/local/bin/code"
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"golang.go", "code"}))
	require.Contains(t, m.Calls(), "code --install-extension golang.go")
}

func TestCheckPresentUsesExtensionListForCodePackages(t *testing.T) {
	in, m := newInstaller(t, codeExtYaml, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("golang.go\n")
	out, err := captureStdout(t, func() error {
		require.Equal(t, []string{"redhat.vscode-yaml"}, in.CheckPresent([]string{"golang.go", "redhat.vscode-yaml"}))
		return nil
	})
	require.NoError(t, err)
	wantLines(t, out, "missing redhat.vscode-yaml")
	notLine(t, out, "missing golang.go")
}

func TestInstallUnknownPackageErrors(t *testing.T) {
	in, _ := newInstaller(t, "packages: {}", "darwin", cmdMap([]string{"brew"}), Options{})
	err := in.Install([]string{"nope"})
	require.ErrorContains(t, err, "unknown package: nope (required entry in packages.yml)")
}

// [<] 🤖🤖
