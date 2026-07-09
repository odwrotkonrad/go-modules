package main

// [>] 🤖🤖

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON()))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("che.schema.json", doc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile("che.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return sch
}

// yamlInstance decodes YAML then round-trips through JSON, yielding the value
// shape jsonschema validates.
func yamlInstance(t *testing.T, b []byte) any {
	t.Helper()
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	j, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(j))
	if err != nil {
		t.Fatal(err)
	}
	return inst
}

// TestSchemaValidatesRealSpecs validates the repo's own che.yml, every
// testutil fixture, and every sibling-repo che.yml reachable in the local
// workspace checkout.
func TestSchemaValidatesRealSpecs(t *testing.T) {
	sch := compileSchema(t)
	paths := []string{"../../../che.yml"}
	fixtures, err := filepath.Glob("../testutil/specs/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, fixtures...)
	for _, pattern := range []string{"../../../../*/che.yml", "../../../../*/*/che.yml", "../../../../*/*/*/che.yml"} {
		hits, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, hits...)
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if err := sch.Validate(yamlInstance(t, b)); err != nil {
				t.Errorf("schema rejects %s: %v", p, err)
			}
		})
	}
}

// TestSpecDocExampleValidates keeps docs/spec.md's Full Example schema-valid:
// the first yaml fence after the Full Example heading must pass.
func TestSpecDocExampleValidates(t *testing.T) {
	b, err := os.ReadFile("../../docs/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	_, rest, ok := strings.Cut(string(b), "## Full Example")
	if !ok {
		t.Fatal("docs/spec.md: no Full Example section")
	}
	_, rest, ok = strings.Cut(rest, "```yaml\n")
	if !ok {
		t.Fatal("docs/spec.md: no yaml fence in Full Example")
	}
	example, _, ok := strings.Cut(rest, "```")
	if !ok {
		t.Fatal("docs/spec.md: unclosed yaml fence in Full Example")
	}
	if err := compileSchema(t).Validate(yamlInstance(t, []byte(example))); err != nil {
		t.Errorf("schema rejects the Full Example: %v", err)
	}
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
			if err := sch.Validate(yamlInstance(t, []byte(doc))); err == nil {
				t.Errorf("schema accepts invalid spec: %s", name)
			}
		})
	}
}

// [<] 🤖🤖
