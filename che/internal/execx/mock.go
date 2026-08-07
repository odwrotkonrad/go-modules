package execx

// [>] 🤖🤖🤖

import (
	"strings"
)

type Mock struct {
	Stub  func(argv []string) ([]byte, error)
	calls []string
	envs  [][]string
}

func (m *Mock) run(c Cmd) ([]byte, error) {
	m.calls = append(m.calls, strings.Join(c.Argv, " "))
	m.envs = append(m.envs, c.Env)
	if m.Stub == nil {
		return nil, nil
	}
	return m.Stub(c.Argv)
}

func (m *Mock) Exec(c Cmd) error {
	out, err := m.run(c)
	if c.Stdout != nil && len(out) > 0 {
		_, _ = c.Stdout.Write(out)
	}
	return err
}

func (m *Mock) Output(c Cmd) ([]byte, error) { return m.run(c) }

func (m *Mock) Calls() []string { return m.calls }

func (m *Mock) Envs() [][]string { return m.envs }

type swapT interface {
	Helper()
	Cleanup(func())
}

func Swap(t swapT, e CmdExecutor) {
	t.Helper()
	prev := Default
	Default = e
	t.Cleanup(func() { Default = prev })
}

// [<] 🤖🤖🤖
