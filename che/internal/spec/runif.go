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

func NewEvaluator(lookupEnv func(string) string) *Evaluator {
	return &Evaluator{
		builtins: map[string]func() string{
			"isOs":   sync.OnceValue(func() string { return fsutil.NormalizeOS(runtime.GOOS) }),
			"isVirt": sync.OnceValue(func() string { return strconv.FormatBool(fsutil.IsVirtualized()) }),
		},
		lookupEnv: lookupEnv,
	}
}

func (e *Evaluator) EvalRunIf(expr string) (bool, error) {
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

func (e *Evaluator) resolve(src string) (string, bool, error) {
	switch {
	case strings.HasPrefix(src, "builtin:"):
		fn, ok := e.builtins[strings.TrimPrefix(src, "builtin:")]
		if !ok {
			return "", false, fmt.Errorf("unknown builtin %q (defined: isOs, isVirt)", src)
		}
		value := fn()
		return value, value == "true", nil
	case strings.HasPrefix(src, "env:"):
		value := e.lookupEnv(strings.TrimPrefix(src, "env:"))
		return value, value != "", nil
	default:
		return "", false, fmt.Errorf("unknown source %q: want builtin:<name> or env:<NAME>", src)
	}
}

// [<] 🤖🤖
