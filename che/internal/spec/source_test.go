package spec

// [>] 🤖🤖

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// sourceWant is profile_source_decode's expected.output.
type sourceWant struct {
	Source     string            `yaml:"source"`
	Profile    string            `yaml:"profile"`
	SourceType string            `yaml:"sourceType"`
	Env        map[string]string `yaml:"env"`
	String     string            `yaml:"string"`
}

func TestProfileSourceDecode(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/profile_source_decode.test.spec.yml", func(t *testing.T, c testyml.Case[sourceWant]) (sourceWant, error) {
		var ref ProfileSourceRecipe
		if err := yaml.Unmarshal([]byte(c.Input.Args.String(t, 0)), &ref); err != nil {
			return sourceWant{}, err
		}
		if err := ref.IsValid(); err != nil {
			return sourceWant{}, err
		}
		return sourceWant{
			Source:     ref.URI,
			Profile:    ref.GetProfileName(),
			SourceType: string(ref.GetSourceType()),
			Env:        ref.Env,
			String:     ref.String(),
		}, nil
	})
}

// AllPass gates on a profile's execIf; FindRecipe errors on undefined names.
// A pass logs at normal level, a reject only at debug level.
func TestExecIfGate(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/exec_if_pass.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) {
		for k, v := range c.Context.Env {
			t.Setenv(k, v)
		}
		if c.Input.Args.Bool(t, 1) {
			log.SetDebug(true)
			t.Cleanup(func() { log.SetDebug(false) })
		}
		dir := testutil.Tree(t, map[string]string{"che.yml": "p:\n  options:\n    execIf: ['env:X']\n"})
		d, err := Load(filepath.Join(dir, "che.yml"))
		require.NoError(t, err)
		var ok bool
		out, err := testutil.CaptureStdout(t, func() error {
			name := c.Input.Args.String(t, 0)
			rec, e := FindRecipe(d.ProfileRecipes, name)
			if e != nil {
				return e
			}
			ok, e = AllPass(name, rec.Options.ExecIf, false, NewEvaluator().EvalExecIf)
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
