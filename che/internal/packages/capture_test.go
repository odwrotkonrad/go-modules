package packages

// [>] 🤖🤖

import (
	"io"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	runErr := fn()
	_ = w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	return ansiRe.ReplaceAllString(string(out), ""), runErr
}

func wantLines(t *testing.T, out string, fragments ...string) {
	t.Helper()
	for _, f := range fragments {
		require.Containsf(t, out, f, "output missing %q", f)
	}
}

func notLine(t *testing.T, out, fragment string) {
	t.Helper()
	require.NotContains(t, out, fragment)
}

// [<] 🤖🤖
