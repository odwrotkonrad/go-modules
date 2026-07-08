package cli

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// TestRenderCmd: host profile renders its templates to derived host dests;
// the non-autoExec ontoRepo profile stays out.
func TestRenderCmd(t *testing.T) {
	home := setupDryRun(t)
	out := testutil.RunDry(t, RenderCmd, true)
	testutil.WantLines(t, out, "render(create,dry-run=delta): "+home+"/.config/zsh/t")
	testutil.NotLine(t, out, "docs/r.md")
}

// TestRenderCmdOntoRepo: --profile=ontoRepo renders only the repo dests.
func TestRenderCmdOntoRepo(t *testing.T) {
	home := setupDryRun(t)
	profileForce = "ontoRepo"
	if err := build(); err != nil {
		t.Fatalf("build() errored: %v", err)
	}
	out := testutil.RunDry(t, RenderCmd, true)
	testutil.WantLines(t, out, "render(create,dry-run=delta): ", "docs/r.md")
	testutil.NotLine(t, out, home+"/.config/zsh/t")
}

// [<] 🤖🤖
