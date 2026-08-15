package chepackages

// [>] 🤖🤖

import "embed"

//go:embed packages.yml
var Builtin []byte

//go:embed all:scripts
var Scripts embed.FS

// [<] 🤖🤖
