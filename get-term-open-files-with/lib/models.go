package lib

// [>] 🤖🤖

type OpenerRule struct {
	Opener string   `yaml:"opener"`
	Types  []string `yaml:"types"`
}

type TerminalRules map[string][]OpenerRule

type language struct {
	Type       string   `yaml:"type"`
	Extensions []string `yaml:"extensions"`
}

// [<] 🤖🤖
