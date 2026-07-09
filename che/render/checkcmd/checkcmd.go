// [>] 🤖🤖
package checkcmd

import (
	"fmt"
	"os"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
)

type Tool struct {
	Name     string
	Version  string
	Usage    string
	Label    string
	NeedsArg bool
	CheckArg string
	Generate func(arg string) (string, error)
}

func (t Tool) Run(args []string) int {
	switch {
	case len(args) == 1 && (args[0] == "-h" || args[0] == "--help"):
		fmt.Print(t.Usage)
		return 0
	case len(args) == 1 && (args[0] == "-v" || args[0] == "--version"):
		fmt.Println(t.Name, "version", t.Version)
		return 0
	case len(args) == 2 && args[0] == "--check":
		return t.check(args[1])
	case t.NeedsArg && len(args) == 1 && args[0] != "" && args[0][0] != '-':
		return t.emit(args[0])
	case !t.NeedsArg && len(args) == 0:
		return t.emit("")
	default:
		fmt.Fprintf(os.Stderr, "invalid arguments: %v\n\n%s", args, t.Usage)
		return 11
	}
}

func (t Tool) emit(arg string) int {
	out, err := t.Generate(arg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 21
	}
	fmt.Print(out)
	return 0
}

func (t Tool) check(path string) int {
	want, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "file not found: %s\n", path)
		return 13
	}
	got, err := t.Generate(t.CheckArg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 21
	}
	if got == string(want) {
		return 0
	}
	fmt.Fprint(os.Stderr, diff(string(want), got, path, t.Label))
	return 22
}

func diff(want, got, wantLabel, gotLabel string) string {
	wf, _ := os.CreateTemp("", "check-want-*")
	gf, _ := os.CreateTemp("", "check-got-*")
	defer os.Remove(wf.Name())
	defer os.Remove(gf.Name())
	wf.WriteString(want)
	gf.WriteString(got)
	wf.Close()
	gf.Close()
	out, _ := execx.Default.Output(execx.Cmd{Argv: []string{
		"diff", "-u",
		"--label", wantLabel, "--label", gotLabel,
		wf.Name(), gf.Name(),
	}})
	return string(out)
}

//[<] 🤖🤖
