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
		"cp(dry-run): "+home+"/.config/zsh/c",
		"archive(dry-run): "+home+"/.local/share/che/backups/che-copy-",
		"cp(dry-run): /Library/LaunchDaemons/otelcol.plist",
		"chown(dry-run): root:wheel /Library/LaunchDaemons/otelcol.plist",
	)
}

// [<] 🤖🤖
