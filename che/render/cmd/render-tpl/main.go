// Render-tpl renders one gomplate template to stdout or dests: op:// (1Password) and gcp:// (GCP Secret Manager) secrets, remoteFile inclusion, frontmatter, markdown transforms.
package main

// [>] 🤖🤖

import (
	"os"

	"gitlab.com/konradodwrot/go-modules/che/render/checkcmd"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

const usage = `usage: render-tpl -f <template>

Render <template> with the shared engine (gomplate built-ins + op:// (1Password)
and gcp:// (GCP Secret Manager) secrets + frontmatter/readBody + native
generators), env vars visible via env.Getenv, to
stdout. Drop-in for 'gomplate -f'. Paths in frontmatter/readBody/renderDirsTree
resolve against the cwd.
`

var version = "dev"

var tool = checkcmd.Tool{
	Name:    "render-tpl",
	Version: version,
	Usage:   usage,
	FlagArg: "-f",
	Generate: func(path string) (string, error) {
		src, err := os.ReadFile(path)
		if err != nil {
			return "", &yamlcfg.CodedError{Code: yamlcfg.CodeFileNotFound, Msg: "file not found: " + path}
		}
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		out, err := render.Exec(path, src, cwd)
		return string(out), err
	},
}

func main() { tool.Main() }

//[<] 🤖🤖
