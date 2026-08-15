package packages

// [>] 🤖🤖

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/fetchx"
)

func makeDefinitionsTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}))
		_, err := tw.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func mockDefinitionsRegistry(t *testing.T, base, version string, archive []byte) {
	t.Helper()
	sum := sha256.Sum256(archive)
	name := "che-packages_" + version + ".tar.gz"
	fetchx.Swap(t, &fetchx.Mock{Bodies: map[string][]byte{
		base + "/latest/version.txt":            []byte(version + "\n"),
		base + "/" + version + "/" + name:       archive,
		base + "/" + version + "/checksums.txt": []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"),
	}})
}

func TestUpdateDefinitionsFetchesVerifiesAndActivates(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "packages")
	archive := makeDefinitionsTarGz(t, map[string]string{
		"packages.yml":            "packages: {}\n",
		"scripts/post-install.sh": "echo ok\n",
	})
	mockDefinitionsRegistry(t, "http://stub", "0.1.0", archive)

	res, err := UpdateDefinitions(cacheDir, "http://stub", DefaultUpdateCooldown, false)
	require.NoError(t, err)
	assert.Equal(t, UpdateResult{Version: "0.1.0", Updated: true}, res)
	dir, version, ok := ResolveCurrentDefinitions(cacheDir)
	require.True(t, ok)
	assert.Equal(t, "0.1.0", version)
	assert.FileExists(t, filepath.Join(dir, "packages.yml"))
	assert.FileExists(t, filepath.Join(dir, "scripts", "post-install.sh"))
	assert.FileExists(t, filepath.Join(cacheDir, "last-check"))
}

func TestUpdateDefinitionsSkipsWithinCooldown(t *testing.T) {
	cacheDir := t.TempDir()
	m := &fetchx.Mock{}
	fetchx.Swap(t, m)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "last-check"), nil, 0o644))

	res, err := UpdateDefinitions(cacheDir, "http://stub", time.Hour, false)
	require.NoError(t, err)
	assert.Equal(t, "cooldown", res.Skipped)
	assert.Empty(t, m.Calls())
}

func TestUpdateDefinitionsForceIgnoresCooldown(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "packages")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "last-check"), nil, 0o644))
	archive := makeDefinitionsTarGz(t, map[string]string{"packages.yml": "packages: {}\n"})
	mockDefinitionsRegistry(t, "http://stub", "0.1.1", archive)

	res, err := UpdateDefinitions(cacheDir, "http://stub", time.Hour, true)
	require.NoError(t, err)
	assert.True(t, res.Updated)
	assert.Equal(t, "0.1.1", res.Version)
}

func TestUpdateDefinitionsReportsUpToDatePresentVersion(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "packages")
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, "0.1.0"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "0.1.0", "packages.yml"), []byte("packages: {}\n"), 0o644))
	m := &fetchx.Mock{Bodies: map[string][]byte{"http://stub/latest/version.txt": []byte("0.1.0")}}
	fetchx.Swap(t, m)

	res, err := UpdateDefinitions(cacheDir, "http://stub", 0, false)
	require.NoError(t, err)
	assert.Equal(t, UpdateResult{Version: "0.1.0", Skipped: "up-to-date"}, res)
	_, version, ok := ResolveCurrentDefinitions(cacheDir)
	require.True(t, ok)
	assert.Equal(t, "0.1.0", version)
}

func TestUpdateDefinitionsRejectsChecksumMismatch(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "packages")
	archive := makeDefinitionsTarGz(t, map[string]string{"packages.yml": "packages: {}\n"})
	name := "che-packages_0.2.0.tar.gz"
	m := &fetchx.Mock{Bodies: map[string][]byte{
		"http://stub/latest/version.txt":  []byte("0.2.0"),
		"http://stub/0.2.0/" + name:       archive,
		"http://stub/0.2.0/checksums.txt": []byte("deadbeef  " + name + "\n"),
	}}
	fetchx.Swap(t, m)

	_, err := UpdateDefinitions(cacheDir, "http://stub", 0, false)
	require.ErrorContains(t, err, "checksum mismatch")
	assert.NoDirExists(t, filepath.Join(cacheDir, "0.2.0"))
}

func TestUpdateDefinitionsPrunesOlderVersions(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "packages")
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, "0.0.9"), 0o755))
	archive := makeDefinitionsTarGz(t, map[string]string{"packages.yml": "packages: {}\n"})
	mockDefinitionsRegistry(t, "http://stub", "0.1.0", archive)

	_, err := UpdateDefinitions(cacheDir, "http://stub", 0, false)
	require.NoError(t, err)
	assert.NoDirExists(t, filepath.Join(cacheDir, "0.0.9"))
	assert.DirExists(t, filepath.Join(cacheDir, "0.1.0"))
}

func TestResolveDefinitionsCacheDir(t *testing.T) {
	assert.Equal(t, "/cc/packages", ResolveDefinitionsCacheDir(map[string]string{"CHE_CACHE_HOME": "/cc"}, "/home/u"))
	assert.Equal(t, "/xdg/che/packages", ResolveDefinitionsCacheDir(map[string]string{"XDG_CACHE_HOME": "/xdg"}, "/home/u"))
	assert.Equal(t, "/home/u/.cache/che/packages", ResolveDefinitionsCacheDir(map[string]string{}, "/home/u"))
}

func TestResolveUpdateBaseURL(t *testing.T) {
	assert.Equal(t, "http://stub", ResolveUpdateBaseURL(map[string]string{"CHE_PACKAGES_UPDATE_URL": "http://stub"}))
	assert.Equal(t, DefaultUpdateBaseURL, ResolveUpdateBaseURL(map[string]string{}))
}

// [<] 🤖🤖
