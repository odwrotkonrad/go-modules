package cli

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// link: mkdir HOME config, archive existing dests, link user zshrc into $HOME and /etc/zshrc.
func TestLinkCmd(t *testing.T) {
	home := setupDryRun(t)
	out := testutil.RunDry(t, LinkCmd, true)
	testutil.WantLines(t, out,
		"mkdir(dry-run): "+home+"/.config/zsh",
		"ln(dry-run): "+home+"/.config/zsh/.zshrc",
		"archive(dry-run): "+home+"/.local/share/che/backups/che-link-",
		"ln(dry-run): /etc/zshrc",
	)
}

// [<] 🤖🤖
