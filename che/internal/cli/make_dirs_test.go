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
		"mkdir(create,dry-run=delta): "+home+"/.config/zsh",
		"mkdir(create,dry-run=delta): "+home+"/.cache/zsh",           // mkdirs entry
		"mkdir(create,dry-run=delta): /var/log/che-test-setgid",      // setgid mkdirs entry
		"mkdir(chmod,dry-run=delta): 2775 /var/log/che-test-setgid", // spec chmod reapplied for setgid bit
	)
}

// [<] 🤖🤖
