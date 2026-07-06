package cli

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// TestRenderCmd: no flag => both host + repo. host *.host.tpl -> $HOME, repo
// *.repo.tpl -> repo-relative dest, both under the dry-run scope.
func TestRenderCmd(t *testing.T) {
	home := setupDryRun(t)
	renderHost, renderRepo = false, false
	out := testutil.RunDry(t, RenderCmd, true)
	testutil.WantLines(t, out,
		"render(create,dry-run=delta): "+home+"/.config/zsh/t",
		"render-repo(create,dry-run=delta): ",
		"docs/r.md",
	)
}

// TestRenderCmdRepoOnly: --repo renders only *.repo.tpl.
func TestRenderCmdRepoOnly(t *testing.T) {
	home := setupDryRun(t)
	renderHost, renderRepo = false, true
	t.Cleanup(func() { renderHost, renderRepo = false, false })
	out := testutil.RunDry(t, RenderCmd, true)
	testutil.WantLines(t, out, "render-repo(create,dry-run=delta): ", "docs/r.md")
	testutil.NotLine(t, out, home+"/.config/zsh/t")
}

// TestRenderCmdHostOnly: --host renders only *.host.tpl.
func TestRenderCmdHostOnly(t *testing.T) {
	home := setupDryRun(t)
	renderHost, renderRepo = true, false
	t.Cleanup(func() { renderHost, renderRepo = false, false })
	out := testutil.RunDry(t, RenderCmd, true)
	testutil.WantLines(t, out, "render(create,dry-run=delta): "+home+"/.config/zsh/t")
	testutil.NotLine(t, out, "render-repo(create")
}

// [<] 🤖🤖
