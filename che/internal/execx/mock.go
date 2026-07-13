package execx

// [>] 🤖🤖🤖

import (
	"strings"
)

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

// swapT is the testing.TB subset Swap needs, kept local so this production file
// does not import testing.
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
