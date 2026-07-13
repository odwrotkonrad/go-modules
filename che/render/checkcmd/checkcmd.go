// Package checkcmd implements the render CLIs' --check mode: render aside, diff against the dest, non-zero on drift.
package checkcmd

// [>] 🤖🤖

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/lib/climain"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

const (
	codeRuntime = 21
	codeDrift   = 22
)

func (t Tool) Main() { climain.RunRaw(t.Run) }

func (t Tool) Run(args []string) (string, error) {
	usage := strings.TrimSuffix(t.Usage, "\n")
	if out, done := climain.HelpVersion(args, usage, t.Name, t.Version); done {
		return out + "\n", nil
	}
	switch {
	case t.FlagArg != "" && len(args) == 2 && args[0] == t.FlagArg:
		return t.generate(args[1])
	case t.FlagArg == "" && len(args) == 2 && args[0] == "--check":
		return "", t.check(args[1])
	case t.FlagArg == "" && t.NeedsArg && len(args) == 1 && args[0] != "" && args[0][0] != '-':
		return t.generate(args[0])
	case t.FlagArg == "" && !t.NeedsArg && len(args) == 0:
		return t.generate("")
	default:
		return "", &yamlcfg.CodedError{Code: yamlcfg.CodeArgs, Msg: fmt.Sprintf("invalid arguments: %v\n\n%s", args, usage)}
	}
}

func (t Tool) generate(arg string) (string, error) {
	out, err := t.Generate(arg)
	if err != nil {
		return "", coded(err)
	}
	return out, nil
}

func (t Tool) check(path string) error {
	want, err := os.ReadFile(path)
	if err != nil {
		return &yamlcfg.CodedError{Code: yamlcfg.CodeFileNotFound, Msg: "file not found: " + path}
	}
	got, err := t.Generate(t.CheckArg)
	if err != nil {
		return coded(err)
	}
	if got == string(want) {
		return nil
	}
	drift := diff(string(want), got, path, t.Label)
	return &yamlcfg.CodedError{Code: codeDrift, Msg: strings.TrimSuffix(drift, "\n")}
}

// coded passes coded errors through, wraps the rest as runtime failures.
func coded(err error) error {
	if _, ok := errors.AsType[*yamlcfg.CodedError](err); ok {
		return err
	}
	return &yamlcfg.CodedError{Code: codeRuntime, Msg: err.Error()}
}

func diff(want, got, wantLabel, gotLabel string) string {
	wf := writeTemp("check-want-*", want)
	gf := writeTemp("check-got-*", got)
	defer os.Remove(wf)
	defer os.Remove(gf)
	out, _ := execx.Default.Output(execx.Cmd{Argv: []string{
		"diff", "-u",
		"--label", wantLabel, "--label", gotLabel,
		wf, gf,
	}})
	return string(out)
}

func writeTemp(pattern, content string) string {
	f, _ := os.CreateTemp("", pattern)
	_, _ = f.WriteString(content)
	f.Close()
	return f.Name()
}

//[<] 🤖🤖
