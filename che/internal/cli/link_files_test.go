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
		"mkdir(create,dry-run=delta): "+home+"/.config/zsh",
		"ln(create,dry-run=delta): "+home+"/.config/zsh/.zshrc",
		"archive(dry-run=delta): "+home+"/.local/share/che/backups/che-link-",
		"ln(create,dry-run=delta): /etc/zshrc",
	)
}

// [<] 🤖🤖
