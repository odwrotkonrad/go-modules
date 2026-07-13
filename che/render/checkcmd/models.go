package checkcmd

// [>] 🤖🤖

type Tool struct {
	Name     string
	Version  string
	Usage    string
	Label    string
	NeedsArg bool
	FlagArg  string
	CheckArg string
	Generate func(arg string) (string, error)
}

// [<] 🤖🤖
