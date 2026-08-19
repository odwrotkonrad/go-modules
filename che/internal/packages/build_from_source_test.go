package packages

// [>] 🤖🤖

import (
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

const rubySourceYaml = `packages:
  ruby:
    - buildFromSource:
        version: 3.4.10
        checksum: sha256:goodsha
        url: https://example.com/ruby-{version}.tar.gz
        configureArgs: [--enable-shared]
`

func makeJ() string { return "make -j" + strconv.Itoa(runtime.NumCPU()) }

func TestInstallBuildFromSourceFlow(t *testing.T) {
	body := tarGzBody(t, "ruby-3.4.10/configure")
	in, m := newInstaller(t, withSha(rubySourceYaml, body), "linux", cmdMap(nil), Options{})
	home := tempHome(t, in)
	testFetch.Bodies["https://example.com/ruby-3.4.10.tar.gz"] = body
	require.NoError(t, in.Install([]string{"ruby"}))
	require.Contains(t, testFetch.Calls(), "https://example.com/ruby-3.4.10.tar.gz")
	require.Equal(t, []string{
		"./configure --prefix=" + filepath.Join(home, ".local") + " --enable-shared",
		makeJ(),
		"make install",
	}, m.Calls())
}

func TestInstallBuildFromSourceSudoOnSystemPrefix(t *testing.T) {
	body := tarGzBody(t, "ruby-3.4.10/configure")
	in, m := newInstaller(t, withSha(rubySourceYaml, body), "linux", cmdMap(nil),
		Options{BinariesRemoteArchiveDestinationCandidates: []string{"/usr/local/bin"}})
	tempHome(t, in)
	testFetch.Bodies["https://example.com/ruby-3.4.10.tar.gz"] = body
	require.NoError(t, in.Install([]string{"ruby"}))
	require.Equal(t, []string{
		"./configure --prefix=/usr/local --enable-shared",
		makeJ(),
		"sudo make install",
	}, m.Calls())
}

func TestInstallBuildFromSourceShaMismatchAborts(t *testing.T) {
	in, m := newInstaller(t, rubySourceYaml, "linux", cmdMap(nil), Options{})
	tempHome(t, in)
	testFetch.Bodies["https://example.com/ruby-3.4.10.tar.gz"] = tarGzBody(t, "ruby-3.4.10/configure")
	err := in.Install([]string{"ruby"})
	require.ErrorContains(t, err, "sha256 mismatch")
	require.Empty(t, m.Calls())
}

const rubyUnpinnedSourceYaml = `packages:
  ruby:
    - buildFromSource:
        url: https://example.com/ruby-{version}.tar.gz
`

func TestInstallBuildFromSourceRequestChecksumVerifies(t *testing.T) {
	body := tarGzBody(t, "ruby-3.4.10/configure")
	in, m := newInstaller(t, rubyUnpinnedSourceYaml, "linux", cmdMap(nil), Options{})
	home := tempHome(t, in)
	testFetch.Bodies["https://example.com/ruby-3.4.10.tar.gz"] = body
	req := Request{Name: "ruby", Versions: []string{"3.4.10"}, Checksum: "sha256:" + shaHex(body)}
	require.NoError(t, in.InstallRequests([]Request{req}))
	require.Equal(t, []string{
		"./configure --prefix=" + filepath.Join(home, ".local"),
		makeJ(),
		"make install",
	}, m.Calls())
}

func TestInstallBuildFromSourceRequestChecksumMismatchAborts(t *testing.T) {
	in, m := newInstaller(t, rubyUnpinnedSourceYaml, "linux", cmdMap(nil), Options{})
	tempHome(t, in)
	testFetch.Bodies["https://example.com/ruby-3.4.10.tar.gz"] = tarGzBody(t, "ruby-3.4.10/configure")
	req := Request{Name: "ruby", Versions: []string{"3.4.10"}, Checksum: "sha256:badsha"}
	err := in.InstallRequests([]Request{req})
	require.ErrorContains(t, err, "sha256 mismatch")
	require.Empty(t, m.Calls())
}

func TestInstallBuildFromSourceSkipsWhenPinPresent(t *testing.T) {
	in, m := newInstaller(t, rubySourceYaml, "linux", cmdMap([]string{"ruby"}), Options{})
	m.Stub = stubOutputs("ruby --version", "ruby 3.4.10 (2025-12-25 revision abc)\n")
	require.NoError(t, in.Install([]string{"ruby"}))
	require.Empty(t, testFetch.Calls())
	require.Equal(t, []string{"ruby --version"}, m.Calls())
}

func TestInstallBuildFromSourceDryRun(t *testing.T) {
	in, m := newInstaller(t, rubySourceYaml, "linux", cmdMap(nil), Options{DryRun: true})
	tempHome(t, in)
	out, err := captureStdout(t, func() error { return in.Install([]string{"ruby"}) })
	require.NoError(t, err)
	wantLines(t, out, "ruby 3.4.10 via buildFromSource")
	require.Empty(t, testFetch.Calls())
	require.Empty(t, m.Calls())
}

func TestInstallBuildFromSourceErrorsWithoutSingleSourceDir(t *testing.T) {
	body := tarGzBody(t, "configure", "extra")
	in, _ := newInstaller(t, withSha(rubySourceYaml, body), "linux", cmdMap(nil), Options{})
	tempHome(t, in)
	testFetch.Bodies["https://example.com/ruby-3.4.10.tar.gz"] = body
	require.ErrorContains(t, in.Install([]string{"ruby"}), "single top-level directory")
}

// [<] 🤖🤖
