package spec

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestCompiledSchema(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/compiled_schema.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		sch, err := CompiledSchema()
		if err != nil {
			return false, err
		}
		return sch != nil, nil
	})
}

// TestValidateSchema: the fixturesGlob case accepts every testutil fixture,
// doc cases assert the findings (empty output: none wanted).
func TestValidateSchema(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/validate_schema.test.spec.yml", func(t *testing.T, c testyml.Case[string]) {
		a := c.Input.Args
		if a.Name(0) == "fixturesGlob" {
			paths, err := filepath.Glob(a.String(t, 0))
			require.NoError(t, err)
			require.NotEmpty(t, paths, "no testutil spec fixtures found")
			for _, p := range paths {
				b, err := os.ReadFile(p)
				require.NoError(t, err)
				assert.Emptyf(t, ValidateSchema(b), "schema rejects %s", p)
			}
			return
		}
		finds := ValidateSchema([]byte(a.String(t, 0)))
		if c.Expected.Output == "" {
			assert.Empty(t, finds)
			return
		}
		require.NotEmptyf(t, finds, "want a finding matching %q", c.Expected.Output)
		testyml.MustMatch(t, strings.Join(finds, "\n"), c.Expected.Output)
	})
}

// [<] 🤖🤖
