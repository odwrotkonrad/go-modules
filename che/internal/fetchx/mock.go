package fetchx

// [>] 🤖🤖

import (
	"fmt"
	"os"
)

type Mock struct {
	Bodies map[string][]byte
	Err    error
	calls  []string
}

func (m *Mock) body(url string) ([]byte, error) {
	m.calls = append(m.calls, url)
	if m.Err != nil {
		return nil, fmt.Errorf("mock fetch %s: %w", url, m.Err)
	}
	if body, ok := m.Bodies[url]; ok {
		return body, nil
	}
	return []byte("asset"), nil
}

func (m *Mock) Download(url, dest string) error {
	body, err := m.body(url)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, body, 0o644)
}

func (m *Mock) Fetch(url string) ([]byte, error) { return m.body(url) }

func (m *Mock) Calls() []string { return m.calls }

// [<] 🤖🤖
