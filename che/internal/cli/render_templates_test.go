package cli

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// TestRenderCmd: one op renders every resolved template; dest path decides
// target (derived host path under $HOME, repo-relative dest in the repo).
func TestRenderCmd(t *testing.T) {
	home := setupDryRun(t)
	out := testutil.RunDry(t, RenderCmd, true)
	testutil.WantLines(t, out,
		"render(create,dry-run=delta): "+home+"/.config/zsh/t",
		"render(create,dry-run=delta): ",
		"docs/r.md",
	)
}

// [<] 🤖🤖
