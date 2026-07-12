// Render-repo-group-index generates a subgroup's repo index: dir structure with each repo's purpose doc inlined.
package main

// [>] 🤖🤖

import (
	"os"

	"gitlab.com/konradodwrot/go-modules/che/render/checkcmd"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

var version = "dev"

const usage = `usage: render-repo-group-index <subgroup-dir>
       render-repo-group-index --check <file>

Print the repo-group index for <subgroup-dir> (stdout): a # Repositories
section (where you are, the group's directory structure tree, then each direct
child repo (a dir with .git) as ## Repo: ./<rel-path> with its
assets/docs-agents/purpose.md body inlined, or a placeholder when missing),
then each child subgroup as ## Subgroup: ./<rel-path> with its repos inlined
recursively (purposes only, no repeated tree or section headings).
--check regenerates for <file>'s dir and diffs against <file>:
exit 0 match, 22 differ (unified diff on stderr).
`

var tool = checkcmd.Tool{
	Name:     "render-repo-group-index",
	Version:  version,
	Usage:    usage,
	Label:    "render-repo-group-index",
	NeedsArg: true,
	CheckArg: ".",
	Generate: render.RepoGroupIndexDir,
}

func main() {
	os.Exit(tool.Run(os.Args[1:]))
}

//[<] 🤖🤖
