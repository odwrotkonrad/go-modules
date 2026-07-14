package execx

// [>] 🤖🤖🤖

import (
	"context"
	"io"
)

type Cmd struct {
	Ctx    context.Context
	Argv   []string
	Dir    string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

type Real struct{}

// [<] 🤖🤖🤖
