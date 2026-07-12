package main

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

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

// TestSchemaValidatesRealSpecs validates the repo's own che.yml, every
// testutil fixture, and every sibling-repo che.yml reachable in the local
// workspace checkout.
func TestSchemaValidatesRealSpecs(t *testing.T) {
	sch := compileSchema(t)
	paths := []string{"../../../che.yml"}
	fixtures, err := filepath.Glob("../testutil/specs/*.yml")
	require.NoError(t, err)
	paths = append(paths, fixtures...)
	for _, pattern := range []string{"../../../../*/che.yml", "../../../../*/*/che.yml", "../../../../*/*/*/che.yml"} {
		hits, err := filepath.Glob(pattern)
		require.NoError(t, err)
		paths = append(paths, hits...)
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			b, err := os.ReadFile(p)
			require.NoError(t, err)
			assert.NoErrorf(t, sch.Validate(yamlInstance(t, b)), "schema rejects %s", p)
		})
	}
}

// TestSpecDocExampleValidates keeps docs/spec.md's Full Example schema-valid:
// the first yaml fence after the Full Example heading must pass.
func TestSpecDocExampleValidates(t *testing.T) {
	b, err := os.ReadFile("../../docs/spec.md")
	require.NoError(t, err)
	_, rest, ok := strings.Cut(string(b), "## Full Example")
	require.True(t, ok, "docs/spec.md: no Full Example section")
	_, rest, ok = strings.Cut(rest, "```yaml\n")
	require.True(t, ok, "docs/spec.md: no yaml fence in Full Example")
	example, _, ok := strings.Cut(rest, "```")
	require.True(t, ok, "docs/spec.md: unclosed yaml fence in Full Example")
	assert.NoError(t, compileSchema(t).Validate(yamlInstance(t, []byte(example))), "schema rejects the Full Example")
}

// TestSchemaRejectsInvalidSpecs guards the schema against loosening: each
// snippet violates one constraint the parser enforces.
func TestSchemaRejectsInvalidSpecs(t *testing.T) {
	sch := compileSchema(t)
	cases := map[string]string{
		"bogus writeType": `
p:
  include:
    renderTemplates:
      - files:
          - source: templates/a.tpl
            dest:
              - {path: a.md, options: {writeType: bogus}}
`,
		"plugin ref without ::profile": `
p:
  plugins:
    - ./plugin
`,
		"unknown profile key": `
p:
  includes:
    link: [HOME/**]
`,
		"file entry without source": `
p:
  include:
    copy:
      - files:
          - {dest: [~/.config/a]}
`,
		"non-octal chmod": `
p:
  include:
    mkdirs:
      - chmod: rwxr-xr-x
        files: [HOME/.cache]
`,
		"mixinProfiles not a list": `
p:
  mixinProfiles: base
`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Errorf(t, sch.Validate(yamlInstance(t, []byte(doc))), "schema accepts invalid spec: %s", name)
		})
	}
}

func TestOptionsTable(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	var dir, mode string
	var toggle bool
	fs.StringVarP(&dir, "dir", "C", "", "change dir; env: X_DIR")
	fs.StringVar(&mode, "mode", "", "pick mode; values: a (one) | b (two); default: off; env: X_MODE")
	fs.BoolVar(&toggle, "toggle", false, "flip it")
	got := optionsTable(fs)
	want := []string{
		"| Option | Env | Values | Default | Description |",
		"| `-C`, `--dir` | `X_DIR` | `string` |  | change dir |",
		"| `--mode` | `X_MODE` | `a (one)` \\| `b (two)` | `off` | pick mode |",
		"| `--toggle` |  | `bool` | `false` | flip it |",
	}
	for _, w := range want {
		assert.Contains(t, got, w+"\n", "optionsTable row")
	}
}

// [<] 🤖🤖
