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
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

//go:embed all:testdata
var td embed.FS

func TestRun(t *testing.T) {
	ran := map[string]bool{}
	Run(t, td, "testdata/spec/run.test.spec.yml", func(t *testing.T, c Case[int]) {
		ran[c.Name] = true
		sum := 0
		for i := range c.Input.Args {
			sum += c.Input.Args.Int(t, i)
		}
		assert.Equal(t, c.Expected.Output, sum)
	})
	assert.True(t, ran["runShouldDecodeCaseAndInvokeCallback"])
	assert.True(t, ran["runShouldDecodeNegativeValues"])
	assert.True(t, ran["runShouldDecodeNamedArgs"])
}

func TestRunContextMerge(t *testing.T) {
	type view struct {
		Function string            `yaml:"function"`
		Pwd      string            `yaml:"pwd"`
		Env      map[string]string `yaml:"env"`
		Mock     string            `yaml:"mock"`
	}
	Run(t, td, "testdata/spec/merge_case_context.test.spec.yml", func(t *testing.T, c Case[view]) {
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
	Run(t, td, "testdata/spec/must_match.test.spec.yml", func(t *testing.T, c Case[struct{}]) {
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
	Run(t, td, "testdata/spec/expected_check.test.spec.yml", func(t *testing.T, c Case[struct{}]) {
		var err error
		if code := c.Input.Args.Int(t, 0); code != 0 {
			err = &yamlcfg.CodedError{Code: code, Msg: c.Input.Args.String(t, 1)}
		}
		assert.Equal(t, err != nil, c.Expected.Check(t, err))
	})
}

func TestEq(t *testing.T) {
	Eq(t, td, "testdata/spec/eq.test.spec.yml", func(t *testing.T, c Case[string]) (string, error) {
		if c.Input.Args.Bool(t, 1) {
			return "", errors.New("stub fail")
		}
		return strings.ToUpper(os.Expand(c.Input.Args.String(t, 0), os.Getenv)), nil
	})
}

func TestArgs(t *testing.T) {
	var in Input
	raw := `args: [pattern: "a/**", 7, flag: true, list: [x, y], users: {ko: 501}]`
	require.NoError(t, yaml.Unmarshal([]byte(raw), &in))
	assert.Equal(t, "pattern", in.Args.Name(0))
	assert.Equal(t, "", in.Args.Name(1))
	assert.Equal(t, "a/**", in.Args.String(t, 0))
	assert.Equal(t, 7, in.Args.Int(t, 1))
	assert.True(t, in.Args.Bool(t, 2))
	assert.Equal(t, []string{"x", "y"}, in.Args.Strings(t, 3))
	var users map[string]int
	in.Args.To(t, 4, &users)
	assert.Equal(t, map[string]int{"ko": 501}, users)
	assert.Equal(t, "", in.Args.String(t, 9))
	assert.False(t, in.Args.Bool(t, 9))
}

func TestSwap(t *testing.T) {
	v := "before"
	t.Run("inner", func(t *testing.T) {
		Swap(t, &v, "after")
		assert.Equal(t, "after", v)
	})
	assert.Equal(t, "before", v)
}

func TestMatch(t *testing.T) {
	assert.True(t, IsMatch("ln(create): /etc/zshrc", "ln(create)"))
	assert.True(t, IsMatch("ln(create): /etc/zshrc", `ln(create): /etc/{{/\w+/}}`))
	assert.False(t, IsMatch("ln(create): /etc/zshrc", "ln.create."), "literal dots must not act as regex")
	assert.False(t, IsMatch("cp: /x", "ln(create)"))
}

func TestCommandArgs(t *testing.T) {
	assert.Equal(t, []string{"services", "bootout"}, Context{Command: "che services bootout"}.CommandArgs())
	assert.Nil(t, Context{Command: "che"}.CommandArgs())
	assert.Nil(t, Context{}.CommandArgs())
}

func TestExpand(t *testing.T) {
	got := Expand("${HOME}/x ${REPO}/y ${MISS}", map[string]string{"HOME": "/h", "REPO": "/r"})
	assert.Equal(t, "/h/x /r/y ${MISS}", got)
}

func TestReadFileExpected(t *testing.T) {
	assert.Equal(t, "expected body\n", ReadFile(t, td, "testdata/fixture/common/expected.txt"))
	EqualExpected(t, td, "testdata/fixture/common/expected.txt", "expected body\n")
}

func TestCopyDir(t *testing.T) {
	dest := t.TempDir()
	CopyDir(t, td, "testdata/fixture/common/tree-nested", dest)
	for rel, want := range map[string]string{"top.txt": "top\n", "dir/inner.txt": "inner\n"} {
		b, err := os.ReadFile(filepath.Join(dest, rel))
		require.NoError(t, err)
		assert.Equal(t, want, string(b), rel)
	}
}

// [<] 🤖🤖
