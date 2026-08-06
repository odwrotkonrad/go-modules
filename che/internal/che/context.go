package che

// [>] 🤖🤖

import (
	"context"
	"crypto/rand"
	"os"
	"strings"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/telemetry"
)

// Context is the launch context: the process world captured once at the CLI
// boundary. It is the ONLY sanctioned ambient-read site; downstream methods
// read receiver fields, not the process.
type Context struct {
	Env     map[string]string // CHE_*/HOME/SUDO_USER/runIf env, snapshot at entry
	Cwd     string            // replaces os.Getwd
	Euid    int               // replaces os.Geteuid
	RunID   string            // one 12-char base36 id per invocation: ledger run key + restore --run-id selector
	RunTs   string            // one TsLayout stamp per invocation: backup archive filenames
	Command string            // the invoked subcommand (ledger SpecDone.Command), set at the CLI boundary
	// Tel is the OTLP telemetry handle started at the CLI boundary after options
	// resolve; nil (tests, disabled) makes every counter/log call a no-op.
	Tel *telemetry.Telemetry
	// RunCtx is the run root span ctx, opened at the CLI boundary; prepare/exec
	// spans parent onto it. nil (tests) -> spans fall back to Background.
	RunCtx context.Context
}

// runContext returns the run root span ctx, or Background when unset (tests).
func (c Context) runContext() context.Context {
	if c.RunCtx != nil {
		return c.RunCtx
	}
	return context.Background()
}

// NewContext snapshots the process launch world (os.Environ -> map, os.Getwd,
// os.Geteuid) and stamps the per-run id (ledger run key) plus the run's
// TsLayout stamp (backup archive filenames).
func NewContext() (Context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Context{}, err
	}
	return Context{Env: environMap(), Cwd: cwd, Euid: os.Geteuid(), RunID: newID(), RunTs: time.Now().Format(fsutil.TsLayout)}, nil
}

// idAlphabet is the base36 token alphabet (run ids + backup ids).
const idAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// newID returns a 12-char base36 random token: one per run (Context.RunID) and
// one per backup archive filename.
func newID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	for i := range b {
		b[i] = idAlphabet[int(b[i])%len(idAlphabet)]
	}
	return string(b)
}

// environMap materializes os.Environ into a KEY -> value map.
func environMap() map[string]string {
	out := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		out[k] = v
	}
	return out
}

// lookupEnv returns an env lookupEnv func over the captured env, the cross-package
// seam handed to options.Resolve / spec.NewEvaluator (primitives, not Context).
func (c Context) lookupEnv() func(string) string {
	return func(k string) string { return c.Env[k] }
}

// [<] 🤖🤖
