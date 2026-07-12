package spec

// [>] 🤖🤖

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// refWant is parse_plugin_ref's expected.output.
type refWant struct {
	URL     string `yaml:"url"`
	Profile string `yaml:"profile"`
	IsPath  bool   `yaml:"isPath"`
}

func TestParsePluginRef(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/parse_plugin_ref.test.spec.yml", func(t *testing.T, c testyml.Case[refWant]) (refWant, error) {
		entry := c.Input.Args.String(t, 0)
		ref, err := parsePluginRef(entry)
		if err != nil {
			return refWant{}, err
		}
		assert.Equal(t, entry, ref.String(), "String() must round-trip")
		return refWant{URL: ref.URL, Profile: ref.Profile, IsPath: ref.IsPath}, nil
	})
}

// ExecIfPass gates on the named profile's execIf; undefined profile errors.
// A pass logs at normal level, a reject only at debug level.
func TestExecIfPass(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/exec_if_pass.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) {
		for k, v := range c.Context.Env {
			t.Setenv(k, v)
		}
		if c.Input.Args.Bool(t, 1) {
			log.SetDebug(true)
			t.Cleanup(func() { log.SetDebug(false) })
		}
		dir := testutil.Tree(t, map[string]string{"che.yml": "p:\n  options:\n    execIf: ['env:X']\n"})
		s, err := Load(filepath.Join(dir, "che.yml"))
		require.NoError(t, err)
		var ok bool
		out, err := testutil.CaptureStdout(t, func() error {
			var e error
			ok, e = s.ExecIfPass(c.Input.Args.String(t, 0), false, NewEvaluator().EvalExecIf)
			return e
		})
		if c.Expected.Check(t, err) {
			return
		}
		assert.Equal(t, c.Expected.Output, ok)
		out = testutil.StripANSI(out)
		for _, m := range c.Expected.StdOut {
			testyml.MustMatch(t, out, m)
		}
		for _, m := range c.NotExpected.StdOut {
			testyml.MustNotMatch(t, out, m)
		}
	})
}

// [<] 🤖🤖
