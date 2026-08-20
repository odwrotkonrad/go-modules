package che

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestWithDotEnvKeepsProcessOnTop(t *testing.T) {
	dir := testutil.Tree(t, map[string]string{".env": "A=file\nexport B=file\n"})
	ctx := Context{Env: map[string]string{"B": "process"}}
	got, err := ctx.withDotEnv(filepath.Join(dir, ".env"))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"A": "file", "B": "process"}, got.Env)
	assert.Equal(t, map[string]string{"A": "file", "B": "file"}, got.DotEnv)
	assert.Equal(t, map[string]string{"B": "process"}, got.ProcessEnv)
}

func TestWithDotEnvToleratesMissingFile(t *testing.T) {
	ctx := Context{Env: map[string]string{"B": "process"}}
	got, err := ctx.withDotEnv(filepath.Join(t.TempDir(), ".env"))
	require.NoError(t, err)
	assert.Equal(t, ctx.Env, got.Env)
	assert.Nil(t, got.DotEnv)
}

// [<] 🤖🤖
