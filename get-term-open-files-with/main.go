// [>] 🤖🤖
package main

import (
	"fmt"

	"gitlab.com/konradodwrot/go-modules/get-term-open-files-with/lib"
	"gitlab.com/konradodwrot/go-modules/lib/climain"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
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

var version = "dev"

func run(args []string, customDir, url string) (string, error) {
	if len(args) != 1 || !terminals[args[0]] {
		return "", &yamlcfg.CodedError{Code: yamlcfg.CodeArgs, Msg: "invalid arguments: " + fmt.Sprint(args)}
	}
	terminal := args[0]
	var sections lib.Sections
	if err := yamlcfg.LoadConfig(configName, customDir, &sections); err != nil {
		return "", err
	}
	byType, err := lib.TypeExtensions(url)
	if err != nil {
		return "", err
	}
	return lib.Render(terminal, sections, byType), nil
}

func main() {
	climain.Run("get-term-open-files-with", version, usage, func(args []string) (string, error) {
		return run(args, "", lib.LanguagesURL)
	})
}

//[<] 🤖🤖
