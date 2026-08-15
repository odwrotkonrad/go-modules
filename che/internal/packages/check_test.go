package packages

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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
