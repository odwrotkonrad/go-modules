package testyml

// [>] 🤖🤖

import (
	"gopkg.in/yaml.v3"
)

// Domain model (the spec-case schema):
//
//	Case
//	  Context   the unit under test (function or command) + env, pwd, mocks;
//	            file-level context deep-merges under each case's (case wins)
//	  Input     Args: positional values, optionally named via single-key maps
//	  Expected / NotExpected
//	    Output       typed want (W)
//	    Matchers     errorOutput/stdOut/stdErr: literal text with {{/regex/}} holes
//	    exitCode, files

// Context: the unit under test (function or command) plus the world around
// it. File-level context deep-merges under each case's own (case wins).
type Context struct {
	Function         string            `yaml:"function"`
	Command          string            `yaml:"command"`
	Pwd              string            `yaml:"pwd"`
	Env              map[string]string `yaml:"env"`
	MockedInterfaces map[string]string `yaml:"mockedInterfaces"`
}

type Input struct {
	Args Args `yaml:"args"`
}

type arg struct {
	name string
	node yaml.Node
}

// Args is the case argument list: bare values or single-key maps naming the
// argument. Names are for the reader, extraction is positional and typed.
type Args []arg

// Expected is the canonical expectation set: function tests use
// output/errorOutput, command tests use stdOut/stdErr/exitCode/files.
type Expected[W any] struct {
	Output      W        `yaml:"output"`
	ErrorOutput Matchers `yaml:"errorOutput"`
	StdOut      Matchers `yaml:"stdOut"`
	StdErr      Matchers `yaml:"stdErr"`
	ExitCode    int      `yaml:"exitCode"`
	Files       string   `yaml:"files"`
}

type Case[W any] struct {
	Name        string      `yaml:"name"`
	Context     Context     `yaml:"context"`
	Input       Input       `yaml:"input"`
	Expected    Expected[W] `yaml:"expected"`
	NotExpected Expected[W] `yaml:"notExpected"`
}

// Matchers is a list of output matchers: literal text with optional
// {{/regex/}} holes.
type Matchers []string

// [<] 🤖🤖
