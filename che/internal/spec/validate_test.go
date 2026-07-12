package spec

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSchemaAcceptsFixtures(t *testing.T) {
	paths, err := filepath.Glob("../testutil/specs/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no testutil spec fixtures found")
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if finds := ValidateSchema(b); len(finds) > 0 {
				t.Errorf("ValidateSchema(%s) = %v, want none", p, finds)
			}
		})
	}
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
			if len(finds) == 0 {
				t.Fatalf("ValidateSchema() = none, want a finding mentioning %q", c.want)
			}
			joined := strings.Join(finds, "\n")
			if !strings.Contains(joined, c.want) {
				t.Errorf("findings do not mention %q:\n%s", c.want, joined)
			}
		})
	}
}

// [<] 🤖🤖
