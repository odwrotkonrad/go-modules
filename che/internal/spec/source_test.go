package spec

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

type sourceWant struct {
	Source     string            `yaml:"source"`
	Profile    string            `yaml:"profile"`
	SourceType string            `yaml:"sourceType"`
	Spec       string            `yaml:"spec"`
	Env        map[string]string `yaml:"env"`
	Variables  map[string]string `yaml:"variables"`
	String     string            `yaml:"string"`
	Ref        string            `yaml:"ref"`
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
			Spec:       ref.Spec,
			Env:        ref.Env,
			Variables:  ref.Variables,
			String:     ref.String(),
			Ref:        ref.Ref,
		}, nil
	})
}

func TestRunIfGate(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/run_if_pass.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) {
		for k, v := range c.Context.Env {
			t.Setenv(k, v)
		}
		if c.Input.Args.Bool(t, 1) {
			t.Cleanup(log.SwapLevel(log.Levels.Trace))
		}
		dir := testutil.Tree(t, map[string]string{"che.yml": "p:\n  options:\n    runIf: ['env:X']\n"})
		d, err := Load(filepath.Join(dir, "che.yml"), Interp{})
		require.NoError(t, err)
		var ok bool
		out, err := testutil.CaptureStdout(t, func() error {
			name := c.Input.Args.String(t, 0)
			rec, e := FindRecipe(d.ProfileRecipes, name)
			if e != nil {
				return e
			}
			ok, _, e = AllPass(name, rec.Options.RunIf, false, NewEvaluator(os.Getenv).EvalRunIf)
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
