package packages

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	sch, err := CompiledSchema()
	require.NoError(t, err)
	return sch
}

func yamlInstance(t *testing.T, b []byte) any {
	t.Helper()
	inst, err := spec.YAMLInstance(b)
	require.NoError(t, err)
	return inst
}

func TestSchemaValidate(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/schema.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) {
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
				require.NotEmptyf(t, hits, "no files match %s", pattern)
				for _, p := range hits {
					b, err := os.ReadFile(p)
					require.NoError(t, err)
					assert.Equalf(t, c.Expected.Output, valid(b), "schema verdict for %s", p)
				}
			}
		default:
			t.Fatalf("unknown arg %q", a.Name(0))
		}
	})
}

// [<] 🤖🤖
