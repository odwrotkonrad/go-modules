package cli

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// prune-links: logs scanned root, fresh fixture has nothing to remove.
func TestPruneCmd(t *testing.T) {
	setupDryRun(t)
	out := testutil.RunDry(t, PruneCmd, true)
	testutil.WantLines(t, out, "prune-links(dry-run): "+theHost.Root)
}

// [<] 🤖🤖
