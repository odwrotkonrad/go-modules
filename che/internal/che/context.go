package che

// [>] 🤖🤖

import (
	"context"
	"crypto/rand"
	"os"
	"strings"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/telemetry"
)

type Context struct {
	Env        map[string]string
	ProcessEnv map[string]string
	RepoFiles  spec.RepoFiles
	Cwd        string
	Euid       int
	RunID      string
	RunTs      string
	Command    string
	Tel        *telemetry.Telemetry
	RunCtx     context.Context
}

func (c Context) runContext() context.Context {
	if c.RunCtx != nil {
		return c.RunCtx
	}
	return context.Background()
}

func NewContext() (Context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Context{}, err
	}
	return Context{Env: environMap(), Cwd: cwd, Euid: os.Geteuid(), RunID: newID(), RunTs: time.Now().Format(fsutil.TsLayout)}, nil
}

const idAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

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

func environMap() map[string]string {
	out := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		out[k] = v
	}
	return out
}

func (c Context) lookupEnv() func(string) string {
	return func(k string) string { return c.Env[k] }
}

// [<] 🤖🤖
