package che

// [>] 🤖🤖

import (
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
	Env     map[string]string // CHE_*/HOME/SUDO_USER/execIf env, snapshot at entry
	Cwd     string            // replaces os.Getwd
	Euid    int               // replaces os.Geteuid
	RunID   string            // one TsLayout stamp per invocation: ledger run + backup filenames
	Command string            // the invoked subcommand (ledger SpecDone.Command), set at the CLI boundary
	// Tel is the OTLP telemetry handle started at the CLI boundary after options
	// resolve; nil (tests, disabled) makes every counter/log call a no-op.
	Tel *telemetry.Telemetry
}

// NewContext snapshots the process launch world (os.Environ -> map, os.Getwd,
// os.Geteuid) and stamps the per-run id (backup filenames + ledger run key).
func NewContext() (Context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Context{}, err
	}
	return Context{Env: environMap(), Cwd: cwd, Euid: os.Geteuid(), RunID: time.Now().Format(fsutil.TsLayout)}, nil
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
