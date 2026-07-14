package render

// [>] 🤖🤖

import (
	"embed"
	"errors"
	"os"
	"testing"
	"time"

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
		return string(resolveAtIncludes(testutil.Tree(t, files), []byte(c.Input.Args.String(t, 1)))), nil
	})
}

func TestIsAtInclude(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/is_at_include_line.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		return isAtIncludeLine(c.Input.Args.String(t, 0)), nil
	})
}

func TestIsSecretRefPresent(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/is_secret_ref_present.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		return IsSecretRefPresent([]byte(c.Input.Args.String(t, 0))), nil
	})
}

func TestParseGCPRef(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/parse_gcp_ref.test.spec.yml", func(t *testing.T, c testyml.Case[parseGCPWant]) {
		project, secret, version, err := parseGCPRef(c.Input.Args.String(t, 0))
		if !c.Expected.Check(t, err) {
			assert.Equal(t, c.Expected.Output.Project, project, "project")
			assert.Equal(t, c.Expected.Output.Secret, secret, "secret")
			assert.Equal(t, c.Expected.Output.Version, version, "version")
		}
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
		return string(Compose(Composition{
			Body:       []byte(a.String(t, 0)),
			HeaderDest: a.String(t, 1),
			TmplName:   a.String(t, 2),
			Opts:       opts,
			Existing:   existing,
			RepoRoot:   root,
		})), nil
	})
}

func TestMergeUpsertEnv(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/merge_upsert_env.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		return string(mergeUpsertEnv([]byte(c.Input.Args.String(t, 0)), []byte(c.Input.Args.String(t, 1)))), nil
	})
}

// splitWant is split_frontmatter's expected.output: expected-file refs for the
// front and body halves.
type splitWant struct {
	FrontFile string `yaml:"frontFile"`
	BodyFile  string `yaml:"bodyFile"`
}

type parseGCPWant struct {
	Project string `yaml:"project"`
	Secret  string `yaml:"secret"`
	Version string `yaml:"version"`
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
		// [why] tilde paths resolve via HOME, ignoring repoRoot
		if c.Input.Args.Bool(t, 2) {
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

func TestIsRateLimitErr(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/is_rate_limit_err.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		var err error
		if msg := c.Input.Args.String(t, 0); msg != "" {
			err = errors.New(msg)
		}
		return isRateLimitErr(err), nil
	})
}

// retryWant is retry's expected.output: the returned value plus op call and
// sleep counts (asserted on error cases too).
type retryWant struct {
	Value string `yaml:"value"`
	Calls int    `yaml:"calls"`
	Slept int    `yaml:"slept"`
}

func TestRetry(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/retry.test.spec.yml", func(t *testing.T, c testyml.Case[retryWant]) {
		a := c.Input.Args
		failLeft, failErr := a.Int(t, 0), errors.New(a.String(t, 1))
		slept, calls := 0, 0
		v, err := retry([]time.Duration{1, 1, 1}, func(time.Duration) { slept++ }, isRateLimitErr,
			func() (string, error) {
				calls++
				if calls <= failLeft {
					return "", failErr
				}
				return "ok", nil
			})
		if c.Expected.Check(t, err) {
			assert.ErrorIs(t, err, failErr)
		} else {
			assert.Equal(t, c.Expected.Output.Value, v)
		}
		assert.Equal(t, c.Expected.Output.Calls, calls, "op calls")
		assert.Equal(t, c.Expected.Output.Slept, slept, "sleeps")
	})
}

func TestDirsTree(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/dirs_tree.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		var files map[string]string
		c.Input.Args.To(t, 0, &files)
		dir := t.TempDir()
		if len(files) > 0 {
			dir = testutil.Repo(t, files)
		}
		got, err := DirsTree(dir)
		if c.Expected.Check(t, err) {
			return
		}
		testyml.EqualExpected(t, td, c.Expected.Files, got)
	})
}

// TestMakefileDoc reads its Makefile and expected doc from the
// render-makefile-doc CLI's testdata via os ([why] embed FS cannot reach
// outside the package dir).
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

func TestExecWithCtx(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/exec_with_ctx.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		var itemCtx map[string]string
		c.Input.Args.To(t, 1, &itemCtx)
		repoRoot := testutil.Repo(t, map[string]string{"x": "x"})
		got, err := ExecWithCtx("t.tpl", []byte(c.Input.Args.String(t, 0)), repoRoot, itemCtx)
		return string(got), err
	})
}

// [<] 🤖🤖
