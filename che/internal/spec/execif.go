package spec

// [>] 🤖🤖

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
)

// NewEvaluator wires the builtin sources (isOs (macos|linux|...), isVirt) and
// the env: lookup, which reads the injected launch env.
func NewEvaluator(lookupEnv func(string) string) *Evaluator {
	return &Evaluator{
		builtins: map[string]func() string{
			"isOs":   sync.OnceValue(func() string { return fsutil.NormalizeOS(runtime.GOOS) }),
			"isVirt": sync.OnceValue(func() string { return strconv.FormatBool(fsutil.IsVirtualized()) }),
		},
		lookupEnv: lookupEnv,
	}
}

// EvalExecIf evaluates one expression: `<source>` (truthy: builtin iff "true",
// env iff set non-empty) or `<source> == <literal>` (string compare, builtins
// render as true/false).
func (e *Evaluator) EvalExecIf(expr string) (bool, error) {
	parts := strings.Split(expr, "==")
	switch len(parts) {
	case 1:
		_, truthy, err := e.resolve(strings.TrimSpace(parts[0]))
		return truthy, err
	case 2:
		val, _, err := e.resolve(strings.TrimSpace(parts[0]))
		if err != nil {
			return false, err
		}
		return val == strings.TrimSpace(parts[1]), nil
	default:
		return false, fmt.Errorf("malformed expression: want <source> or <source> == <literal>")
	}
}

// resolve maps a source to its value plus its bare-form truthiness.
func (e *Evaluator) resolve(src string) (string, bool, error) {
	switch {
	case strings.HasPrefix(src, "builtin:"):
		fn, ok := e.builtins[strings.TrimPrefix(src, "builtin:")]
		if !ok {
			return "", false, fmt.Errorf("unknown builtin %q (defined: isOs, isVirt)", src)
		}
		v := fn()
		return v, v == "true", nil
	case strings.HasPrefix(src, "env:"):
		v := e.lookupEnv(strings.TrimPrefix(src, "env:"))
		return v, v != "", nil
	default:
		return "", false, fmt.Errorf("unknown source %q: want builtin:<name> or env:<NAME>", src)
	}
}

// [<] 🤖🤖
