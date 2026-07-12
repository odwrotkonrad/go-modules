// Render-tpl renders one gomplate template to stdout or dests: op:// secrets, remoteFile inclusion, frontmatter, markdown transforms.
package main

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/render/render"
	"gitlab.com/konradodwrot/go-modules/lib/climain"
)

const usage = `usage: render-tpl -f <template>

Render <template> with the shared engine (gomplate built-ins + op:// secrets +
frontmatter/readBody + native generators), env vars visible via env.Getenv, to
stdout. Drop-in for 'gomplate -f'. Paths in frontmatter/readBody/renderDirsTree
resolve against the cwd.
`

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if out, done := climain.HelpVersion(args, strings.TrimSuffix(usage, "\n"), "render-tpl", version); done {
		fmt.Println(out)
		return 0
	}
	if len(args) != 2 || args[0] != "-f" {
		fmt.Fprintf(os.Stderr, "invalid arguments: %v\n\n%s", args, usage)
		return 11
	}
	path := args[1]
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "file not found: %s\n", path)
		return 13
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 21
	}
	out, err := render.Exec(path, src, cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 21
	}
	fmt.Print(string(out))
	return 0
}

//[<] 🤖🤖
