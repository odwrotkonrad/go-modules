package packages

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckPresentReportsMissing(t *testing.T) {
	in, _ := newInstaller(t, "packages:\n  bat: [brew]\n  fd: [brew]", "darwin", cmdMap([]string{"bat"}), Options{})
	out, err := captureStdout(t, func() error {
		require.Equal(t, []string{"fd"}, in.CheckPresent([]string{"bat", "fd"}))
		return nil
	})
	require.NoError(t, err)
	wantLines(t, out, "missing fd")
}

func TestCheckPresentSkipsPathProbeWhenDisabled(t *testing.T) {
	yml := `packages:
  apt-transport-https:
    installers:
      - apt:
          verify:
            pkgMgrVersionCheck: true
            checkInPath: false
  nvm:
    verify:
      cmd: true
      checkInPath: false
    installers:
      - script:
          url: https://example.com/install.sh
  fd: [apt]`
	in, _ := newInstaller(t, yml, "linux", cmdMap([]string{"apt-get"}), Options{})
	out, err := captureStdout(t, func() error {
		require.Equal(t, []string{"fd"}, in.CheckPresent([]string{"apt-transport-https", "nvm", "fd"}))
		return nil
	})
	require.NoError(t, err)
	wantLines(t, out, "missing fd")
	notLine(t, out, "missing apt-transport-https")
	notLine(t, out, "missing nvm")
}

func TestCheckUpgradableBrewOutdated(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  bat: [brew]\n  fd: [brew]", "darwin", cmdMap([]string{"brew"}), Options{})
	m.Stub = stubOutputs("brew outdated", "bat\n")
	out, err := captureStdout(t, func() error { return in.CheckUpgradable([]string{"bat", "fd"}) })
	require.NoError(t, err)
	wantLines(t, out, "upgradable bat via brew")
	notLine(t, out, "upgradable fd via brew")
}

func TestCheckUpgradableBinaryPinDrift(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"kind"}), Options{})
	m.Stub = stubOutputs("kind ", "kind version 0.30.0\n")
	out, err := captureStdout(t, func() error { return in.CheckUpgradable([]string{"kind"}) })
	require.NoError(t, err)
	wantLines(t, out, "upgradable kind via binariesRemoteArchive: yaml pins 0.32.0")
}

func TestCheckNotShadowedWarns(t *testing.T) {
	prefix := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(prefix, "bin"), 0o755))
	expected := filepath.Join(prefix, "bin", "kind")
	require.NoError(t, os.WriteFile(expected, []byte(""), 0o755))
	cmds := map[string]string{"brew": "/opt/homebrew/bin/brew", "kind": "/usr/local/bin/kind"}
	in, m := newInstaller(t, "packages:\n  kind: [brew]", "darwin", cmds, Options{})
	m.Stub = stubOutputs("brew --prefix", prefix+"\n")
	out, err := captureStdout(t, func() error { return in.CheckNotShadowed([]string{"kind"}) })
	require.NoError(t, err)
	wantLines(t, out, "kind shadowed by /usr/local/bin/kind (expected "+expected+")")
}

func TestCheckSinglePresentListsDuplicates(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	for _, d := range []string{dirA, dirB} {
		require.NoError(t, os.WriteFile(filepath.Join(d, "kind"), []byte(""), 0o755))
	}
	in, _ := newInstaller(t, "packages:\n  kind: [brew]", "darwin", cmdMap(nil), Options{})
	in.Host.PathDirs = func() []string { return []string{dirA, dirB} }
	out, err := captureStdout(t, func() error { return in.CheckSinglePresent([]string{"kind"}) })
	require.NoError(t, err)
	wantLines(t, out, "multiple present kind: "+filepath.Join(dirA, "kind")+", "+filepath.Join(dirB, "kind"))
}

// [<] 🤖🤖
