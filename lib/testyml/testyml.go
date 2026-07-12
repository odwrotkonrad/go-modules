// Package testyml runs <unit>.spec.yml table specs from embedded testdata and materializes fixture trees.
package testyml

// [>] 🤖🤖

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

// Context declares what a spec file or case runs against: the unit under test
// (function or command) plus the world around it. File-level context deep-merges
// under each case's own (case wins).
type Context struct {
	Function         string            `yaml:"function"`
	Command          string            `yaml:"command"`
	Pwd              string            `yaml:"pwd"`
	Env              map[string]string `yaml:"env"`
	MockedInterfaces map[string]string `yaml:"mockedInterfaces"`
}

// CommandArgs returns the context command's argv past the binary name.
func (c Context) CommandArgs() []string {
	f := strings.Fields(c.Command)
	if len(f) <= 1 {
		return nil
	}
	return f[1:]
}

// Input carries a case's arguments.
type Input struct {
	Args Args `yaml:"args"`
}

// arg is one input argument: an optional reader-facing name plus its value node.
type arg struct {
	name string
	node yaml.Node
}

// Args is the case argument list: bare values or single-key maps naming the
// argument. Names are for the reader, extraction is positional and typed.
type Args []arg

// UnmarshalYAML decodes a sequence of bare values or single-key naming maps.
func (a *Args) UnmarshalYAML(node *yaml.Node) error {
	var items []yaml.Node
	if err := node.Decode(&items); err != nil {
		return err
	}
	out := make(Args, 0, len(items))
	for i := range items {
		if items[i].Kind == yaml.MappingNode && len(items[i].Content) == 2 {
			out = append(out, arg{name: items[i].Content[0].Value, node: *items[i].Content[1]})
			continue
		}
		out = append(out, arg{node: items[i]})
	}
	*a = out
	return nil
}

// Name returns argument i's declared name ("" when bare or absent).
func (a Args) Name(i int) string {
	if i >= len(a) {
		return ""
	}
	return a[i].name
}

// To decodes argument i into out; absent trailing arguments leave out untouched.
func (a Args) To(t *testing.T, i int, out any) {
	t.Helper()
	if i >= len(a) {
		return
	}
	require.NoErrorf(t, a[i].node.Decode(out), "args[%d] (%s)", i, a[i].name)
}

// String returns argument i as a string ("" when absent).
func (a Args) String(t *testing.T, i int) string {
	t.Helper()
	var v string
	a.To(t, i, &v)
	return v
}

// Bool returns argument i as a bool (false when absent).
func (a Args) Bool(t *testing.T, i int) bool {
	t.Helper()
	var v bool
	a.To(t, i, &v)
	return v
}

// Int returns argument i as an int (0 when absent).
func (a Args) Int(t *testing.T, i int) int {
	t.Helper()
	var v int
	a.To(t, i, &v)
	return v
}

// Strings returns argument i as a string list (nil when absent).
func (a Args) Strings(t *testing.T, i int) []string {
	t.Helper()
	var v []string
	a.To(t, i, &v)
	return v
}

// Expected is the canonical expectation set: function tests use
// output/errorOutput, command tests use stdOut/stdErr/exitCode/files.
type Expected[W any] struct {
	Output      W        `yaml:"output"`
	ErrorOutput Matchers `yaml:"errorOutput"`
	StdOut      Matchers `yaml:"stdOut"`
	StdErr      Matchers `yaml:"stdErr"`
	ExitCode    int      `yaml:"exitCode"`
	Files       string   `yaml:"files"`
}

// IsErrorWanted reports whether the case expects the unit to fail: a non-zero
// exitCode or error-message matchers (errorOutput/stdErr). A bare "expect any
// error" is not expressible — every error case asserts something concrete.
func (e Expected[W]) IsErrorWanted() bool {
	return e.ExitCode != 0 || len(e.ErrorOutput) > 0 || len(e.StdErr) > 0
}

// Check runs the standard error ladder. Error wanted: assert the message
// matchers plus yamlcfg.Code when exitCode is set, return true (case done).
// Otherwise require no error and return false.
func (e Expected[W]) Check(t *testing.T, err error) bool {
	t.Helper()
	if !e.IsErrorWanted() {
		require.NoError(t, err)
		return false
	}
	require.Error(t, err)
	for _, m := range append(append(Matchers{}, e.ErrorOutput...), e.StdErr...) {
		if !IsMatch(err.Error(), m) {
			t.Errorf("error %q missing %q", err.Error(), m)
		}
	}
	if e.ExitCode != 0 {
		assert.Equal(t, e.ExitCode, yamlcfg.Code(err), "exit code (%v)", err)
	}
	return true
}

// Case is the canonical spec case shape.
type Case[W any] struct {
	Name        string      `yaml:"name"`
	Context     Context     `yaml:"context"`
	Input       Input       `yaml:"input"`
	Expected    Expected[W] `yaml:"expected"`
	NotExpected Expected[W] `yaml:"notExpected"`
}

// Eq runs a function spec: applies the context env, calls fn, runs the error
// ladder, asserts expected.output.
func Eq[W any](t *testing.T, fsys fs.FS, path string, fn func(t *testing.T, c Case[W]) (W, error)) {
	t.Helper()
	Run(t, fsys, path, func(t *testing.T, c Case[W]) {
		for k, v := range c.Context.Env {
			t.Setenv(k, v)
		}
		got, err := fn(t, c)
		if c.Expected.Check(t, err) {
			return
		}
		assert.Equal(t, c.Expected.Output, got)
	})
}

// Swap sets *ptr for the test, restoring the previous value on cleanup.
func Swap[T any](t testing.TB, ptr *T, v T) {
	t.Helper()
	prev := *ptr
	*ptr = v
	t.Cleanup(func() { *ptr = prev })
}

// Matchers is a list of output matchers: literal text with optional
// {{/regex/}} holes.
type Matchers []string

// UnmarshalYAML accepts a single matcher scalar or a matcher list.
func (m *Matchers) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		*m = Matchers{s}
		return nil
	}
	var list []string
	if err := node.Decode(&list); err != nil {
		return err
	}
	*m = list
	return nil
}

var holeRe = regexp.MustCompile(`\{\{/(.*?)/\}\}`)

func compileMatcher(matcher string) *regexp.Regexp {
	var b strings.Builder
	last := 0
	for _, loc := range holeRe.FindAllStringSubmatchIndex(matcher, -1) {
		b.WriteString(regexp.QuoteMeta(matcher[last:loc[0]]))
		b.WriteString(matcher[loc[2]:loc[3]])
		last = loc[1]
	}
	b.WriteString(regexp.QuoteMeta(matcher[last:]))
	return regexp.MustCompile(b.String())
}

// IsMatch reports whether s matches the literal-with-holes matcher.
func IsMatch(s, matcher string) bool {
	return compileMatcher(matcher).MatchString(s)
}

// MustMatch asserts s matches the matcher.
func MustMatch(t *testing.T, s, matcher string) {
	t.Helper()
	if !IsMatch(s, matcher) {
		t.Errorf("output missing %q:\n--- got ---\n%s", matcher, s)
	}
}

// MustNotMatch asserts s does not match the matcher.
func MustNotMatch(t *testing.T, s, matcher string) {
	t.Helper()
	if IsMatch(s, matcher) {
		t.Errorf("output unexpectedly matches %q:\n--- got ---\n%s", matcher, s)
	}
}

// Run decodes path's spec file ({context?, cases}) and runs fn per case as a
// named subtest. Spec files are named <unit-under-test>.test.spec.yml. Each
// case's context deep-merges over the file-level one (case wins) before the
// strict decode into C; the resolved context must name a function or command,
// its pwd must live under testdata/, and every case must carry
// expected/notExpected/contains.
func Run[C any](t *testing.T, fsys fs.FS, path string, fn func(t *testing.T, c C)) {
	t.Helper()
	if !strings.HasSuffix(path, ".test.spec.yml") {
		t.Fatalf("%s: spec files are named <unit-under-test>.test.spec.yml", path)
	}
	raw, err := fs.ReadFile(fsys, path)
	require.NoErrorf(t, err, "read cases %s", path)
	var file struct {
		Context yaml.Node   `yaml:"context"`
		Cases   []yaml.Node `yaml:"cases"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	require.NoErrorf(t, dec.Decode(&file), "decode cases %s", path)
	require.NotEmptyf(t, file.Cases, "%s: no cases", path)
	seen := map[string]bool{}
	for i := range file.Cases {
		node := &file.Cases[i]
		name := caseName(t, path, i, node)
		require.Falsef(t, seen[name], "%s: duplicate case name %q", path, name)
		seen[name] = true
		requireWantKey(t, path, name, node)
		mergeCaseContext(t, path, name, &file.Context, node)
		enc, err := yaml.Marshal(node)
		require.NoErrorf(t, err, "%s: case %q: re-encode", path, name)
		var c C
		require.NoErrorf(t, strictDecode(enc, &c), "%s: case %q", path, name)
		t.Run(name, func(t *testing.T) { fn(t, c) })
	}
}

func strictDecode(raw []byte, out any) error {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	return dec.Decode(out)
}

func caseName(t *testing.T, path string, i int, node *yaml.Node) string {
	t.Helper()
	var head struct {
		Name string `yaml:"name"`
	}
	require.NoErrorf(t, node.Decode(&head), "%s: case %d", path, i)
	require.NotEmptyf(t, head.Name, "%s: case %d: missing name", path, i)
	return head.Name
}

func requireWantKey(t *testing.T, path, name string, node *yaml.Node) {
	t.Helper()
	for i := 0; i+1 < len(node.Content); i += 2 {
		switch node.Content[i].Value {
		case "expected", "notExpected", "contains":
			return
		}
	}
	t.Fatalf("%s: case %q: missing expected/notExpected", path, name)
}

// mergeCaseContext replaces the case's context with fileCtx deep-merged under
// it (case wins) and validates the result names a function or command.
func mergeCaseContext(t *testing.T, path, name string, fileCtx, node *yaml.Node) {
	t.Helper()
	merged := mergeNode(cloneNode(t, fileCtx), mapValue(node, "context"))
	var ctx Context
	if merged != nil {
		enc, err := yaml.Marshal(merged)
		require.NoErrorf(t, err, "%s: case %q: context re-encode", path, name)
		require.NoErrorf(t, strictDecode(enc, &ctx), "%s: case %q: context", path, name)
	}
	if ctx.Function == "" && ctx.Command == "" {
		t.Fatalf("%s: case %q: context names neither function nor command", path, name)
	}
	if ctx.Pwd != "" && !strings.HasPrefix(ctx.Pwd, "testdata/") {
		t.Fatalf("%s: case %q: context.pwd %q must start with testdata/", path, name, ctx.Pwd)
	}
	setMapValue(node, "context", merged)
}

// cloneNode deep-copies a yaml node via a marshal round-trip (nil for zero nodes).
func cloneNode(t *testing.T, n *yaml.Node) *yaml.Node {
	t.Helper()
	if n == nil || n.Kind == 0 {
		return nil
	}
	enc, err := yaml.Marshal(n)
	require.NoError(t, err)
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal(enc, &doc))
	if len(doc.Content) == 0 {
		return nil
	}
	return doc.Content[0]
}

// mergeNode deep-merges mappings with over winning; non-mappings resolve to over.
func mergeNode(base, over *yaml.Node) *yaml.Node {
	if over == nil || over.Kind == 0 {
		return base
	}
	if base == nil || base.Kind != yaml.MappingNode || over.Kind != yaml.MappingNode {
		return over
	}
	for i := 0; i+1 < len(over.Content); i += 2 {
		key, val := over.Content[i], over.Content[i+1]
		found := false
		for j := 0; j+1 < len(base.Content); j += 2 {
			if base.Content[j].Value == key.Value {
				base.Content[j+1] = mergeNode(base.Content[j+1], val)
				found = true
				break
			}
		}
		if !found {
			base.Content = append(base.Content, key, val)
		}
	}
	return base
}

func mapValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func setMapValue(m *yaml.Node, key string, val *yaml.Node) {
	if val == nil {
		return
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = val
			return
		}
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key}, val)
}

// Expand replaces ${VAR} holes in s from vars.
func Expand(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "${"+k+"}", v)
	}
	return s
}

// ReadFile returns the embedded fixture's content.
func ReadFile(t *testing.T, fsys fs.FS, path string) string {
	t.Helper()
	b, err := fs.ReadFile(fsys, path)
	require.NoErrorf(t, err, "read fixture %s", path)
	return string(b)
}

// EqualExpected asserts got equals the expected-file fixture's content.
func EqualExpected(t *testing.T, fsys fs.FS, path, got string) {
	t.Helper()
	assert.Equal(t, ReadFile(t, fsys, path), got, "mismatch vs expected %s", path)
}

// CopyDir materializes an embedded fixture dir onto the real filesystem.
func CopyDir(t *testing.T, fsys fs.FS, src, dest string) {
	t.Helper()
	err := fs.WalkDir(fsys, src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, src), "/")
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	require.NoErrorf(t, err, "copy fixture dir %s -> %s", src, dest)
}

// [<] 🤖🤖
