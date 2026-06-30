// [>] 🤖🤖
package main

import (
	"errors"
	"fmt"
	"os"

	"gitlab.com/konradodwrot/go/cruft/get-term-open-files-with/lib"
)

const usage = `usage: get-term-open-files-with <any|vscode|kitty>

Print ` + "`<ext>=<opener>`" + ` zsh suffix-alias lines for the terminal from
term-open-files-with.yml (system + user, deep-merged) expanded against
GitHub-linguist language data (cached, fetched on first run).

Exit Codes:
  11 invalid arguments
  12 invalid config
  13 config not found
  14 linguist fetch failed
`

const configName = "term-open-files-with.yml"

var terminals = map[string]bool{"any": true, "vscode": true, "kitty": true}

func run(args []string, customDir, url string) (string, error) {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		return usage, nil
	}
	if len(args) != 1 || !terminals[args[0]] {
		return "", &lib.CodedError{Code: lib.CodeArgs, Msg: "invalid arguments: " + fmt.Sprint(args)}
	}
	terminal := args[0]
	var sections lib.Sections
	if err := lib.LoadConfig(configName, customDir, &sections); err != nil {
		return "", err
	}
	byType, err := lib.TypeExtensions(url)
	if err != nil {
		return "", err
	}
	return lib.Render(terminal, sections, byType), nil
}

func main() {
	out, err := run(os.Args[1:], "", lib.LanguagesURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		var ce *lib.CodedError
		if errors.As(err, &ce) {
			os.Exit(ce.Code)
		}
		os.Exit(1)
	}
	fmt.Println(out)
}

//[<] 🤖🤖
