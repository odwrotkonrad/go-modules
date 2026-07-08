// [>] 🤖🤖
package main

import (
	"fmt"
	"os"

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
	Usage:    usage,
	Label:    "render-dirs-tree",
	CheckArg: ".",
	Generate: func(string) (string, error) { return render.DirsTree(".") },
}

func main() {
	args := os.Args[1:]
	if len(args) == 1 && (args[0] == "-v" || args[0] == "--version") {
		fmt.Println("render-dirs-tree version", version)
		return
	}
	os.Exit(tool.Run(args))
}

//[<] 🤖🤖
