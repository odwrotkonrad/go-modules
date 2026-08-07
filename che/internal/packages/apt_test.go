package packages

// [>] 🤖🤖

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
	require.NoError(t, in.Install([]string{"docker"}))
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "apt-get install")
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
