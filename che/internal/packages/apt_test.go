package packages

// [>] 🤖🤖

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const dockerAptYaml = `installerRegistries:
  apt:
    - url: https://download.docker.com/linux/debian
      verificationKey: https://download.docker.com/linux/debian/gpg
packages:
  docker:
    installers:
      - apt:
          packageName: docker-ce
          fromRegistry: download.docker.com/linux/debian
    completions:
      zsh: {cmd: docker completion zsh}
  docker-desktop:
    command: docker
    installers:
      - script:
          os: darwin
          run: echo install-desktop
`

func aptStub(t *testing.T) func(argv []string) ([]byte, error) {
	t.Helper()
	return chainStubs(
		failOn("dpkg -s"),
		stubOutputs("dpkg --print-architecture", "amd64\n"),
	)
}

func stubOsRelease(in *Installer) {
	in.Host.ReadFile = func(path string) ([]byte, error) {
		if path == "/etc/os-release" {
			return []byte("ID=debian\nVERSION_CODENAME=\"bookworm\"\n"), nil
		}
		return nil, fmt.Errorf("read %s: not stubbed", path)
	}
}

var errNotInstalled = &notInstalledErr{}

type notInstalledErr struct{}

func (*notInstalledErr) Error() string { return "not installed" }

func TestInstallAptRepoConfiguresAndInstalls(t *testing.T) {
	in, m := newInstaller(t, dockerAptYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = aptStub(t)
	stubOsRelease(in)
	require.NoError(t, in.Install([]string{"docker"}))
	require.Contains(t, testFetch.Calls(), "https://download.docker.com/linux/debian/gpg")
	requireCalls(t, m,
		"sudo install -m 0755 -d /etc/apt/keyrings",
		"/etc/apt/keyrings/download.docker.com-linux-debian.asc",
		"/etc/apt/sources.list.d/download.docker.com-linux-debian.sources",
		"sudo apt-get update",
		"sudo apt-get install --yes --no-install-recommends docker-ce")
}

func TestInstallAptRepoSkipsWhenAllInstalled(t *testing.T) {
	in, m := newInstaller(t, dockerAptYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = stubOutputs("dpkg --print-architecture", "amd64\n")
	stubOsRelease(in)
	require.NoError(t, in.Install([]string{"docker"}))
	refuteCalls(t, m, "apt-get install")
	requireCalls(t, m, "/etc/apt/sources.list.d/download.docker.com-linux-debian.sources")
}

func TestInstallAptRepoDryRunAnnounces(t *testing.T) {
	in, m := newInstaller(t, dockerAptYaml, "linux", cmdMap([]string{"apt-get"}), Options{DryRun: true})
	m.Stub = aptStub(t)
	out, err := captureStdout(t, func() error { return in.Install([]string{"docker"}) })
	require.NoError(t, err)
	wantLines(t, out, "install docker via apt (dry run)")
	refuteCalls(t, m, "curl")
}

func TestAptPrerequisitesSkippedWhenInstalled(t *testing.T) {
	const yml = `packages:
  x:
    installers:
      - apt:
          packageName: x-cli
          prerequisitePackages: [gnupg]
`
	in, m := newInstaller(t, yml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = failOn("dpkg -s x-cli")
	require.NoError(t, in.Install([]string{"x"}))
	refuteCalls(t, m, "--no-install-recommends gnupg")
	requireCalls(t, m, "--no-install-recommends x-cli")
}

func TestAptRepoRequiresUrlAndGpg(t *testing.T) {
	const yml = `installerRegistries:
  apt:
    - url: https://example.com
packages:
  x:
    installers:
      - apt: {fromRegistry: example.com}
`
	in, m := newInstaller(t, yml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = aptStub(t)
	require.ErrorContains(t, in.Install([]string{"x"}), `apt registry "example.com" requires url and verificationKey`)
}

const pinnedCurlYaml = `installerRegistries:
  apt:
    - url: https://deb.debian.org/debian
      verificationKey: /usr/share/keyrings/debian-archive-keyring.gpg
      suites: bookworm-backports
      components: main
packages:
  curl:
    version: 8.21.0
    installers:
      - apt:
          versionMap: {"8.14.1": "8.14.1-2+deb13u2~bpo13+1"}
          fromRegistry: deb.debian.org/debian
`

func TestInstallAptPinsMappedPackageVersion(t *testing.T) {
	in, m := newInstaller(t, pinnedCurlYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = aptStub(t)
	require.NoError(t, in.Install([]string{"curl"}))
	requireCalls(t, m,
		"sudo apt-get install --yes --no-install-recommends -t bookworm-backports --allow-downgrades curl=8.14.1-2+deb13u2~bpo13+1")
}

func TestInstallAptReinstallsOnPinMismatch(t *testing.T) {
	in, m := newInstaller(t, pinnedCurlYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = stubOutputs("dpkg-query", "7.88.1-10+deb12u15")
	require.NoError(t, in.Install([]string{"curl"}))
	requireCalls(t, m, "--allow-downgrades curl=8.14.1-2+deb13u2~bpo13+1")
}

func TestInstallAptSkipsWhenPinInstalled(t *testing.T) {
	in, m := newInstaller(t, pinnedCurlYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = stubOutputs("dpkg-query", "8.14.1-2+deb13u2~bpo13+1")
	require.NoError(t, in.Install([]string{"curl"}))
	refuteCalls(t, m, "apt-get install")
}

func TestInstallAptPathKeySkipsKeyDownload(t *testing.T) {
	in, m := newInstaller(t, pinnedCurlYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = stubOutputs("dpkg-query", "8.14.1-2+deb13u2~bpo13+1")
	require.NoError(t, in.Install([]string{"curl"}))
	requireCalls(t, m, "/etc/apt/sources.list.d/deb.debian.org-debian-bookworm-backports-main.sources")
	refuteCalls(t, m, "curl -fsSL", "/etc/apt/keyrings", "apt-get install")
}

func TestInstallAptUnknownRegistryErrors(t *testing.T) {
	const yml = `packages:
  x:
    installers:
      - apt: {fromRegistry: nowhere.example.com}
`
	in, m := newInstaller(t, yml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = aptStub(t)
	require.ErrorContains(t, in.Install([]string{"x"}), `unknown apt registry "nowhere.example.com"`)
}

func TestAptAmbiguousRegistryNarrowsWithSuites(t *testing.T) {
	const yml = `installerRegistries:
  apt:
    - url: https://deb.debian.org/debian
      verificationKey: /usr/share/keyrings/debian-archive-keyring.gpg
      suites: bookworm-backports
      components: main
    - url: https://deb.debian.org/debian
      verificationKey: /usr/share/keyrings/debian-archive-keyring.gpg
      suites: bookworm
      components: main
packages:
  x:
    installers:
      - apt: {fromRegistry: deb.debian.org/debian}
  y:
    installers:
      - apt: {fromRegistry: deb.debian.org/debian::bookworm-backports}
`
	in, m := newInstaller(t, yml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = aptStub(t)
	require.ErrorContains(t, in.Install([]string{"x"}), "ambiguous apt registry")
	require.NoError(t, in.Install([]string{"y"}))
	requireCalls(t, m, "-t bookworm-backports")
}

func TestAptVersionsMapValidation(t *testing.T) {
	err := yaml.Unmarshal([]byte(`packages:
  x:
    installers:
      - apt:
          versionMap: {"1.0": "1.0-1", "2.0": "2.0-1"}
`), &File{})
	require.ErrorContains(t, err, "exactly one binary version")
	err = yaml.Unmarshal([]byte(`packages:
  x:
    installers:
      - apt:
          installPackages: [a, b]
`), &File{})
	require.ErrorContains(t, err, "installPackages is gone")
}

func TestCommandOverrideSkipsScriptWhenPresent(t *testing.T) {
	in, m := newInstaller(t, dockerAptYaml, "darwin", cmdMap([]string{"docker"}), Options{})
	require.NoError(t, in.Install([]string{"docker-desktop"}))
	refuteCalls(t, m, "install-desktop")
}

func TestCommandOverrideRunsScriptWhenAbsent(t *testing.T) {
	in, m := newInstaller(t, dockerAptYaml, "darwin", cmdMap(nil), Options{})
	require.NoError(t, in.Install([]string{"docker-desktop"}))
	requireCalls(t, m, "install-desktop")
}

func TestNoApplicableManagerStillInstallsCompletions(t *testing.T) {
	in, m := newInstaller(t, dockerAptYaml, "darwin", cmdMap([]string{"docker"}), completionsOpts(Options{}))
	home := tempHome(t, in)
	m.Stub = stubOutputs("sh ", "#compdef docker\n")
	require.NoError(t, in.Install([]string{"docker"}))
	require.FileExists(t, filepath.Join(home, ".local", "share", "zsh", "site-functions", "_docker"))
}

// [<] 🤖🤖
