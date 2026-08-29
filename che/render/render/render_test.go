package render

// [>] 🤖🤖

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata/spec all:testdata/fixture
var td embed.FS

func TestAutogenHeader(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/autogen_header.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		return autogenHeader(c.Input.Args.String(t, 0), "tmpl"), nil
	})
}

func TestResolveAtIncludes(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_at_includes.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		var files map[string]string
		c.Input.Args.To(t, 0, &files)
		got, err := resolveAtIncludes(testutil.Tree(t, files), []byte(c.Input.Args.String(t, 1)))
		return string(got), err
	})
}

func TestUnresolvedIncludes(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/unresolved_includes.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) ([]string, error) {
		var files map[string]string
		c.Input.Args.To(t, 0, &files)
		return UnresolvedIncludes(testutil.Tree(t, files), []byte(c.Input.Args.String(t, 1))), nil
	})
}

func TestLocalFileBodies(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/local_file_bodies.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) ([]string, error) {
		var files map[string]string
		c.Input.Args.To(t, 0, &files)
		out := []string{}
		for _, b := range LocalFileBodies(testutil.Tree(t, files), []byte(c.Input.Args.String(t, 1))) {
			out = append(out, string(b))
		}
		return out, nil
	})
}

func TestIsAtInclude(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/is_at_include_line.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		return isAtIncludeLine(c.Input.Args.String(t, 0)), nil
	})
}

func TestCompose(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/compose.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		a := c.Input.Args
		var opts Options
		a.To(t, 3, &opts)
		var existing []byte
		if s := a.String(t, 4); s != "" {
			existing = []byte(s)
		}
		var files map[string]string
		a.To(t, 5, &files)
		var root string
		if len(files) > 0 {
			root = testutil.Tree(t, files)
		}
		got, err := Compose(Composition{
			Body:       []byte(a.String(t, 0)),
			HeaderDest: a.String(t, 1),
			TmplName:   a.String(t, 2),
			Opts:       opts,
			Existing:   existing,
			RepoRoot:   root,
		})
		return string(got), err
	})
}

func TestMergeUpsertEnv(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/merge_upsert_env.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		a := c.Input.Args
		var mode string
		if len(a) > 2 {
			mode = a.String(t, 2)
		}
		runs := 0
		shell := func(cmd string) (string, error) { runs++; return "ran:" + cmd, nil }
		out, err := mergeUpsertEnv([]byte(a.String(t, 0)), []byte(a.String(t, 1)), mode, shell)
		return fmt.Sprintf("%s|runs=%d", out, runs), err
	})
}

type splitWant struct {
	FrontFile string `yaml:"frontFile"`
	BodyFile  string `yaml:"bodyFile"`
}

func TestSplitFrontmatter(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/split_frontmatter.test.spec.yml", func(t *testing.T, c testyml.Case[splitWant]) {
		front, body := SplitFrontmatter(testyml.ReadFile(t, td, c.Input.Args.String(t, 0)))
		testyml.EqualExpected(t, td, c.Expected.Output.FrontFile, front)
		testyml.EqualExpected(t, td, c.Expected.Output.BodyFile, body)
	})
}

func TestReadFrontmatter(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/read_frontmatter.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		var files map[string]string
		c.Input.Args.To(t, 0, &files)
		return ReadFrontmatter(testutil.Tree(t, files), c.Input.Args.String(t, 1))
	})
}

func TestReadBody(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/read_body.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		var files map[string]string
		c.Input.Args.To(t, 0, &files)
		return ReadBody(testutil.Tree(t, files), c.Input.Args.String(t, 1))
	})
}

func TestRenderMarkdown(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/render_markdown.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		root := t.TempDir()
		testyml.CopyDir(t, td, c.Context.Pwd, root)
		repoRoot := root
		tildeViaHome := c.Input.Args.Bool(t, 2)
		if tildeViaHome {
			t.Setenv("HOME", root)
			repoRoot = "/unused"
		}
		got, err := RenderMarkdown(repoRoot, c.Input.Args.String(t, 0), c.Input.Args.Strings(t, 1)...)
		if c.Expected.Check(t, err) {
			return
		}
		testyml.EqualExpected(t, td, c.Expected.Files, got)
	})
}

func TestDirsTree(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/dirs_tree.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		var files map[string]string
		c.Input.Args.To(t, 0, &files)
		dir := t.TempDir()
		if len(files) > 0 && c.Input.Args.Bool(t, 1) {
			for p, body := range files {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, p), []byte(body), 0o644))
			}
		} else if len(files) > 0 {
			dir = testutil.Repo(t, files)
		}
		got, err := DirsTree(dir)
		if c.Expected.Check(t, err) {
			return
		}
		testyml.EqualExpected(t, td, c.Expected.Files, got)
	})
}

func TestMakefileDoc(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/makefile_doc.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		got, err := MakefileDoc(c.Input.Args.String(t, 0))
		if c.Expected.Check(t, err) {
			return
		}
		want, err := os.ReadFile(c.Expected.Files)
		require.NoError(t, err)
		assert.Equal(t, string(want), got)
	})
}

func TestExecWithData(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/exec_with_data.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		var data Data
		c.Input.Args.To(t, 1, &data)
		repoRoot := testutil.Repo(t, map[string]string{"x": "x"})
		got, err := ExecWithData("t.tpl", []byte(c.Input.Args.String(t, 0)), repoRoot, data)
		return string(got), err
	})
}

type sectionWant struct {
	Name  string `yaml:"name"`
	Open  int    `yaml:"open"`
	Close int    `yaml:"close"`
	Depth int    `yaml:"depth"`
}

func TestSectionsParse(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/sections_parse.test.spec.yml", func(t *testing.T, c testyml.Case[[]sectionWant]) ([]sectionWant, error) {
		sections, err := sectionsParse(c.Input.Args.String(t, 0), c.Input.Args.String(t, 1))
		if err != nil {
			return nil, err
		}
		var out []sectionWant
		for _, s := range sections {
			out = append(out, sectionWant{Name: s.name, Open: s.open + 1, Close: s.close + 1, Depth: s.depth})
		}
		return out, nil
	})
}

func TestSectionsInject(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/sections_inject.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		a := c.Input.Args
		out, err := sectionsInject(a.String(t, 0), a.String(t, 1), a.String(t, 2))
		if err != nil || !a.Bool(t, 3) {
			return out, err
		}
		return sectionsInject(out, a.String(t, 1), a.String(t, 2))
	})
}

// [<] 🤖🤖

// [>] 🤖🤖
func TestLocalFileRelativeInclude(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pwd.md"), []byte("LOCAL"), 0o644))
	body := []byte(`{{ localFile "pwd.md" }}`)
	got, err := ExecWithData("t.tpl", body, dir, Data{})
	require.NoError(t, err)
	assert.Equal(t, "LOCAL", string(got))
}

// [<] 🤖🤖
