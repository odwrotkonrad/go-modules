package spec

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSchemaAcceptsFixtures(t *testing.T) {
	paths, err := filepath.Glob("../testutil/specs/*.yml")
	require.NoError(t, err)
	require.NotEmpty(t, paths, "no testutil spec fixtures found")
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			b, err := os.ReadFile(p)
			require.NoError(t, err)
			assert.Empty(t, ValidateSchema(b))
		})
	}
}

func TestCompiledSchema(t *testing.T) {
	sch, err := CompiledSchema()
	require.NoError(t, err)
	require.NotNil(t, sch)
}

func TestValidateSchemaUnparseableYAML(t *testing.T) {
	assert.Nil(t, ValidateSchema([]byte("p: [")), "parse errors belong to Load")
}

func TestValidateSchemaRootViolation(t *testing.T) {
	finds := ValidateSchema([]byte("- a\n- b\n"))
	require.NotEmpty(t, finds, "want a root finding")
	assert.True(t, strings.HasPrefix(finds[0], "/: "), "root finding not anchored at /: %q", finds[0])
}

func TestValidateSchemaFindsViolations(t *testing.T) {
	cases := map[string]struct {
		doc  string
		want string
	}{
		"extra key": {
			doc: `
p:
  includes:
    link: [HOME/**]
`,
			want: "includes",
		},
		"bogus writeType enum": {
			doc: `
p:
  include:
    renderTemplates:
      - files:
          - source: templates/a.tpl
            dest:
              - {path: a.md, options: {writeType: bogus}}
`,
			want: "writeType",
		},
		"non-octal chmod": {
			doc: `
p:
  include:
    mkdirs:
      - chmod: rwxr-xr-x
        files: [HOME/.cache]
`,
			want: "chmod",
		},
		"group without files": {
			doc: `
p:
  include:
    copy:
      - chmod: "0644"
`,
			want: "files",
		},
		"plugin ref without profile": {
			doc: `
p:
  plugins:
    - ./plugin
`,
			want: "plugins",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			finds := ValidateSchema([]byte(c.doc))
			require.NotEmptyf(t, finds, "want a finding mentioning %q", c.want)
			assert.Contains(t, strings.Join(finds, "\n"), c.want)
		})
	}
}

// [<] 🤖🤖
