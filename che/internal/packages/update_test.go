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
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
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

	res, err := UpdateDefinitions(cacheDir, Source{URL: "http://stub"}, DefaultUpdateCooldown, false)
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

	res, err := UpdateDefinitions(cacheDir, Source{URL: "http://stub"}, time.Hour, false)
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

	res, err := UpdateDefinitions(cacheDir, Source{URL: "http://stub"}, time.Hour, true)
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

	res, err := UpdateDefinitions(cacheDir, Source{URL: "http://stub"}, 0, false)
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

	_, err := UpdateDefinitions(cacheDir, Source{URL: "http://stub"}, 0, false)
	require.ErrorContains(t, err, "checksum mismatch")
	assert.NoDirExists(t, filepath.Join(cacheDir, "0.2.0"))
}

func TestUpdateDefinitionsPrunesOlderVersions(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "packages")
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, "0.0.9"), 0o755))
	archive := makeDefinitionsTarGz(t, map[string]string{"packages.yml": "packages: {}\n"})
	mockDefinitionsRegistry(t, "http://stub", "0.1.0", archive)

	_, err := UpdateDefinitions(cacheDir, Source{URL: "http://stub"}, 0, false)
	require.NoError(t, err)
	assert.NoDirExists(t, filepath.Join(cacheDir, "0.0.9"))
	assert.DirExists(t, filepath.Join(cacheDir, "0.1.0"))
}

func TestResolveDefinitionsCacheDir(t *testing.T) {
	assert.Equal(t, "/cc/packages", ResolveDefinitionsCacheDir(map[string]string{"CHE_CACHE_HOME": "/cc"}, "/home/u"))
	assert.Equal(t, "/xdg/che/packages", ResolveDefinitionsCacheDir(map[string]string{"XDG_CACHE_HOME": "/xdg"}, "/home/u"))
	assert.Equal(t, "/home/u/.cache/che/packages", ResolveDefinitionsCacheDir(map[string]string{}, "/home/u"))
}

func TestUpdateDefinitionsClonesGitSource(t *testing.T) {
	origin := testutil.Repo(t, map[string]string{
		"packages.yml":            "packages: {}\n",
		"scripts/post-install.sh": "echo ok\n",
	})
	cacheDir := filepath.Join(t.TempDir(), "packages")
	m := &fetchx.Mock{}
	fetchx.Swap(t, m)

	res, err := UpdateDefinitions(cacheDir, Source{URL: origin + "/.git"}, 0, false)
	require.NoError(t, err)
	assert.True(t, res.Updated)
	dir, version, ok := ResolveCurrentDefinitions(cacheDir)
	require.True(t, ok)
	assert.Equal(t, res.Version, version)
	assert.FileExists(t, filepath.Join(dir, "packages.yml"))
	assert.FileExists(t, filepath.Join(dir, "scripts", "post-install.sh"))
	assert.Empty(t, m.Calls(), "a git source never touches the registry")
}

// [why] the cache dir is keyed on the resolved commit, not the ref: a branch names different
//
//	content over time, and a dir keyed on its name would serve the first clone forever.
func TestUpdateDefinitionsGitSourceKeysCacheOnCommit(t *testing.T) {
	origin := testutil.Repo(t, map[string]string{"packages.yml": "packages: {}\n"})
	fetchx.Swap(t, &fetchx.Mock{})

	res, err := UpdateDefinitions(filepath.Join(t.TempDir(), "packages"), Source{URL: origin + "/.git"}, 0, false)
	require.NoError(t, err)
	assert.Len(t, res.Version, 12)
	assert.NotEqual(t, "main", res.Version)
}

func TestUpdateDefinitionsGitSourceReportsMissingCatalog(t *testing.T) {
	origin := testutil.Repo(t, map[string]string{"readme.md": "no catalog here\n"})
	fetchx.Swap(t, &fetchx.Mock{})

	_, err := UpdateDefinitions(filepath.Join(t.TempDir(), "packages"), Source{URL: origin + "/.git"}, 0, false)
	require.ErrorContains(t, err, "carries no packages.yml")
}

func TestResolveSourceURL(t *testing.T) {
	assert.Equal(t, "http://stub", ResolveSourceURL(map[string]string{"CHE_PACKAGES_SOURCE_URL": "http://stub"}))
	assert.Equal(t, "http://legacy", ResolveSourceURL(map[string]string{"CHE_PACKAGES_UPDATE_URL": "http://legacy"}))
	assert.Equal(t, "http://stub", ResolveSourceURL(map[string]string{"CHE_PACKAGES_SOURCE_URL": "http://stub", "CHE_PACKAGES_UPDATE_URL": "http://legacy"}))
	assert.Equal(t, DefaultSourceURL, ResolveSourceURL(map[string]string{}))
}

func TestResolveSourceRef(t *testing.T) {
	assert.Equal(t, "0.0.11", ResolveSourceRef(map[string]string{"CHE_PACKAGES_REF": "0.0.11"}))
	assert.Empty(t, ResolveSourceRef(map[string]string{}))
}

func TestSourceKindRecognisesGitAndRegistry(t *testing.T) {
	for _, url := range []string{
		"https://gitlab.com/konradodwrot/che-packages.git",
		"git+https://gitlab.com/konradodwrot/che-packages",
		"git@gitlab.com:konradodwrot/che-packages.git",
		"ssh://git@gitlab.com/konradodwrot/che-packages",
	} {
		assert.Equal(t, SourceGit, Source{URL: url}.Kind(), url)
	}
	for _, url := range []string{DefaultSourceURL, "http://stub", "https://example.test/packages/generic/che-packages"} {
		assert.Equal(t, SourceRegistry, Source{URL: url}.Kind(), url)
	}
}

func TestSourcePinnedReportsExactVersionRequest(t *testing.T) {
	assert.True(t, Source{URL: "http://stub", Ref: "0.1.0"}.Pinned())
	assert.False(t, Source{URL: "http://stub"}.Pinned())
}

func TestUpdateDefinitionsPinnedRefFetchesExactVersionWithoutResolvingLatest(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "packages")
	archive := makeDefinitionsTarGz(t, map[string]string{"packages.yml": "packages: {}\n"})
	sum := sha256.Sum256(archive)
	name := "che-packages_0.3.0.tar.gz"
	m := &fetchx.Mock{Bodies: map[string][]byte{
		"http://stub/0.3.0/" + name:       archive,
		"http://stub/0.3.0/checksums.txt": []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"),
	}}
	fetchx.Swap(t, m)

	res, err := UpdateDefinitions(cacheDir, Source{URL: "http://stub", Ref: "0.3.0"}, 0, false)
	require.NoError(t, err)
	assert.Equal(t, UpdateResult{Version: "0.3.0", Updated: true}, res)
	assert.NotContains(t, m.Calls(), "http://stub/latest/version.txt")
}

// [why] the cooldown guards resolving which version is newest: a pinned ref resolves nothing,
//
//	so honouring it there would withhold the exact version the caller asked for.
func TestUpdateDefinitionsPinnedRefIgnoresCooldown(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "packages")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "last-check"), nil, 0o644))
	archive := makeDefinitionsTarGz(t, map[string]string{"packages.yml": "packages: {}\n"})
	sum := sha256.Sum256(archive)
	name := "che-packages_0.4.0.tar.gz"
	fetchx.Swap(t, &fetchx.Mock{Bodies: map[string][]byte{
		"http://stub/0.4.0/" + name:       archive,
		"http://stub/0.4.0/checksums.txt": []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"),
	}})

	res, err := UpdateDefinitions(cacheDir, Source{URL: "http://stub", Ref: "0.4.0"}, time.Hour, false)
	require.NoError(t, err)
	assert.True(t, res.Updated)
	assert.Equal(t, "0.4.0", res.Version)
}

// [<] 🤖🤖
