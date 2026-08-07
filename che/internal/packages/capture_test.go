package packages

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	out, err := testutil.CaptureStdout(t, fn)
	return testutil.StripANSI(out), err
}

func wantLines(t *testing.T, out string, fragments ...string) {
	t.Helper()
	testutil.WantLines(t, out, fragments...)
}

func notLine(t *testing.T, out, fragment string) {
	t.Helper()
	testutil.NotLine(t, out, fragment)
}

// [<] 🤖🤖
