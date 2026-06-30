package cli

// [>] 🤖🤖

import (
	"strings"
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// detect prints only the resolved profile, no dry-run lines.
func TestDetectCmd(t *testing.T) {
	setupDryRun(t)
	out := testutil.RunDry(t, DetectCmd, false)
	if got := strings.TrimSpace(out); got != testutil.CheProfile {
		t.Errorf("detect = %q, want %q", got, testutil.CheProfile)
	}
}

// [<] 🤖🤖
