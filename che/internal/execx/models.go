package execx

// [>] 🤖🤖🤖

import (
	"io"
)

type Cmd struct {
	Argv   []string
	Dir    string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

type Real struct{}

type Mock struct {
	Stub  func(argv []string) ([]byte, error)
	calls []string
}

// [<] 🤖🤖🤖
