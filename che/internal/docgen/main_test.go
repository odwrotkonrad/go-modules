package main

// [>] 🤖🤖

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	sch, err := spec.CompiledSchema()
	require.NoError(t, err)
	return sch
}

func yamlInstance(t *testing.T, b []byte) any {
	t.Helper()
	inst, err := spec.YAMLInstance(b)
	require.NoError(t, err)
	return inst
}

// TestSchemaValidate drives the compiled schema per arg shape: doc (inline
// snippet), path (one file), globs (repo fixtures + every sibling-repo
// che.yml reachable in the local workspace checkout), docExample (the first
// yaml fence after docs/spec.md's Full Example heading).
func TestSchemaValidate(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/schema_validate.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) {
		sch := compileSchema(t)
		a := c.Input.Args
		valid := func(b []byte) bool { return sch.Validate(yamlInstance(t, b)) == nil }
		switch a.Name(0) {
		case "doc":
			assert.Equal(t, c.Expected.Output, valid([]byte(a.String(t, 0))))
		case "path":
			b, err := os.ReadFile(a.String(t, 0))
			require.NoError(t, err)
			assert.Equal(t, c.Expected.Output, valid(b))
		case "globs":
			for _, pattern := range a.Strings(t, 0) {
				hits, err := filepath.Glob(pattern)
				require.NoError(t, err)
				for _, p := range hits {
					b, err := os.ReadFile(p)
					require.NoError(t, err)
					assert.Equalf(t, c.Expected.Output, valid(b), "schema verdict for %s", p)
				}
			}
		case "docExample":
			b, err := os.ReadFile(a.String(t, 0))
			require.NoError(t, err)
			_, rest, ok := strings.Cut(string(b), "## Full Example")
			require.True(t, ok, "no Full Example section")
			_, rest, ok = strings.Cut(rest, "```yaml\n")
			require.True(t, ok, "no yaml fence in Full Example")
			example, _, ok := strings.Cut(rest, "```")
			require.True(t, ok, "unclosed yaml fence in Full Example")
			assert.Equal(t, c.Expected.Output, valid([]byte(example)))
		default:
			t.Fatalf("unknown arg %q", a.Name(0))
		}
	})
}

type flagDef struct {
	Type  string `yaml:"type"`
	Name  string `yaml:"name"`
	Short string `yaml:"short"`
	Usage string `yaml:"usage"`
}

func TestOptionsTable(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/options_table.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) {
		var defs []flagDef
		c.Input.Args.To(t, 0, &defs)
		fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
		for _, d := range defs {
			switch d.Type {
			case "string":
				fs.StringP(d.Name, d.Short, "", d.Usage)
			case "bool":
				fs.BoolP(d.Name, d.Short, false, d.Usage)
			default:
				t.Fatalf("unknown flag type %q", d.Type)
			}
		}
		got := optionsTable(fs)
		for _, w := range c.Expected.Output {
			assert.Contains(t, got, w+"\n", "optionsTable row")
		}
	})
}

// [<] 🤖🤖
