package che

// [>] 🤖🤖

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

func TestExportEnvRestoresPriorState(t *testing.T) {
	t.Setenv("CHE_TEST_KEPT", "before")
	os.Unsetenv("CHE_TEST_FRESH")
	restore := exportEnv(map[string]string{"CHE_TEST_KEPT": "during", "CHE_TEST_FRESH": "during"})
	assert.Equal(t, "during", os.Getenv("CHE_TEST_KEPT"))
	assert.Equal(t, "during", os.Getenv("CHE_TEST_FRESH"))
	restore()
	assert.Equal(t, "before", os.Getenv("CHE_TEST_KEPT"))
	_, set := os.LookupEnv("CHE_TEST_FRESH")
	assert.False(t, set)
}

func TestWithRepoFilesReadsBothFilesBeneathProcess(t *testing.T) {
	dir := testutil.Tree(t, map[string]string{
		".che/che.env":          "A=file\nexport B=file\nC=${{ env.B }}-${{ env.D || d }}\n",
		".che/cheVariables.yml": "REF: v1\nFROM_ENV: '${{ env.B }}'\nSCOPED: {value: s, scope: recursiveSpecs}\n",
	})
	ctx := Context{Env: map[string]string{"B": "process"}}
	got, err := ctx.withRepoFiles(dir)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"A": "file", "B": "process", "C": "process-d"}, got.Env)
	assert.Equal(t, map[string]string{"A": "file", "B": "file", "C": "process-d"}, got.RepoFiles.Env)
	assert.Equal(t, map[string]string{"REF": "v1", "FROM_ENV": "process", "SCOPED": "s"}, got.RepoFiles.Shared.Values())
	assert.Equal(t, spec.VarScopes.RecursiveSpecs, got.RepoFiles.Shared["SCOPED"].Scope)
	assert.Equal(t, map[string]string{"B": "process"}, got.ProcessEnv)
}

func TestWithRepoFilesRejectsVarRefsInRepoFiles(t *testing.T) {
	dir := testutil.Tree(t, map[string]string{".che/che.env": "PIN=${{ var.REF }}\n", ".che/cheVariables.yml": "REF: v1\n"})
	_, err := ctx0().withRepoFiles(dir)
	require.ErrorContains(t, err, "not available in repo files")
}

func TestWithRepoFilesRejectsLegacyVariablesFile(t *testing.T) {
	dir := testutil.Tree(t, map[string]string{".che/che.variables.yml": "REF: v1\n"})
	_, err := ctx0().withRepoFiles(dir)
	require.ErrorContains(t, err, "che.variables.yml is no longer read, rename to cheVariables.yml")
}

func TestWithRepoFilesPrefersRootOverDotChe(t *testing.T) {
	dir := testutil.Tree(t, map[string]string{"che.env": "A=root\n", ".che/che.env": "A=nested\n"})
	got, err := ctx0().withRepoFiles(dir)
	require.NoError(t, err)
	assert.Equal(t, "root", got.Env["A"])
}

func TestWithRepoFilesToleratesMissingFiles(t *testing.T) {
	ctx := Context{Env: map[string]string{"B": "process"}}
	got, err := ctx.withRepoFiles(t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, ctx.Env, got.Env)
	assert.Nil(t, got.RepoFiles.Env)
}

func TestWithRepoFilesIgnoresDotEnv(t *testing.T) {
	dir := testutil.Tree(t, map[string]string{".env": "A=dotenv\n"})
	got, err := ctx0().withRepoFiles(dir)
	require.NoError(t, err)
	assert.Empty(t, got.Env["A"])
}

func ctx0() Context { return Context{Env: map[string]string{}} }

// [<] 🤖🤖
