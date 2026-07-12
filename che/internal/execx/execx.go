// Package execx runs external commands behind a swappable executor, mockable in tests.
package execx

// [>] 🤖🤖🤖

import (
	"io"
	"os/exec"
	"strings"
	"testing"
)

type Cmd struct {
	Argv   []string
	Dir    string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

type CmdExecutor interface {
	Exec(c Cmd) error
	Output(c Cmd) ([]byte, error)
}

var Default CmdExecutor = Real{}

type Real struct{}

// command maps a Cmd onto an exec.Cmd, Stdout left unset (Output owns it).
func (c Cmd) command() *exec.Cmd {
	cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
	cmd.Dir, cmd.Env, cmd.Stderr = c.Dir, c.Env, c.Stderr
	return cmd
}

func (Real) Exec(c Cmd) error {
	cmd := c.command()
	cmd.Stdout = c.Stdout
	return cmd.Run()
}

func (Real) Output(c Cmd) ([]byte, error) {
	return c.command().Output()
}

type Mock struct {
	Stub  func(argv []string) ([]byte, error)
	calls []string
}

func (m *Mock) run(argv []string) ([]byte, error) {
	m.calls = append(m.calls, strings.Join(argv, " "))
	if m.Stub == nil {
		return nil, nil
	}
	return m.Stub(argv)
}

func (m *Mock) Exec(c Cmd) error {
	out, err := m.run(c.Argv)
	if c.Stdout != nil && len(out) > 0 {
		_, _ = c.Stdout.Write(out)
	}
	return err
}

func (m *Mock) Output(c Cmd) ([]byte, error) { return m.run(c.Argv) }

func (m *Mock) Calls() []string { return m.calls }

func Swap(t testing.TB, e CmdExecutor) {
	t.Helper()
	prev := Default
	Default = e
	t.Cleanup(func() { Default = prev })
}

// [<] 🤖🤖🤖
