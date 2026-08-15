package main

// [>] 🤖🤖

import (
	"gitlab.com/konradodwrot/go-modules/get-os-open-files-with/lib"
	"gitlab.com/konradodwrot/go-modules/lib/climain"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
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

var version = "dev"

func run(args []string, userDir string) (string, error) {
	if len(args) != 0 {
		return "", yamlcfg.ArgsError(args)
	}
	node, err := yamlcfg.LoadConfigNode(configName, userDir)
	if err != nil {
		return "", err
	}
	return lib.RenderDutiLines(node)
}

func main() {
	climain.RunLine("get-os-open-files-with", version, usage, func(args []string) (string, error) {
		return run(args, "")
	})
}

//[<] 🤖🤖
