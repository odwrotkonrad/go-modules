package cli

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// mk-dirs: repo-tree config dirs plus profile extra-dir, under $HOME.
func TestDirsCmd(t *testing.T) {
	home := setupDryRun(t)
	out := testutil.RunDry(t, DirsCmd, true)
	testutil.WantLines(t, out,
		"mkdir(dry-run): "+home+"/.config/zsh",
		"mkdir(dry-run): "+home+"/.cache/zsh",           // mkdirs entry
		"mkdir(dry-run): /var/log/che-test-setgid",      // setgid mkdirs entry
		"chmod(dry-run): 2775 /var/log/che-test-setgid", // spec chmod reapplied for setgid bit
	)
}

// [<] 🤖🤖
