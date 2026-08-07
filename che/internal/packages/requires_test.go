package packages

// [>] 🤖🤖

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const requiresYaml = `packages:
  libffi:
    - apt: libffi-dev
  pyenv:
    - apt: pyenv
  python3:
    version: "3.14.*"
    requires: [pyenv, libffi]
    installMethods:
      - versionManager:
          tool: pyenv
          versions: ["3.14.5"]
          global: "3.14.5"
`

func TestRequiresInstallBeforeDependent(t *testing.T) {
	cmds := cmdMap([]string{"apt-get"})
	in, m := newInstaller(t, requiresYaml, "linux", cmds, Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		if strings.HasPrefix(joined, "dpkg -s") {
			return nil, errNotInstalled
		}
		if joined == "sudo apt-get install --yes --no-install-recommends pyenv" {
			cmds["pyenv"] = "/usr/bin/pyenv"
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"python3"}))
	calls := strings.Join(m.Calls(), "\n")
	pyenvAt := strings.Index(calls, "--no-install-recommends pyenv")
	libffiAt := strings.Index(calls, "--no-install-recommends libffi-dev")
	installAt := strings.Index(calls, "pyenv install --skip-existing")
	require.Positive(t, pyenvAt)
	require.Positive(t, libffiAt)
	require.Positive(t, installAt)
	require.Less(t, pyenvAt, installAt)
	require.Less(t, libffiAt, installAt)
}

func TestRequiresCycleErrors(t *testing.T) {
	const yml = `packages:
  a:
    requires: [b]
    installMethods: [brew]
  b:
    requires: [a]
    installMethods: [brew]
`
	in, _ := newInstaller(t, yml, "darwin", cmdMap([]string{"brew"}), Options{})
	require.ErrorContains(t, in.Install([]string{"a"}), "requires cycle")
}

func TestRequiresUnknownPackageErrors(t *testing.T) {
	const yml = `packages:
  a:
    requires: [nope]
    installMethods: [brew]
`
	in, _ := newInstaller(t, yml, "darwin", cmdMap([]string{"brew"}), Options{})
	require.ErrorContains(t, in.Install([]string{"a"}), "unknown package: nope")
}

func TestRequestedVersionDrivesVersionManager(t *testing.T) {
	in, m := newInstaller(t, requiresYaml, "linux", cmdMap([]string{"apt-get", "pyenv"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(argv, " "), "dpkg -s") {
			return nil, errNotInstalled
		}
		return nil, nil
	}
	require.NoError(t, in.InstallRequests([]Request{{Name: "python3", Versions: []string{"3.14.2"}}}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "pyenv install --skip-existing 3.14.2")
	require.Contains(t, calls, "pyenv global 3.14.2")
	require.NotContains(t, calls, "3.14.5")
}

func TestRequestedVersionOverridesEntryDefault(t *testing.T) {
	in, m := newInstaller(t, requiresYaml, "linux", cmdMap([]string{"apt-get", "pyenv"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(argv, " "), "dpkg -s") {
			return nil, errNotInstalled
		}
		return nil, nil
	}
	require.NoError(t, in.InstallRequests([]Request{{Name: "python3", Versions: []string{"3.13.1"}}}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "pyenv install --skip-existing 3.13.1")
}

func TestRequestedMultipleVersionsWithGlobal(t *testing.T) {
	in, m := newInstaller(t, requiresYaml, "linux", cmdMap([]string{"apt-get", "pyenv"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(argv, " "), "dpkg -s") {
			return nil, errNotInstalled
		}
		return nil, nil
	}
	require.NoError(t, in.InstallRequests([]Request{
		{Name: "python3", Versions: []string{"3.14.5", "3.13.1"}, Global: "3.13.1"},
	}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "pyenv install --skip-existing 3.14.5")
	require.Contains(t, calls, "pyenv install --skip-existing 3.13.1")
	require.Contains(t, calls, "pyenv global 3.13.1")
}

func TestRequestedGlobalDefaultsToFirstVersion(t *testing.T) {
	in, m := newInstaller(t, requiresYaml, "linux", cmdMap([]string{"apt-get", "pyenv"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(argv, " "), "dpkg -s") {
			return nil, errNotInstalled
		}
		return nil, nil
	}
	require.NoError(t, in.InstallRequests([]Request{
		{Name: "python3", Versions: []string{"3.14.5", "3.13.1"}},
	}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "pyenv global 3.14.5")
}

func TestRequestedMultipleVersionsNeedVersionManager(t *testing.T) {
	const yml = `packages:
  kind:
    installMethods:
      - prebuiltArchive:
          version: 0.32.0
          url: https://example.com/kind-{version}
          sha256: {linux-amd64: goodsha}
`
	in, _ := newInstaller(t, yml, "linux", cmdMap([]string{"sha256sum"}), Options{})
	err := in.InstallRequests([]Request{{Name: "kind", Versions: []string{"0.32.0", "0.31.0"}}})
	require.ErrorContains(t, err, "multiple versions need a version-manager installation method")
}

func TestRequestedVersionRejectedWhenPinnedAssetDiffers(t *testing.T) {
	const yml = `packages:
  kind:
    version: "0.32.*"
    installMethods:
      - prebuiltArchive:
          version: 0.32.0
          url: https://example.com/kind-{version}
          sha256: {linux-amd64: goodsha}
`
	in, _ := newInstaller(t, yml, "linux", cmdMap([]string{"sha256sum"}), Options{})
	err := in.InstallRequests([]Request{{Name: "kind", Versions: []string{"0.32.1"}}})
	require.ErrorContains(t, err, "no checksum for the requested version")
}

// [<] 🤖🤖
