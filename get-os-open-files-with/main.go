// [>] 🤖🤖
package main

import (
	"errors"
	"fmt"
	"os"

	"gitlab.com/konradodwrot/go/cruft/get-os-open-files-with/lib"
)

const usage = `usage: get-os-open-files-with

Print ` + "`<bundle> <uti> <role>`" + ` file-handler association lines from
os-open-files-with.yml (system + user, deep-merged), in config order.

Exit Codes:
  11 invalid arguments
  12 invalid config
  13 config not found
`

const configName = "os-open-files-with.yml"

func run(args []string, customDir string) (string, error) {
	switch {
	case len(args) == 1 && (args[0] == "--help" || args[0] == "-h"):
		return usage, nil
	case len(args) == 0:
		node, err := lib.LoadConfigNode(configName, customDir)
		if err != nil {
			return "", err
		}
		return lib.Render(node)
	default:
		return "", &lib.CodedError{Code: lib.CodeArgs, Msg: "invalid arguments: " + fmt.Sprint(args)}
	}
}

func main() {
	out, err := run(os.Args[1:], "")
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
