package packages

// [>] 🤖🤖

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const kubectxYaml = `packages:
  kubectx:
    - binariesRemoteArchive:
        archScheme: odd
        extractBinaries: [kubectx]
        platformEligibility:
          - linux-amd64: sha256:goodsha
        version: 0.11.0
        url: https://example.com/v{version}/kubectx_v{version}_{os}_{arch}.tar.gz
`

const kindYaml = `packages:
  kind:
    - binariesRemoteArchive:
        platformEligibility:
          - linux-amd64: sha256:goodsha
        version: 0.32.0
        url: https://example.com/v{version}/kind-{os}-{arch}
`

func requireSymlink(t *testing.T, link, target string) {
	t.Helper()
	resolved, err := os.Readlink(link)
	require.NoError(t, err)
	require.Equal(t, target, resolved)
	require.FileExists(t, target)
}

func TestInstallBinaryTarFlow(t *testing.T) {
	body := tarGzBody(t, "kubectx")
	in, m := newInstaller(t, withSha(kubectxYaml, body), "linux", cmdMap(nil), Options{})
	home := tempHome(t, in)
	testFetch.Bodies["https://example.com/v0.11.0/kubectx_v0.11.0_linux_x86_64.tar.gz"] = body
	require.NoError(t, in.Install([]string{"kubectx"}))
	require.Contains(t, testFetch.Calls(), "https://example.com/v0.11.0/kubectx_v0.11.0_linux_x86_64.tar.gz")
	requireSymlink(t, filepath.Join(home, ".local", "bin", "kubectx"), filepath.Join(home, ".local", "opt", "kubectx", "kubectx"))
	require.Empty(t, m.Calls())
}

func TestInstallBinaryShaMismatchAborts(t *testing.T) {
	in, _ := newInstaller(t, kubectxYaml, "linux", cmdMap(nil), Options{})
	home := tempHome(t, in)
	err := in.Install([]string{"kubectx"})
	require.ErrorContains(t, err, "sha256 mismatch")
	require.NoDirExists(t, filepath.Join(home, ".local", "opt", "kubectx"))
}

func TestInstallBinaryBareAsset(t *testing.T) {
	body := []byte("#!/bin/sh\n")
	in, m := newInstaller(t, withSha(kindYaml, body), "linux", cmdMap(nil), Options{})
	home := tempHome(t, in)
	testFetch.Bodies["https://example.com/v0.32.0/kind-linux-amd64"] = body
	require.NoError(t, in.Install([]string{"kind"}))
	require.Contains(t, testFetch.Calls(), "https://example.com/v0.32.0/kind-linux-amd64")
	bin := filepath.Join(home, ".local", "bin", "kind")
	fi, err := os.Stat(bin)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o755), fi.Mode().Perm())
	got, err := os.ReadFile(bin)
	require.NoError(t, err)
	require.Equal(t, body, got)
	require.Empty(t, m.Calls())
}

func TestInstallBinaryDownloadCacheReusesAsset(t *testing.T) {
	body := tarGzBody(t, "kubectx")
	opts := Options{DownloadCacheDir: t.TempDir()}
	yaml := withSha(kubectxYaml, body)
	in, _ := newInstaller(t, yaml, "linux", cmdMap(nil), opts)
	tempHome(t, in)
	testFetch.Bodies["https://example.com/v0.11.0/kubectx_v0.11.0_linux_x86_64.tar.gz"] = body
	require.NoError(t, in.Install([]string{"kubectx"}))
	require.Len(t, testFetch.Calls(), 1)
	in, _ = newInstaller(t, yaml, "linux", cmdMap(nil), opts)
	home := tempHome(t, in)
	require.NoError(t, in.Install([]string{"kubectx"}))
	require.Empty(t, testFetch.Calls())
	require.FileExists(t, filepath.Join(home, ".local", "opt", "kubectx", "kubectx"))
}

func TestInstallBinaryDownloadCacheEvictsOnShaMismatch(t *testing.T) {
	cache := t.TempDir()
	in, _ := newInstaller(t, kubectxYaml, "linux", cmdMap(nil), Options{DownloadCacheDir: cache})
	tempHome(t, in)
	err := in.Install([]string{"kubectx"})
	require.ErrorContains(t, err, "sha256 mismatch")
	entries, err := os.ReadDir(cache)
	require.NoError(t, err)
	require.Empty(t, entries)
}

const zigYaml = `packages:
  zig:
    - binariesRemoteArchive:
        archScheme: uname
        extractBinaries: ["zig-{arch}-linux-{version}/zig"]
        platformEligibility:
          - linux-amd64: sha256:goodsha
        version: 0.16.0
        url: https://example.com/{version}/zig-{arch}-linux-{version}.tar.xz
`

func TestInstallBinaryTreeFlow(t *testing.T) {
	body := tarXzBody(t, "zig-x86_64-linux-0.16.0/zig")
	in, _ := newInstaller(t, withSha(zigYaml, body), "linux", cmdMap(nil), Options{})
	home := tempHome(t, in)
	testFetch.Bodies["https://example.com/0.16.0/zig-x86_64-linux-0.16.0.tar.xz"] = body
	require.NoError(t, in.Install([]string{"zig"}))
	require.Contains(t, testFetch.Calls(), "https://example.com/0.16.0/zig-x86_64-linux-0.16.0.tar.xz")
	requireSymlink(t, filepath.Join(home, ".local", "bin", "zig"),
		filepath.Join(home, ".local", "opt", "zig", "zig-x86_64-linux-0.16.0", "zig"))
}

const gcloudArchiveYaml = `archSchemes:
  vendor: {arm64: arm, amd64: x86_64}
packages:
  gcloud:
    - binariesRemoteArchive:
        archScheme: vendor
        extractBinaries: [sdk/bin/gcloud, sdk/bin/gsutil]
        platformEligibility:
          - darwin-arm64: sha256:goodsha
        version: 572.0.0
        url: https://example.com/cli-{version}-{os}-{arch}.tar.gz
`

func TestInstallBinariesRemoteArchiveArchScheme(t *testing.T) {
	body := tarGzBody(t, "sdk/bin/gcloud", "sdk/bin/gsutil")
	in, _ := newInstaller(t, withSha(gcloudArchiveYaml, body), "darwin", cmdMap(nil), Options{})
	in.Host.Arch = "arm64"
	home := tempHome(t, in)
	testFetch.Bodies["https://example.com/cli-572.0.0-darwin-arm.tar.gz"] = body
	require.NoError(t, in.Install([]string{"gcloud"}))
	require.Contains(t, testFetch.Calls(), "https://example.com/cli-572.0.0-darwin-arm.tar.gz")
	requireSymlink(t, filepath.Join(home, ".local", "bin", "gcloud"), filepath.Join(home, ".local", "opt", "gcloud", "sdk", "bin", "gcloud"))
	requireSymlink(t, filepath.Join(home, ".local", "bin", "gsutil"), filepath.Join(home, ".local", "opt", "gcloud", "sdk", "bin", "gsutil"))
}

func TestInstallBinarySkipsWhenPinPresent(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"kind"}), Options{})
	m.Stub = stubOutputs("kind ", "kind version 0.32.0\n")
	require.NoError(t, in.Install([]string{"kind"}))
	require.Empty(t, testFetch.Calls())
}

func TestInstallBinaryReinstallsOnPinDrift(t *testing.T) {
	body := []byte("kind-bin")
	in, m := newInstaller(t, withSha(kindYaml, body), "linux", cmdMap([]string{"kind"}), Options{})
	tempHome(t, in)
	m.Stub = stubOutputs("kind ", "kind version 0.30.0\n")
	testFetch.Bodies["https://example.com/v0.32.0/kind-linux-amd64"] = body
	require.NoError(t, in.Install([]string{"kind"}))
	require.NotEmpty(t, testFetch.Calls())
}

const awsYaml = `packages:
  aws:
    - script:
        os: darwin
        url: https://awscli.amazonaws.com/v2/install.sh
    - binariesRemoteArchive:
        archScheme: uname
        platformEligibility: [linux-amd64, linux-arm64]
        extractBinaries: [aws/dist/aws, aws/dist/aws_completer]
        url: https://awscli.amazonaws.com/awscli-exe-linux-{arch}.zip
`

func TestInstallAwsScriptOnDarwin(t *testing.T) {
	in, m := newInstaller(t, awsYaml, "darwin", cmdMap(nil), Options{})
	require.NoError(t, in.Install([]string{"aws"}))
	require.Contains(t, testFetch.Calls(), "https://awscli.amazonaws.com/v2/install.sh")
	requireCalls(t, m, "che-script-")
}

func TestInstallAwsSkipsWhenPresent(t *testing.T) {
	in, m := newInstaller(t, awsYaml, "darwin", cmdMap([]string{"aws"}), Options{})
	require.NoError(t, in.Install([]string{"aws"}))
	require.Empty(t, m.Calls())
}

func TestInstallAwsLinuxArchive(t *testing.T) {
	in, m := newInstaller(t, awsYaml, "linux", cmdMap(nil), Options{})
	home := tempHome(t, in)
	testFetch.Bodies["https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip"] = zipBody(t, "aws/dist/aws", "aws/dist/aws_completer")
	require.NoError(t, in.Install([]string{"aws"}))
	require.Contains(t, testFetch.Calls(), "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip")
	requireSymlink(t, filepath.Join(home, ".local", "bin", "aws"), filepath.Join(home, ".local", "opt", "aws", "aws", "dist", "aws"))
	requireSymlink(t, filepath.Join(home, ".local", "bin", "aws_completer"), filepath.Join(home, ".local", "opt", "aws", "aws", "dist", "aws_completer"))
	refuteCalls(t, m, "install --update")
}

func TestInstallBinaryCustomDestination(t *testing.T) {
	body := []byte("kind-bin")
	in, _ := newInstaller(t, withSha(kindYaml, body), "linux", cmdMap(nil),
		Options{BinariesRemoteArchiveDestinationCandidates: []string{"~/bin"}})
	home := tempHome(t, in)
	testFetch.Bodies["https://example.com/v0.32.0/kind-linux-amd64"] = body
	require.NoError(t, in.Install([]string{"kind"}))
	require.FileExists(t, filepath.Join(home, "bin", "kind"))
}

func TestInstallBinaryPicksFirstCandidateOnPath(t *testing.T) {
	body := []byte("kind-bin")
	in, _ := newInstaller(t, withSha(kindYaml, body), "linux", cmdMap(nil),
		Options{BinariesRemoteArchiveDestinationCandidates: []string{"/custom/bin", "~/bin"}, BinariesRemoteArchiveCheckPresentOnPath: true})
	home := tempHome(t, in)
	in.Host.PathDirs = func() []string { return []string{"/usr/bin", filepath.Join(home, "bin")} }
	testFetch.Bodies["https://example.com/v0.32.0/kind-linux-amd64"] = body
	out, err := captureStdout(t, func() error { return in.Install([]string{"kind"}) })
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(home, "bin", "kind"))
	notLine(t, out, "not on path")
}

func TestInstallBinaryWarnsWhenNoCandidateOnPath(t *testing.T) {
	body := []byte("kind-bin")
	dirA, dirB := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")
	in, _ := newInstaller(t, withSha(kindYaml, body), "linux", cmdMap(nil),
		Options{BinariesRemoteArchiveDestinationCandidates: []string{dirA, dirB}, BinariesRemoteArchiveCheckPresentOnPath: true})
	in.Host.PathDirs = func() []string { return []string{"/usr/bin"} }
	testFetch.Bodies["https://example.com/v0.32.0/kind-linux-amd64"] = body
	out, err := captureStdout(t, func() error { return in.Install([]string{"kind"}) })
	require.NoError(t, err)
	wantLines(t, out, "no packages.binariesRemoteArchive.installDestinationCandidates entry is on PATH ("+dirA+", "+dirB+"), using "+dirA)
	require.FileExists(t, filepath.Join(dirA, "kind"))
}

func TestInstallBinariesRemoteArchiveCheckPresentOnPathOffUsesFirstCandidate(t *testing.T) {
	body := []byte("kind-bin")
	dirA := filepath.Join(t.TempDir(), "a")
	in, _ := newInstaller(t, withSha(kindYaml, body), "linux", cmdMap(nil),
		Options{BinariesRemoteArchiveDestinationCandidates: []string{dirA, "~/bin"}})
	home := tempHome(t, in)
	in.Host.PathDirs = func() []string { return []string{filepath.Join(home, "bin")} }
	testFetch.Bodies["https://example.com/v0.32.0/kind-linux-amd64"] = body
	out, err := captureStdout(t, func() error { return in.Install([]string{"kind"}) })
	require.NoError(t, err)
	notLine(t, out, "not on path")
	require.FileExists(t, filepath.Join(dirA, "kind"))
}

// [<] 🤖🤖

// [>] 🤖🤖
func TestExtractTarPreservesModTimes(t *testing.T) {
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	newer := time.Date(2024, 6, 7, 8, 9, 10, 0, time.UTC)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range []struct {
		name string
		mod  time.Time
	}{{"src/configure.ac", old}, {"src/configure", newer}} {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o755, Size: 3, ModTime: f.mod}))
		_, err := tw.Write([]byte("bin"))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	asset := filepath.Join(t.TempDir(), "src.tar.gz")
	require.NoError(t, os.WriteFile(asset, buf.Bytes(), 0o644))
	dest := t.TempDir()
	require.NoError(t, extractTar(asset, dest))
	fiAc, err := os.Stat(filepath.Join(dest, "src", "configure.ac"))
	require.NoError(t, err)
	fiCfg, err := os.Stat(filepath.Join(dest, "src", "configure"))
	require.NoError(t, err)
	require.Equal(t, old, fiAc.ModTime().UTC())
	require.Equal(t, newer, fiCfg.ModTime().UTC())
	require.True(t, fiCfg.ModTime().After(fiAc.ModTime()))
}

// [<] 🤖🤖
