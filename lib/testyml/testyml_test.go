package testyml

// [>] 🤖🤖

import (
	"embed"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

//go:embed all:testdata
var td embed.FS

func TestRun(t *testing.T) {
	ran := 0
	Run(t, td, "testdata/spec/funcs/run.test.spec.yml", func(t *testing.T, c Case[int]) {
		ran++
		sum := 0
		for i := range c.Input.Args {
			sum += c.Input.Args.Int(t, i)
		}
		assert.Equal(t, c.Expected.Output, sum)
	})
	assert.Equal(t, 3, ran, "every spec case must invoke the callback")
}

func TestRunContextMerge(t *testing.T) {
	type view struct {
		Function string            `yaml:"function"`
		Pwd      string            `yaml:"pwd"`
		Env      map[string]string `yaml:"env"`
		Mock     string            `yaml:"mock"`
	}
	Run(t, td, "testdata/spec/funcs/merge_case_context.test.spec.yml", func(t *testing.T, c Case[view]) {
		got := view{
			Function: c.Context.Function,
			Pwd:      c.Context.Pwd,
			Env:      c.Context.Env,
			Mock:     c.Context.MockedInterfaces["execx.CmdExecutor"],
		}
		assert.Equal(t, c.Expected.Output, got)
	})
}

func TestMatchers(t *testing.T) {
	Run(t, td, "testdata/spec/funcs/must_match.test.spec.yml", func(t *testing.T, c Case[struct{}]) {
		s := c.Input.Args.String(t, 0)
		for _, m := range c.Expected.StdOut {
			MustMatch(t, s, m)
		}
		for _, m := range c.NotExpected.StdOut {
			MustNotMatch(t, s, m)
		}
	})
}

func TestCheck(t *testing.T) {
	Run(t, td, "testdata/spec/funcs/expected_check.test.spec.yml", func(t *testing.T, c Case[struct{}]) {
		var err error
		if code := c.Input.Args.Int(t, 0); code != 0 {
			err = &yamlcfg.CodedError{Code: code, Msg: c.Input.Args.String(t, 1)}
		}
		assert.Equal(t, err != nil, c.Expected.Check(t, err))
	})
}

func TestEq(t *testing.T) {
	Eq(t, td, "testdata/spec/funcs/eq.test.spec.yml", func(t *testing.T, c Case[string]) (string, error) {
		if c.Input.Args.Bool(t, 1) {
			return "", errors.New("stub fail")
		}
		return strings.ToUpper(os.Expand(c.Input.Args.String(t, 0), os.Getenv)), nil
	})
}

func TestArgs(t *testing.T) {
	type view struct {
		Name0    string         `yaml:"name0"`
		Name1    string         `yaml:"name1"`
		Str0     string         `yaml:"str0"`
		Int1     int            `yaml:"int1"`
		Bool2    bool           `yaml:"bool2"`
		Strings3 []string       `yaml:"strings3"`
		Users4   map[string]int `yaml:"users4"`
		Str9     string         `yaml:"str9"`
		Bool9    bool           `yaml:"bool9"`
	}
	Run(t, td, "testdata/spec/funcs/args.test.spec.yml", func(t *testing.T, c Case[view]) {
		a := c.Input.Args
		got := view{
			Name0:    a.Name(0),
			Name1:    a.Name(1),
			Str0:     a.String(t, 0),
			Int1:     a.Int(t, 1),
			Bool2:    a.Bool(t, 2),
			Strings3: a.Strings(t, 3),
			Str9:     a.String(t, 9),
			Bool9:    a.Bool(t, 9),
		}
		a.To(t, 4, &got.Users4)
		assert.Equal(t, c.Expected.Output, got)
	})
}

func TestSwap(t *testing.T) {
	v := "before"
	Run(t, td, "testdata/spec/funcs/swap.test.spec.yml", func(t *testing.T, c Case[string]) {
		Swap(t, &v, c.Input.Args.String(t, 0))
		assert.Equal(t, c.Expected.Output, v)
	})
	assert.Equal(t, "before", v)
}

func TestMatch(t *testing.T) {
	Eq(t, td, "testdata/spec/funcs/is_match.test.spec.yml", func(t *testing.T, c Case[bool]) (bool, error) {
		return IsMatch(c.Input.Args.String(t, 0), c.Input.Args.String(t, 1)), nil
	})
}

func TestCommandArgs(t *testing.T) {
	Eq(t, td, "testdata/spec/funcs/command_args.test.spec.yml", func(t *testing.T, c Case[[]string]) ([]string, error) {
		return Context{Command: c.Input.Args.String(t, 0)}.CommandArgs(), nil
	})
}

func TestExpand(t *testing.T) {
	Eq(t, td, "testdata/spec/funcs/expand.test.spec.yml", func(t *testing.T, c Case[string]) (string, error) {
		var vars map[string]string
		c.Input.Args.To(t, 1, &vars)
		return Expand(c.Input.Args.String(t, 0), vars), nil
	})
}

func TestReadFileExpected(t *testing.T) {
	Run(t, td, "testdata/spec/funcs/read_file.test.spec.yml", func(t *testing.T, c Case[string]) {
		got := ReadFile(t, td, c.Input.Args.String(t, 0))
		assert.Equal(t, c.Expected.Output, got)
		EqualExpected(t, td, c.Input.Args.String(t, 0), got)
	})
}

func TestCopyDir(t *testing.T) {
	Run(t, td, "testdata/spec/funcs/copy_dir.test.spec.yml", func(t *testing.T, c Case[map[string]string]) {
		dest := t.TempDir()
		CopyDir(t, td, c.Input.Args.String(t, 0), dest)
		for rel, want := range c.Expected.Output {
			b, err := os.ReadFile(filepath.Join(dest, rel))
			require.NoError(t, err)
			assert.Equal(t, want, string(b), rel)
		}
	})
}

// [<] 🤖🤖
