package packages

// [>] 🤖🤖

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const requiresYaml = `packages:
  libffi:
    - apt:
        packageName: libffi-dev
  pyenv:
    - apt:
        packageName: pyenv
  python3:
    version: "3.14.*"
    requires: [pyenv, libffi]
    installers:
      - pyenv:
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

// [<] 🤖🤖
