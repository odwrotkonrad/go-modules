package execx

// TODO: consider redesigning the data model for these types now that they're consolidated in one place.

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
