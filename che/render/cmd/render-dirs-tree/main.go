// [>] 🤖🤖
package main

import (
	"os"

	"gitlab.com/konradodwrot/go/render-files/checkcmd"
)

const usage = `usage: render-dirs-tree
       render-dirs-tree --check <file>

Print the plain directory tree of the cwd repo's tracked files (stdout):
read tracked paths from the git index, drop each file leaf, nest and sort
the remaining dirs, 2-space indented, one dir per line.
--check regenerates and diffs against <file>:
exit 0 match, 22 differ (unified diff on stderr).
`

var tool = checkcmd.Tool{
	Usage:    usage,
	Label:    "render-dirs-tree",
	CheckArg: ".",
	Generate: func(string) (string, error) { return Generate(".") },
}

func main() {
	os.Exit(tool.Run(os.Args[1:]))
}

//[<] 🤖🤖
