package packages

// [>] 🤖🤖

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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

var errNotInstalled = &notInstalledErr{}

type notInstalledErr struct{}

func (*notInstalledErr) Error() string { return "not installed" }

func TestNoApplicableManagerStillInstallsCompletions(t *testing.T) {
	in, m := newInstaller(t, dockerAptYaml, "darwin", cmdMap([]string{"docker"}), completionsOpts(Options{}))
	home := tempHome(t, in)
	m.Stub = stubOutputs("sh ", "#compdef docker\n")
	require.NoError(t, in.Install([]string{"docker"}))
	require.FileExists(t, filepath.Join(home, ".local", "share", "zsh", "site-functions", "_docker"))
}

// [<] 🤖🤖
