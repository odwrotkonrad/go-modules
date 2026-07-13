package testutil

// TODO: consider redesigning the data model for these types now that they're consolidated in one place.

// [>] 🤖🤖🤖

import (
	"os/user"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
)

// MockSet is the full safe-double set for a command harness: the executor is
// already swapped in, the host-scoped doubles wire via WithFS and co.
type MockSet struct {
	Exec   *CmdMockExecutor
	FS     *FileSystemMockWriter
	Reader *FileSystemMockReader
}

// CmdMockExecutor is the execx.CmdExecutor test double: records every call,
// nothing spawns, models launchd state and the source-checkout git CLI.
type CmdMockExecutor struct {
	execx.Mock
	Fail           bool     // every call fails
	FailCmds       []string // substring-matched commands that fail
	Out            string   // canned output body
	NotLoaded      bool     // launchd initial state: service not loaded
	NoPid          bool     // launchd print reports a pid-less service
	StubbornPrints int      // prints still reporting present after bootout
	Bodies         []string // captured install file bodies

	loaded *bool             // launchd state, lazily seeded from NotLoaded
	clones map[string]string // source clone dir -> source url
}

// FileSystemMockWriter is a record-only fsutil.FileSystemWriter: every call
// appends one formatted line, nothing touches the filesystem, nothing prints.
type FileSystemMockWriter struct {
	calls []string
}

// FileSystemMockReader is the fsutil.FileSystemReader test double: reads pass
// through to the live filesystem only under Roots (the test fixture repo +
// HOME), Files serves Stat/ReadFile from a path->content map, every other
// path reports absent. The zero value denies all reads, so live host state
// (/etc, /Library, ...) never leaks into test results.
type FileSystemMockReader struct {
	Roots []string
	Files map[string]string
}

// UserMockLookup is the fsutil.UserLookup test double: users served from a
// map, unknown names error. The zero value knows no one.
type UserMockLookup map[string]user.User

// GroupMockLookup is the fsutil.GroupLookup test double: groups served from a
// map, unknown names error. The zero value knows no group.
type GroupMockLookup map[string]user.Group

// RemoteMockFetcher is the host.RemoteFetcher test double: ref -> content,
// no git, unknown refs error.
type RemoteMockFetcher map[string]string

// [<] 🤖🤖🤖
