package packages

// [>] 🤖🤖

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const dockerAptYaml = `packages:
  docker:
    installMethods:
      - apt:
          installPackages: [docker-ce, docker-ce-cli, containerd.io]
          fromSource:
            url: https://download.docker.com/linux/debian
            verificationKey: https://download.docker.com/linux/debian/gpg
    completions:
      zsh: {cmd: docker completion zsh}
  docker-desktop:
    command: docker
    installMethods:
      - script:
          os: darwin
          run: echo install-desktop
`

func aptStub(t *testing.T) func(argv []string) ([]byte, error) {
	t.Helper()
	return func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		switch {
		case strings.HasPrefix(joined, "dpkg -s"):
			return nil, errNotInstalled
		case strings.HasPrefix(joined, "dpkg --print-architecture"):
			return []byte("amd64\n"), nil
		case strings.HasPrefix(joined, "sh -ec . /etc/os-release"):
			return []byte("bookworm\n"), nil
		}
		return nil, nil
	}
}

var errNotInstalled = &notInstalledErr{}

type notInstalledErr struct{}

func (*notInstalledErr) Error() string { return "not installed" }

func TestInstallAptRepoConfiguresAndInstalls(t *testing.T) {
	in, m := newInstaller(t, dockerAptYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = aptStub(t)
	require.NoError(t, in.Install([]string{"docker"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "https://download.docker.com/linux/debian/gpg")
	require.Contains(t, calls, "sudo install -m 0755 -d /etc/apt/keyrings")
	require.Contains(t, calls, "/etc/apt/keyrings/docker.asc")
	require.Contains(t, calls, "/etc/apt/sources.list.d/docker.sources")
	require.Contains(t, calls, "sudo apt-get update")
	require.Contains(t, calls, "sudo apt-get install --yes --no-install-recommends docker-ce docker-ce-cli containerd.io")
}

func TestInstallAptRepoSkipsWhenAllInstalled(t *testing.T) {
	in, m := newInstaller(t, dockerAptYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		switch {
		case strings.HasPrefix(joined, "dpkg --print-architecture"):
			return []byte("amd64\n"), nil
		case strings.HasPrefix(joined, "sh -ec . /etc/os-release"):
			return []byte("bookworm\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"docker"}))
	calls := strings.Join(m.Calls(), "\n")
	require.NotContains(t, calls, "apt-get install")
	require.Contains(t, calls, "/etc/apt/sources.list.d/docker.sources")
}

func TestInstallAptRepoDryRunAnnounces(t *testing.T) {
	in, m := newInstaller(t, dockerAptYaml, "linux", cmdMap([]string{"apt-get"}), Options{DryRun: true})
	m.Stub = aptStub(t)
	out, err := captureStdout(t, func() error { return in.Install([]string{"docker"}) })
	require.NoError(t, err)
	wantLines(t, out, "install docker via apt (dry run)")
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "curl")
}

func TestAptPrerequisitesSkippedWhenInstalled(t *testing.T) {
	const yml = `packages:
  x:
    installMethods:
      - apt:
          installPackages: [x-cli]
          prerequisitePackages: [gnupg]
`
	in, m := newInstaller(t, yml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(argv, " "), "dpkg -s x-cli") {
			return nil, errNotInstalled
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"x"}))
	calls := strings.Join(m.Calls(), "\n")
	require.NotContains(t, calls, "--no-install-recommends gnupg")
	require.Contains(t, calls, "--no-install-recommends x-cli")
}

func TestAptRepoRequiresUrlAndGpg(t *testing.T) {
	const yml = `packages:
  x:
    installMethods:
      - apt:
          fromSource: {url: https://example.com}
`
	in, _ := newInstaller(t, yml, "linux", cmdMap([]string{"apt-get"}), Options{})
	require.ErrorContains(t, in.Install([]string{"x"}), "apt fromSource requires url and verificationKey")
}

const pinnedCurlYaml = `packages:
  curl:
    version: 8.21.0
    installMethods:
      - apt:
          versions: {"8.14.1": "8.14.1-2+deb13u2~bpo13+1"}
`

func TestInstallAptPinsMappedPackageVersion(t *testing.T) {
	in, m := newInstaller(t, pinnedCurlYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = aptStub(t)
	require.NoError(t, in.Install([]string{"curl"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"),
		"sudo apt-get install --yes --no-install-recommends --allow-downgrades curl=8.14.1-2+deb13u2~bpo13+1")
}

func TestInstallAptReinstallsOnPinMismatch(t *testing.T) {
	in, m := newInstaller(t, pinnedCurlYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(argv, " "), "dpkg-query") {
			return []byte("7.88.1-10+deb12u15"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"curl"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "--allow-downgrades curl=8.14.1-2+deb13u2~bpo13+1")
}

func TestInstallAptSkipsWhenPinInstalled(t *testing.T) {
	in, m := newInstaller(t, pinnedCurlYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(argv, " "), "dpkg-query") {
			return []byte("8.14.1-2+deb13u2~bpo13+1"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"curl"}))
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "apt-get install")
}

const backportsYaml = `packages:
  debian-backports:
    installMethods:
      - apt:
          installPackages: [debian-archive-keyring]
          fromSource:
            url: https://deb.debian.org/debian
            verificationKey: /usr/share/keyrings/debian-archive-keyring.gpg
            suites: bookworm-backports
            components: main
`

func TestInstallAptPathKeySkipsKeyDownload(t *testing.T) {
	in, m := newInstaller(t, backportsYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	require.NoError(t, in.Install([]string{"debian-backports"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "/etc/apt/sources.list.d/debian-backports.sources")
	require.NotContains(t, calls, "curl")
	require.NotContains(t, calls, "/etc/apt/keyrings")
	require.NotContains(t, calls, "apt-get install")
}

func TestAptVersionsMapValidation(t *testing.T) {
	err := yaml.Unmarshal([]byte(`packages:
  x:
    installMethods:
      - apt:
          versions: {"1.0": "1.0-1", "2.0": "2.0-1"}
`), &File{})
	require.ErrorContains(t, err, "exactly one binary version")
	err = yaml.Unmarshal([]byte(`packages:
  x:
    installMethods:
      - apt:
          installPackages: [a, b]
          versions: {"1.0": "1.0-1"}
`), &File{})
	require.ErrorContains(t, err, "single installPackages entry")
}

func TestCommandOverrideSkipsScriptWhenPresent(t *testing.T) {
	in, m := newInstaller(t, dockerAptYaml, "darwin", cmdMap([]string{"docker"}), Options{})
	require.NoError(t, in.Install([]string{"docker-desktop"}))
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "install-desktop")
}

func TestCommandOverrideRunsScriptWhenAbsent(t *testing.T) {
	in, m := newInstaller(t, dockerAptYaml, "darwin", cmdMap(nil), Options{})
	require.NoError(t, in.Install([]string{"docker-desktop"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "install-desktop")
}

func TestNoApplicableManagerStillInstallsCompletions(t *testing.T) {
	in, m := newInstaller(t, dockerAptYaml, "darwin", cmdMap([]string{"docker"}), completionsOpts(Options{}))
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "sh" {
			return []byte("#compdef docker\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"docker"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "/home/u/.local/share/zsh/site-functions/_docker")
}

// [<] 🤖🤖
