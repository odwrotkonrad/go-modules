// [>] 🤖🤖
package main

import (
	"fmt"
	"os"

	"gitlab.com/konradodwrot/go/render-files/checkcmd"
	"gitlab.com/konradodwrot/go/render-files/render"
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
	Usage:    usage,
	Label:    "render-repo-group-index",
	NeedsArg: true,
	CheckArg: ".",
	Generate: func(arg string) (string, error) { return render.RepoGroupIndexDir(arg) },
}

func main() {
	args := os.Args[1:]
	if len(args) == 1 && (args[0] == "-v" || args[0] == "--version") {
		fmt.Println("render-repo-group-index version", version)
		return
	}
	os.Exit(tool.Run(args))
}

//[<] 🤖🤖
