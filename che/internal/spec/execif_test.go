package spec

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func stubEvaluator(os string, virt bool) *Evaluator {
	return &Evaluator{builtins: map[string]func() string{
		"isOs":   func() string { return os },
		"isVirt": func() string { return boolStr(virt) },
	}}
}

func TestEvalExecIf(t *testing.T) {
	type in struct {
		Env  map[string]string
		Args []string
	}
	type want struct {
		Value bool
		Error bool
	}
	type c struct {
		Name string
		In   in
		Want want
	}
	e := stubEvaluator("macos", false)
	testyml.Run(t, td, "testdata/spec/eval_exec_if.spec.yml", func(t *testing.T, c c) {
		for k, v := range c.In.Env {
			t.Setenv(k, v)
		}
		expr := c.In.Args[0]
		got, err := e.EvalExecIf(expr)
		if c.Want.Error {
			if err == nil {
				t.Fatalf("EvalExecIf(%q) = %v, want error", expr, got)
			}
			return
		}
		if err != nil {
			t.Fatalf("EvalExecIf(%q) errored: %v", expr, err)
		}
		if got != c.Want.Value {
			t.Errorf("EvalExecIf(%q) = %v, want %v", expr, got, c.Want.Value)
		}
	})
}

// [<] 🤖🤖
