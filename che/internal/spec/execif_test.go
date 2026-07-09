package spec

// [>] 🤖🤖

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func stubEvaluator(os string, virt bool) *Evaluator {
	return &Evaluator{builtins: map[string]func() string{
		"isOs":   func() string { return os },
		"isVirt": func() string { return strconv.FormatBool(virt) },
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

// failReader is a fsutil.FileSystemReader whose every read fails, so live
// container markers (/.dockerenv, /proc/1/cgroup) never leak into results.
type failReader struct{}

func (failReader) Stat(string) (os.FileInfo, error)      { return nil, fs.ErrNotExist }
func (failReader) Lstat(string) (os.FileInfo, error)     { return nil, fs.ErrNotExist }
func (failReader) ReadDir(string) ([]os.DirEntry, error) { return nil, fs.ErrNotExist }
func (failReader) ReadFile(string) ([]byte, error)       { return nil, fs.ErrNotExist }
func (failReader) Readlink(string) (string, error)       { return "", fs.ErrNotExist }
func (failReader) EvalSymlinks(string) (string, error)   { return "", fs.ErrNotExist }

// TestNewEvaluatorBuiltins drives the real NewEvaluator wiring against a mock
// executor: isVirt resolves from the mocked detect command and caches (one
// exec across two evals).
func TestNewEvaluatorBuiltins(t *testing.T) {
	type in struct {
		Args     []string
		ExecOut  string `yaml:"execOut"`
		ExecFail bool   `yaml:"execFail"`
	}
	type want struct {
		Value     bool
		ExecCalls int `yaml:"execCalls"`
	}
	type c struct {
		Name string
		In   in
		Want want
	}
	testyml.Run(t, td, "testdata/spec/exec_if_builtins.spec.yml", func(t *testing.T, c c) {
		m := &execx.Mock{Stub: func(argv []string) ([]byte, error) {
			if c.In.ExecFail {
				return nil, errors.New("stub: detect fail")
			}
			return []byte(c.In.ExecOut), nil
		}}
		execx.Swap(t, m)
		prev := fsutil.DetectReader
		fsutil.DetectReader = failReader{}
		t.Cleanup(func() { fsutil.DetectReader = prev })
		e := NewEvaluator()
		for range 2 {
			got, err := e.EvalExecIf(c.In.Args[0])
			if err != nil {
				t.Fatalf("EvalExecIf(%q) errored: %v", c.In.Args[0], err)
			}
			if got != c.Want.Value {
				t.Errorf("EvalExecIf(%q) = %v, want %v", c.In.Args[0], got, c.Want.Value)
			}
		}
		if len(m.Calls()) != c.Want.ExecCalls {
			t.Errorf("exec calls = %v, want %d (builtin must cache)", m.Calls(), c.Want.ExecCalls)
		}
	})
}

// [<] 🤖🤖
