package che

// [>] 🤖🤖

import (
	"os"
	"strings"
)

// appCtx is the launch context: the process world captured once at the CLI
// boundary. It is the ONLY sanctioned ambient-read site; downstream methods
// read receiver fields, not the process.
type appCtx struct {
	Env  map[string]string // CHE_*/HOME/SUDO_USER/execIf env, snapshot at entry
	Cwd  string            // replaces os.Getwd
	Euid int               // replaces os.Geteuid
}

// NewContext snapshots the process launch world (os.Environ -> map, os.Getwd,
// os.Geteuid).
func NewContext() (appCtx, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return appCtx{}, err
	}
	return appCtx{Env: environMap(), Cwd: cwd, Euid: os.Geteuid()}, nil
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
// seam handed to options.Resolve / spec.NewEvaluator (primitives, not appCtx).
func (c appCtx) lookupEnv() func(string) string {
	return func(k string) string { return c.Env[k] }
}

// [<] 🤖🤖
