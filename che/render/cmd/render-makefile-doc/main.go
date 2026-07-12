// Render-makefile-doc generates the agents' Makefile doc from [genai-include] sections.
package main

// [>] 🤖🤖

import (
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
	Name:     "render-makefile-doc",
	Version:  version,
	Usage:    usage,
	Label:    "generated",
	NeedsArg: true,
	CheckArg: "Makefile",
	Generate: lib.Generate,
}

func main() {
	os.Exit(tool.Run(os.Args[1:]))
}

//[<] 🤖🤖
