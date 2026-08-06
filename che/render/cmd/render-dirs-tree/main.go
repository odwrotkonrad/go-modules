// Render-dirs-tree prints an indented directory tree of git-tracked files.
package main

// [>] 🤖🤖

import (
	"gitlab.com/konradodwrot/go-modules/che/render/checkcmd"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

var version = "dev"

const usage = `usage: render-dirs-tree
       render-dirs-tree --check <file>

Print the plain directory tree of the cwd repo's tracked files (stdout):
read tracked paths from the git index, drop each file leaf, nest and sort
the remaining dirs, 2-space indented, one dir per line.
--check regenerates and diffs against <file>:
exit 0 match, 22 differ (unified diff on stderr).
`

var tool = checkcmd.Tool{
	Name:     "render-dirs-tree",
	Version:  version,
	Usage:    usage,
	Label:    "render-dirs-tree",
	CheckArg: ".",
	Generate: func(string) (string, error) { return render.DirsTree(".") },
}

func main() { tool.Main() }

//[<] 🤖🤖
