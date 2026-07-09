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
	type in struct {
		Tree string
		Env  map[string]string
		Args []string
	}
	type c struct {
		Name string
		In   in
		Want testyml.Want
	}
	testyml.Run(t, td, "testdata/spec/run.spec.yml", func(t *testing.T, c c) {
		dir := t.TempDir()
		if c.In.Tree != "" {
			testyml.CopyDir(t, td, "testdata/fixture/run/"+c.In.Tree, dir)
		}
		t.Chdir(dir)
		for k, v := range c.In.Env {
			t.Setenv(k, v)
		}
		args := make([]string, len(c.In.Args))
		for i, a := range c.In.Args {
			args[i] = testyml.Expand(a, map[string]string{"DIR": dir})
		}
		c.Want.CheckCode(t, run(args))
	})
}

// [<] 🤖🤖
