package lib

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeMakefiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	}
	return dir
}

const shared = `#[what] render the docs
render-templates:
	@che render-templates

#[what] seed .env
repo-render-env:
	@che render-templates --profiles=envSeed
`

func TestGenerateDocumentsIncludedTargets(t *testing.T) {
	dir := writeMakefiles(t, map[string]string{
		"Makefile":         "##[>] Docs [genai-include]\ninclude shared/render.mk\n##[<] Docs\n",
		"shared/render.mk": shared,
	})

	out, err := Generate(filepath.Join(dir, "Makefile"))

	require.NoError(t, err)
	assert.Contains(t, out, "`render-templates` render the docs")
	assert.Contains(t, out, "`repo-render-env` seed .env")
	assert.NotContains(t, out, "shared/render.mk`")
}

func TestGenerateFollowsAnOptionalIncludeThatExists(t *testing.T) {
	dir := writeMakefiles(t, map[string]string{
		"Makefile":         "##[>] Docs [genai-include]\n-include shared/render.mk\n##[<] Docs\n",
		"shared/render.mk": shared,
	})

	out, err := Generate(filepath.Join(dir, "Makefile"))

	require.NoError(t, err)
	assert.Contains(t, out, "`render-templates` render the docs")
}

func TestGenerateFailsOnAnUnreadableMandatoryInclude(t *testing.T) {
	dir := writeMakefiles(t, map[string]string{
		"Makefile": "##[>] Docs [genai-include]\ninclude shared/render.mk\n##[<] Docs\n",
	})

	_, err := Generate(filepath.Join(dir, "Makefile"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "shared/render.mk")
}

// [why] a fresh clone has not rendered its shared tree yet: the bootstrap rule
//
//	builds it, so an absent optional include is the normal first state
func TestGenerateToleratesAnAbsentOptionalInclude(t *testing.T) {
	dir := writeMakefiles(t, map[string]string{
		"Makefile": "##[>] Docs [genai-include]\n-include shared/render.mk\n#[what] local target\nbuild:\n\t@true\n##[<] Docs\n",
	})

	out, err := Generate(filepath.Join(dir, "Makefile"))

	require.NoError(t, err)
	assert.Contains(t, out, "`build` local target")
}

func TestGenerateIncludesEachFileOnce(t *testing.T) {
	dir := writeMakefiles(t, map[string]string{
		"Makefile":         "##[>] Docs [genai-include]\ninclude shared/render.mk\ninclude shared/render.mk\n##[<] Docs\n",
		"shared/render.mk": shared,
	})

	out, err := Generate(filepath.Join(dir, "Makefile"))

	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(out, "`render-templates`"))
}

// [<] 🤖🤖
