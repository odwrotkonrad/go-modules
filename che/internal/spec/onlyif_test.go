package spec

// [>] 🤖🤖

import (
	"testing"
)

// stubEvaluator returns an Evaluator with fixed builtin values.
func stubEvaluator(os string, virt bool) *Evaluator {
	return &Evaluator{builtins: map[string]func() string{
		"isOs":   func() string { return os },
		"isVirt": func() string { return boolStr(virt) },
	}}
}

func TestEvalOnlyIf(t *testing.T) {
	t.Setenv("CHE_TEST_SET", "grp")
	t.Setenv("CHE_TEST_EMPTY", "")
	e := stubEvaluator("macos", false)
	cases := []struct {
		expr string
		want bool
	}{
		{"env:CHE_TEST_SET", true},
		{"env:CHE_TEST_EMPTY", false},
		{"env:CHE_TEST_UNSET", false},
		{"env:CHE_TEST_SET == grp", true},
		{"env:CHE_TEST_SET == other", false},
		{"env:CHE_TEST_UNSET == ", true},
		{"builtin:isOs == macos", true},
		{"builtin:isOs == linux", false},
		{"builtin:isOs==macos", true},
		{"builtin:isVirt", false},
		{"builtin:isVirt == false", true},
	}
	for _, c := range cases {
		got, err := e.EvalOnlyIf(c.expr)
		if err != nil {
			t.Errorf("EvalOnlyIf(%q) errored: %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("EvalOnlyIf(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestEvalOnlyIfErrors(t *testing.T) {
	e := stubEvaluator("macos", false)
	for _, expr := range []string{
		"builtin:isWindows",
		"host:isOs",
		"GITLAB_GROUP",
		"a == b == c",
	} {
		if got, err := e.EvalOnlyIf(expr); err == nil {
			t.Errorf("EvalOnlyIf(%q) = %v, want error", expr, got)
		}
	}
}

// [<] 🤖🤖
