package packages

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const pythonVmYaml = `packages:
  python3:
    - pyenv:
        versions: ["3.14.5", "3.12.9"]
        global: "3.14.5"
`

const nodeVmYaml = `packages:
  node:
    - nvm:
        versions: ["24.16.0"]
        global: "24.16.0"
`

func TestVersionManagerPyenvInstallsMissingAndSetsGlobal(t *testing.T) {
	in, m := newInstaller(t, pythonVmYaml, "linux", cmdMap([]string{"pyenv"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		if joined == "pyenv versions --bare" {
			return []byte("3.12.9\n"), nil
		}
		if joined == "pyenv global" {
			return []byte("3.12.9\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"python3"}))
	requireCalls(t, m, "pyenv install --skip-existing 3.14.5", "pyenv global 3.14.5")
	refuteCalls(t, m, "pyenv install --skip-existing 3.12.9")
}

func TestVersionManagerPyenvSkipsWhenSatisfied(t *testing.T) {
	in, m := newInstaller(t, pythonVmYaml, "linux", cmdMap([]string{"pyenv"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		if joined == "pyenv versions --bare" {
			return []byte("3.14.5\n3.12.9\n"), nil
		}
		if joined == "pyenv global" {
			return []byte("3.14.5\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"python3"}))
	refuteCalls(t, m, "pyenv install")
}

func TestVersionManagerInstallsWithoutToolPresent(t *testing.T) {
	in, m := newInstaller(t, pythonVmYaml, "linux", cmdMap(nil), Options{})
	_, err := captureStdout(t, func() error { return in.Install([]string{"python3"}) })
	require.NoError(t, err)
	requireCalls(t, m, "pyenv install")
}

func nvmHome(t *testing.T, in *Installer) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nvm.sh"), []byte(""), 0o644))
	prev := in.Host.Getenv
	in.Host.Getenv = func(k string) string {
		if k == "NVM_DIR" {
			return dir
		}
		return prev(k)
	}
	return dir
}

func TestVersionManagerNvmInstallsAndAliasesDefault(t *testing.T) {
	in, m := newInstaller(t, nodeVmYaml, "linux", cmdMap(nil), Options{})
	dir := nvmHome(t, in)
	require.NoError(t, in.Install([]string{"node"}))
	requireCalls(t, m, `. "`+dir+`/nvm.sh" && nvm install 24.16.0`, "nvm alias default 24.16.0")
}

func TestVersionManagerNvmSkipsWhenSatisfied(t *testing.T) {
	in, m := newInstaller(t, nodeVmYaml, "linux", cmdMap(nil), Options{})
	dir := nvmHome(t, in)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "versions", "node", "v24.16.0"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "alias"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alias", "default"), []byte("v24.16.0\n"), 0o644))
	require.NoError(t, in.Install([]string{"node"}))
	require.Empty(t, m.Calls())
}

func TestVersionManagerDryRunAnnounces(t *testing.T) {
	in, m := newInstaller(t, pythonVmYaml, "linux", cmdMap([]string{"pyenv"}), Options{DryRun: true})
	out, err := captureStdout(t, func() error { return in.Install([]string{"python3"}) })
	require.NoError(t, err)
	wantLines(t, out, "install python3 via pyenv (versions: 3.14.5 3.12.9, global: 3.14.5) (dry run)")
	refuteCalls(t, m, "pyenv install")
}

func TestVersionManagerUnknownToolErrors(t *testing.T) {
	const yml = `packages:
  x: [{rbenv: {versions: ["3.4.0"]}}]
`
	in, _ := newInstaller(t, yml, "linux", cmdMap(nil), Options{})
	require.ErrorContains(t, in.Install([]string{"x"}), `unknown manager for x: rbenv`)
}

func TestInstallScriptValidateArtifactSkipsWhenPresent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nvm.sh"), []byte(""), 0o644))
	yml := "packages:\n  nvm: [{script: {url: https://example.com/i.sh, validateArtifact: " + dir + "/nvm.sh}}]\n"
	in, m := newInstaller(t, yml, "linux", cmdMap(nil), Options{})
	require.NoError(t, in.Install([]string{"nvm"}))
	require.Empty(t, m.Calls())
}

func TestInstallScriptValidateArtifactRunsWhenAbsent(t *testing.T) {
	yml := "packages:\n  nvm: [{script: {url: https://example.com/i.sh, validateArtifact: " + t.TempDir() + "/nvm.sh}}]\n"
	in, m := newInstaller(t, yml, "linux", cmdMap(nil), Options{})
	m.Stub = stubOutputs("curl ", "echo hi\n")
	require.NoError(t, in.Install([]string{"nvm"}))
	requireCalls(t, m, "che-script-")
}

// [<] 🤖🤖
