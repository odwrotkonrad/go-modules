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
	testyml.Eq(t, td, "testdata/spec/parse_plugin_ref.test.spec.yml", func(t *testing.T, c testyml.Case[refWant]) (refWant, error) {
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
	dir := testutil.Tree(t, map[string]string{"che.yml": "p:\n  options:\n    execIf: ['env:X']\n"})
	s, err := Load(filepath.Join(dir, "che.yml"))
	require.NoError(t, err)
	eval := NewEvaluator().EvalExecIf
	t.Setenv("X", "")
	out, _ := testutil.CaptureStdout(t, func() error {
		ok, err := s.ExecIfPass("p", false, eval)
		require.NoError(t, err)
		assert.False(t, ok, "unset env must reject")
		return nil
	})
	testutil.NotLine(t, out, "execIf(reject)")
	log.SetDebug(true)
	t.Cleanup(func() { log.SetDebug(false) })
	out, _ = testutil.CaptureStdout(t, func() error {
		_, err := s.ExecIfPass("p", false, eval)
		return err
	})
	testutil.WantLines(t, out, "execIf(reject): profile p: env:X")
	t.Setenv("X", "1")
	out, _ = testutil.CaptureStdout(t, func() error {
		ok, err := s.ExecIfPass("p", false, eval)
		require.NoError(t, err)
		assert.True(t, ok, "set env must pass")
		return nil
	})
	testutil.WantLines(t, out, "execIf(pass): profile p: env:X")
	_, err = s.ExecIfPass("nope", false, eval)
	assert.Error(t, err, "undefined profile")
}

// [<] 🤖🤖
