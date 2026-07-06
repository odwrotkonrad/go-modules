package cli

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// copy: *.host.cp into $HOME (marker stripped); archive existing dests, copy, chown root:wheel the daemon plist.
func TestCopyCmd(t *testing.T) {
	home := setupDryRun(t)
	out := testutil.RunDry(t, CopyCmd, true)
	testutil.WantLines(t, out,
		"cp(create,dry-run=delta): "+home+"/.config/zsh/c",
		"archive(dry-run=delta): "+home+"/.local/share/che/backups/che-copy-",
		"cp(create,dry-run=delta): /Library/LaunchDaemons/otelcol.plist",
		"cp(chown,dry-run=delta): root:wheel /Library/LaunchDaemons/otelcol.plist",
	)
}

// [<] 🤖🤖
