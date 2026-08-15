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

var Default CmdExecutor = OS{}

func (c Cmd) command() *exec.Cmd {
	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, c.Argv[0], c.Argv[1:]...)
	cmd.Dir, cmd.Env, cmd.Stderr = c.Dir, c.Env, c.Stderr
	return cmd
}

func (OS) Exec(c Cmd) error {
	cmd := c.command()
	cmd.Stdout = c.Stdout
	return cmd.Run()
}

func (OS) Output(c Cmd) ([]byte, error) {
	return c.command().Output()
}

// [<] 🤖🤖🤖
