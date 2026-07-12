package main

// [>] 🤖🤖

import (
	"embed"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func TestRun(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/render-tpl.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		dir := t.TempDir()
		if c.Context.Pwd != "" {
			testyml.CopyDir(t, td, c.Context.Pwd, dir)
		}
		t.Chdir(dir)
		for k, v := range c.Context.Env {
			t.Setenv(k, v)
		}
		args := c.Context.CommandArgs()
		for i, a := range args {
			args[i] = testyml.Expand(a, map[string]string{"DIR": dir})
		}
		_, err := tool.Run(args)
		c.Expected.Check(t, err)
	})
}

// [<] 🤖🤖
