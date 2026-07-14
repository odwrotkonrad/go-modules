// Package execx runs external commands behind a swappable executor, mockable in tests.
package execx

// [>] 🤖🤖🤖

import (
	"context"
	"os/exec"
)

type CmdExecutor interface {
	Exec(c Cmd) error
	Output(c Cmd) ([]byte, error)
}

var Default CmdExecutor = Real{}

// command maps a Cmd onto an exec.Cmd, Stdout left unset (Output owns it).
// Cmd.Ctx (nil -> Background) binds the child process's lifetime, so a cancelled
// ctx kills it.
func (c Cmd) command() *exec.Cmd {
	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, c.Argv[0], c.Argv[1:]...)
	cmd.Dir, cmd.Env, cmd.Stderr = c.Dir, c.Env, c.Stderr
	return cmd
}

func (Real) Exec(c Cmd) error {
	cmd := c.command()
	cmd.Stdout = c.Stdout
	return cmd.Run()
}

func (Real) Output(c Cmd) ([]byte, error) {
	return c.command().Output()
}

// [<] 🤖🤖🤖
