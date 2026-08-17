package builtin

// [>] 🤖🤖

import (
	"embed"
	"strings"
)

//go:embed data/packages.yml
var Catalog []byte

//go:embed all:data/scripts
var scripts embed.FS

//go:embed data/version.txt
var version string

func Version() string { return strings.TrimSpace(version) }

func ReadScript(name string) ([]byte, error) { return scripts.ReadFile("data/scripts/" + name) }

// [<] 🤖🤖
