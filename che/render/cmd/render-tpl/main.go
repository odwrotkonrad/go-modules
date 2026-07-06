// [>] 🤖🤖
package main

import (
	"fmt"
	"os"

	"gitlab.com/konradodwrot/go/render-files/render"
)

const usage = `usage: render-tpl -f <template>

Render <template> with the shared engine (gomplate built-ins + op:// secrets +
frontmatter/readBody + native generators), env vars visible via env.Getenv, to
stdout. Drop-in for 'gomplate -f'. Paths in frontmatter/readBody/renderDirsTree
resolve against the cwd.
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Print(usage)
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
