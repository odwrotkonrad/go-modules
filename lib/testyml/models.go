package testyml

// [>] 🤖🤖

import (
	"gopkg.in/yaml.v3"
)

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

type Args []arg

type Expected[W any] struct {
	Output       W              `yaml:"output"`
	ErrorOutput  Matchers       `yaml:"errorOutput"`
	StdOut       Matchers       `yaml:"stdOut"`
	StdOutCounts map[string]int `yaml:"stdOutCounts"`
	StdErr       Matchers       `yaml:"stdErr"`
	ExitCode     int            `yaml:"exitCode"`
	Files        string         `yaml:"files"`
}

type Case[W any] struct {
	Name        string      `yaml:"name"`
	Context     Context     `yaml:"context"`
	Input       Input       `yaml:"input"`
	Expected    Expected[W] `yaml:"expected"`
	NotExpected Expected[W] `yaml:"notExpected"`
}

type Matchers []string

// [<] 🤖🤖
