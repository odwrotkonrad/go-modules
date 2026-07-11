package testutil

// [>] 🤖🤖

import "fmt"

// MockFetcher is a record-free host.RemoteFetcher test double: ref -> content,
// no git, unknown refs error.
type MockFetcher map[string]string

func (m MockFetcher) Fetch(ref string) (string, error) {
	content, ok := m[ref]
	if !ok {
		return "", fmt.Errorf("MockFetcher: no fixture for %q", ref)
	}
	return content, nil
}

// [<] 🤖🤖
