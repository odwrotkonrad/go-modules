package packages

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
)

func newInstaller(t *testing.T, filesYaml, osname string, cmds map[string]string, opts Options) (*Installer, *execx.Mock) {
	t.Helper()
	var f File
	require.NoError(t, yaml.Unmarshal([]byte(filesYaml), &f))
	m := &execx.Mock{}
	execx.Swap(t, m)
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

func TestInstallBrewTapQualified(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  che: [{brew: konradodwrot/tap/che}]", "darwin", cmdMap([]string{"brew"}), Options{})
	m.Stub = failOn("brew list")
	require.NoError(t, in.Install([]string{"che"}))
	require.Equal(t, []string{
		"brew list che",
		"brew tap konradodwrot/tap",
		"brew trust --formula konradodwrot/tap/che",
		"brew install konradodwrot/tap/che",
	}, m.Calls())
}

func TestInstallAptSudoAndUpdateOnce(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  fd: [brew, {apt: fd-find}]\n  jq: [apt]", "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = failOn("dpkg -s")
	require.NoError(t, in.Install([]string{"fd", "jq"}))
	calls := m.Calls()
	require.Contains(t, calls, "sudo apt-get update")
	require.Contains(t, calls, "sudo apt-get install --yes --no-install-recommends fd-find")
	require.Contains(t, calls, "sudo apt-get install --yes --no-install-recommends jq")
	require.Equal(t, 1, strings.Count(strings.Join(calls, "\n"), "apt-get update"))
}

func TestInstallNpmPinReinstallsOnDrift(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  ccstatusline: [{npm: ccstatusline@2.2.22}]", "darwin",
		cmdMap([]string{"npm", "ccstatusline"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "npm" && argv[1] == "ls" {
			return []byte("ccstatusline@2.2.0\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"ccstatusline"}))
	require.Contains(t, m.Calls(), "npm install --global ccstatusline@2.2.22")
}

func TestInstallNpmPinSkipsWhenMatching(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  ccstatusline: [{npm: ccstatusline@2.2.22}]", "darwin",
		cmdMap([]string{"npm", "ccstatusline"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "npm" && argv[1] == "ls" {
			return []byte("ccstatusline@2.2.22\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"ccstatusline"}))
	require.NotContains(t, m.Calls(), "npm install --global ccstatusline@2.2.22")
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
	in, m := newInstaller(t, "packages:\n  x: [{npm: x}]\n  npm: [{apt: npm}]", "linux", cmds, Options{})
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

func TestInstallSkipsNoApplicableManager(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  bat: [brew]", "linux", cmdMap(nil), Options{})
	out, err := captureStdout(t, func() error { return in.Install([]string{"bat"}) })
	require.NoError(t, err)
	wantLines(t, out, "will not install bat: no applicable manager")
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
	out, err := captureStdout(t, func() error { return in.Install([]string{"brew"}) })
	require.NoError(t, err)
	wantLines(t, out, "will not install brew: no applicable manager")
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

func TestInstallScriptPathFallsBackToShipped(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  brew: [{script: {path: scripts/install-brew.sh}}]", "darwin", cmdMap(nil), Options{})
	in.FilePath = BuiltinPath
	require.NoError(t, in.Install([]string{"brew"}))
	calls := m.Calls()
	require.Len(t, calls, 1)
	require.Contains(t, calls[0], "/bin/sh -ec")
	require.Contains(t, calls[0], "NONINTERACTIVE=1")
}

func TestInstallScriptPathMissingErrors(t *testing.T) {
	in, _ := newInstaller(t, "packages:\n  x: [{script: {path: scripts/nope.sh}}]", "darwin", cmdMap(nil), Options{})
	in.FilePath = BuiltinPath
	err := in.Install([]string{"x"})
	require.ErrorContains(t, err, "install script not found: scripts/nope.sh")
}

func TestInstallScriptUrlFetchesAndRuns(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  x: [{script: {remoteUrl: https://example.com/install.sh}}]", "darwin", cmdMap(nil), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "curl" {
			return []byte("echo fetched"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"x"}))
	calls := m.Calls()
	require.Len(t, calls, 2)
	require.Contains(t, calls[0], "curl -fsSL")
	require.Equal(t, "/bin/sh -ec echo fetched", calls[1])
}

func TestInstallScriptUrlFetchFailureAborts(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  x: [{script: {remoteUrl: https://example.com/install.sh}}]", "darwin", cmdMap(nil), Options{})
	m.Stub = failOn("curl")
	err := in.Install([]string{"x"})
	require.ErrorContains(t, err, "install script fetch failed: https://example.com/install.sh")
}

const gcloudYaml = `packages:
  gcloud:
    - script:
        os: linux
        path: scripts/install-gcloud-apt.sh
    - script:
        os: darwin
        version: 572.0.0
        path: scripts/install-gcloud.sh
        sha256:
          darwin-arm64: gsha
`

func TestInstallScriptPinReinstallsOnDrift(t *testing.T) {
	in, m := newInstaller(t, gcloudYaml, "darwin", cmdMap([]string{"gcloud"}), Options{})
	in.Host.Arch, in.Host.ArchX, in.Host.ArchG = "arm64", "aarch64", "arm64"
	in.FilePath = BuiltinPath
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "gcloud" {
			return []byte("Google Cloud SDK 570.0.0\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"gcloud"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "/bin/sh -ec")
}

func TestInstallScriptPinSkipsWhenMatching(t *testing.T) {
	in, m := newInstaller(t, gcloudYaml, "darwin", cmdMap([]string{"gcloud"}), Options{})
	in.Host.Arch = "arm64"
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "gcloud" {
			return []byte("Google Cloud SDK 572.0.0\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"gcloud"}))
	require.Equal(t, []string{"gcloud --version"}, m.Calls())
}

func TestInstallScriptShaGatesApplicability(t *testing.T) {
	in, m := newInstaller(t, gcloudYaml, "darwin", cmdMap(nil), Options{})
	in.Host.Arch = "amd64"
	out, err := captureStdout(t, func() error { return in.Install([]string{"gcloud"}) })
	require.NoError(t, err)
	wantLines(t, out, "will not install gcloud: no applicable manager")
	require.Empty(t, m.Calls())
}

func TestInstallScriptPicksLinuxAptScript(t *testing.T) {
	in, m := newInstaller(t, gcloudYaml, "linux", cmdMap(nil), Options{})
	in.FilePath = BuiltinPath
	require.NoError(t, in.Install([]string{"gcloud"}))
	calls := m.Calls()
	require.Len(t, calls, 1)
	require.Contains(t, calls[0], "google-cloud-cli")
}

func TestScriptEnvCarriesPinShaAndHost(t *testing.T) {
	in, _ := newInstaller(t, gcloudYaml, "darwin", cmdMap(nil), Options{})
	in.Host.Arch = "arm64"
	env := in.scriptEnv("gcloud", &ScriptSpec{Version: "572.0.0", Sha256: map[string]string{"darwin-arm64": "gsha"}})
	joined := strings.Join(env, "\n")
	for _, want := range []string{
		"CHE_PKG_NAME=gcloud", "CHE_PKG_VERSION=572.0.0", "CHE_PKG_SHA256=gsha",
		"CHE_PKG_OS=darwin", "CHE_PKG_ARCH=arm64",
	} {
		require.Contains(t, joined, want)
	}
}

func TestCheckUpgradableScriptPinDrift(t *testing.T) {
	in, m := newInstaller(t, gcloudYaml, "darwin", cmdMap([]string{"gcloud"}), Options{})
	in.Host.Arch = "arm64"
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "gcloud" {
			return []byte("Google Cloud SDK 570.0.0\n"), nil
		}
		return nil, nil
	}
	out, err := captureStdout(t, func() error { return in.CheckUpgradable([]string{"gcloud"}) })
	require.NoError(t, err)
	wantLines(t, out, "upgradable gcloud via script: yaml pins 572.0.0")
}

func TestInstallPythonPyenvPinFromBuiltin(t *testing.T) {
	f, err := LoadBuiltin()
	require.NoError(t, err)
	m := &execx.Mock{}
	execx.Swap(t, m)
	in := &Installer{File: f, FilePath: BuiltinPath, Host: testHost("darwin", "arm64", cmdMap([]string{"python3", "pyenv"})), Opts: Options{}}
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "python3" {
			return []byte("Python 3.9.6\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"python3"}))
	calls := m.Calls()
	require.Len(t, calls, 3)
	require.Contains(t, calls[2], "/bin/sh -ec")
	require.Contains(t, calls[2], "pyenv install -s")

	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "python3" {
			return []byte("Python 3.14.5\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"python3"}))
	require.Len(t, m.Calls(), 4)
}

const codeExtYaml = "packages:\n  golang.go: [code]\n  redhat.vscode-yaml: [code]\n  code: [{brew: {cask: visual-studio-code}}]"

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
	require.Equal(t, []string{"code --list-extensions"}, m.Calls())
}

func TestInstallCodeExtensionSkipsWithoutCodeCommand(t *testing.T) {
	in, m := newInstaller(t, codeExtYaml, "linux", cmdMap(nil), Options{})
	out, err := captureStdout(t, func() error { return in.Install([]string{"golang.go"}) })
	require.NoError(t, err)
	wantLines(t, out, "will not install golang.go: no applicable manager")
	require.Empty(t, m.Calls())
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
	require.ErrorContains(t, err, "unknown package: nope (add it to packages.yml)")
}

// [<] 🤖🤖
