// [>] 🤖🤖
package main

import (
	"fmt"
	"os"

	"gitlab.com/konradodwrot/go-modules/che/render/checkcmd"
	"gitlab.com/konradodwrot/go-modules/che/render/lib"
)

var version = "dev"

const usage = `usage: render-makefile-doc <makefile-path>
       render-makefile-doc --check <doc-file>

Emit makefile.agents.md from a Makefile's [genai-include] sections (stdout).
--check regenerates from ./Makefile and diffs against <doc-file>:
exit 0 match, 22 differ (unified diff on stderr).
`

var tool = checkcmd.Tool{
	Usage:    usage,
	Label:    "generated",
	NeedsArg: true,
	CheckArg: "Makefile",
	Generate: lib.Generate,
}

func main() {
	args := os.Args[1:]
	if len(args) == 1 && (args[0] == "-v" || args[0] == "--version") {
		fmt.Println("render-makefile-doc version", version)
		return
	}
	os.Exit(tool.Run(args))
}

//[<] 🤖🤖
