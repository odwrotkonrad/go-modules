package spec

// [>] 🤖🤖

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func stubEvaluator(osName string, virt bool) *Evaluator {
	return &Evaluator{
		builtins: map[string]func() string{
			"isOs":   func() string { return osName },
			"isVirt": func() string { return strconv.FormatBool(virt) },
		},
		lookupEnv: os.Getenv,
		exec:      execx.Default,
	}
}

func TestEvalRunIf(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/eval_run_if.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		m := testutil.ApplyMocks(t, c.Context.MockedInterfaces)
		m.Exec.Fail = c.Input.Args.Bool(t, 1)
		return stubEvaluator("macos", false).EvalRunIf(c.Input.Args.String(t, 0))
	})
}

type cmdWant struct {
	Value     bool   `yaml:"value"`
	ExecCalls int    `yaml:"execCalls"`
	ExecCmd   string `yaml:"execCmd"`
}

func TestEvalRunIfCmdUncached(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/eval_run_if_cmd.test.spec.yml", func(t *testing.T, c testyml.Case[cmdWant]) {
		m := testutil.ApplyMocks(t, c.Context.MockedInterfaces)
		e := stubEvaluator("macos", false)
		expr := c.Input.Args.String(t, 0)
		for range 2 {
			got, err := e.EvalRunIf(expr)
			require.NoErrorf(t, err, "EvalRunIf(%q)", expr)
			assert.Equal(t, c.Expected.Output.Value, got, "EvalRunIf(%q)", expr)
		}
		assert.Len(t, m.Exec.Calls(), c.Expected.Output.ExecCalls, "cmd: must not cache")
		for _, call := range m.Exec.Calls() {
			assert.Equal(t, c.Expected.Output.ExecCmd, call)
		}
	})
}

type builtinsWant struct {
	Value     bool `yaml:"value"`
	ExecCalls int  `yaml:"execCalls"`
}

func TestNewEvaluatorBuiltins(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/new_evaluator.test.spec.yml", func(t *testing.T, c testyml.Case[builtinsWant]) {
		m := testutil.ApplyMocks(t, c.Context.MockedInterfaces)
		m.Exec.Out = c.Input.Args.String(t, 1)
		m.Exec.Fail = c.Input.Args.Bool(t, 2)
		testyml.Swap(t, &fsutil.DetectReader, fsutil.FileSystemReader(m.Reader))
		e := NewEvaluator(os.Getenv)
		expr := c.Input.Args.String(t, 0)
		for range 2 {
			got, err := e.EvalRunIf(expr)
			require.NoErrorf(t, err, "EvalRunIf(%q)", expr)
			assert.Equal(t, c.Expected.Output.Value, got, "EvalRunIf(%q)", expr)
		}
		assert.Len(t, m.Exec.Calls(), c.Expected.Output.ExecCalls, "builtin must cache")
	})
}

// [<] 🤖🤖
