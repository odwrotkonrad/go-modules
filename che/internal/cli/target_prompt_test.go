package cli

// [>] 🤖🤖

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

type promptWant struct {
	Profiles string   `yaml:"profiles"`
	Prompt   []string `yaml:"prompt"`
}

func TestSelectTargetTypes(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/select_target_types.test.spec.yml",
		func(t *testing.T, c testyml.Case[promptWant]) {
			a, _, _ := setupRepoEnv(t, c.Context.Pwd)
			t.Setenv("CHE_DRY_RUN", "")
			t.Setenv("CHE_VALIDATE_SPEC", "")
			for k, v := range c.Context.Env {
				t.Setenv(k, v)
			}
			if extra := c.Input.Args.String(t, 0); extra != "" {
				require.NoError(t, os.WriteFile(filepath.Join(a.flags.CheWorkingDirectory, "che.yml"), []byte(extra), 0o644))
			}
			a.flags.TargetProfileTypes = c.Input.Args.Strings(t, 2)
			if p := c.Input.Args.String(t, 3); p != "" {
				a.flags.Profiles = []string{p}
			}
			var out bytes.Buffer
			a.prompt = targetPrompt{
				in:         strings.NewReader(c.Input.Args.String(t, 1)),
				out:        &out,
				isTerminal: func() bool { return c.Input.Args.Bool(t, 4) },
			}
			err := a.init("run")
			if err == nil {
				err = a.selectTargetTypes()
			}
			if c.Expected.Check(t, err) {
				return
			}
			var names []string
			for _, p := range a.specs.AllProfiles() {
				names = append(names, p.Ref())
			}
			assert.Equal(t, c.Expected.Output.Profiles, strings.Join(names, ","))
			for _, line := range c.Expected.Output.Prompt {
				assert.Contains(t, out.String(), line)
			}
			if len(c.Expected.Output.Prompt) == 0 {
				assert.Empty(t, out.String())
			}
		})
}

// [<] 🤖🤖
