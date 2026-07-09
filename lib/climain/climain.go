// [>] 🤖🤖
package climain

import (
	"errors"
	"fmt"
	"os"

	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

func HelpVersion(args []string, usage, name, version string) (out string, done bool) {
	if len(args) != 1 {
		return "", false
	}
	switch args[0] {
	case "-h", "--help":
		return usage, true
	case "-v", "--version":
		return name + " version " + version, true
	}
	return "", false
}

func Exit(out string, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	} else {
		fmt.Println(out)
	}
	os.Exit(exitCode(err))
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ce *yamlcfg.CodedError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return 1
}

//[<] 🤖🤖
