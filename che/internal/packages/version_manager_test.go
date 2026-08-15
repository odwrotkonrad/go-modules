package packages

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const nodeVmYaml = `packages:
  node:
    - nvm:
        versions: ["24.16.0"]
        global: "24.16.0"
`

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
