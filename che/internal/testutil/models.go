package testutil

// [>] 🤖🤖🤖

import (
	"os/user"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
)

type MockSet struct {
	Exec   *CmdMockExecutor
	FS     *FileSystemMockWriter
	Reader *FileSystemMockReader
}

type CmdMockExecutor struct {
	execx.Mock
	Fail     bool
	FailCmds []string
	Out      string
	Bodies   []string

	clones map[string]string
}

type FileSystemMockWriter struct {
	calls []string
}

type FileSystemMockReader struct {
	Roots []string
	Files map[string]string
}

type UserMockLookup map[string]user.User

type GroupMockLookup map[string]user.Group

type RemoteMockFetcher map[string]string

// [<] 🤖🤖🤖
