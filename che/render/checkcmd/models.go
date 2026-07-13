package checkcmd

// TODO: consider redesigning the data model for these types now that they're consolidated in one place.

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
