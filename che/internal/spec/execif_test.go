package spec

// [>] 🤖🤖

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func stubEvaluator(os string, virt bool) *Evaluator {
	return &Evaluator{builtins: map[string]func() string{
		"isOs":   func() string { return os },
		"isVirt": func() string { return strconv.FormatBool(virt) },
	}}
}

func TestEvalExecIf(t *testing.T) {
	e := stubEvaluator("macos", false)
	testyml.Eq(t, td, "testdata/spec/eval_exec_if.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		return e.EvalExecIf(c.Input.Args.String(t, 0))
	})
}

// builtinsWant is exec_if_builtins' expected.output: the eval result plus the
// executor call count (isVirt must cache: one exec across two evals).
type builtinsWant struct {
	Value     bool `yaml:"value"`
	ExecCalls int  `yaml:"execCalls"`
}

// TestNewEvaluatorBuiltins drives the real NewEvaluator wiring against the
// mock executor and a deny-all reader, so live container markers never leak in.
func TestNewEvaluatorBuiltins(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/new_evaluator.test.spec.yml", func(t *testing.T, c testyml.Case[builtinsWant]) {
		m := testutil.ApplyMocks(t, c.Context.MockedInterfaces)
		m.Exec.Out = c.Input.Args.String(t, 1)
		m.Exec.Fail = c.Input.Args.Bool(t, 2)
		testyml.Swap(t, &fsutil.DetectReader, fsutil.FileSystemReader(m.Reader))
		e := NewEvaluator()
		expr := c.Input.Args.String(t, 0)
		for range 2 {
			got, err := e.EvalExecIf(expr)
			require.NoErrorf(t, err, "EvalExecIf(%q)", expr)
			assert.Equal(t, c.Expected.Output.Value, got, "EvalExecIf(%q)", expr)
		}
		assert.Len(t, m.Exec.Calls(), c.Expected.Output.ExecCalls, "builtin must cache")
	})
}

// [<] 🤖🤖
