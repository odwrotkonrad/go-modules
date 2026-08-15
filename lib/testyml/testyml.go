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

	"gitlab.com/konradodwrot/go-modules/lib/climain"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

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
	require.NoErrorf(t, StrictDecode(raw, &file), "decode cases %s", path)
	require.NotEmptyf(t, file.Cases, "%s: no cases", path)
	seen := map[string]bool{}
	for i := range file.Cases {
		node := &file.Cases[i]
		name := decodeCaseName(t, path, i, node)
		require.Falsef(t, seen[name], "%s: duplicate case name %q", path, name)
		seen[name] = true
		requireAssertionKey(t, path, name, node)
		mergeCaseContext(t, path, name, &file.Context, node)
		var c C
		require.NoErrorf(t, StrictDecodeNode(node, &c), "%s: case %q", path, name)
		t.Run(name, func(t *testing.T) { fn(t, c) })
	}
}

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

func decodeCaseName(t *testing.T, path string, i int, node *yaml.Node) string {
	t.Helper()
	var head struct {
		Name string `yaml:"name"`
	}
	require.NoErrorf(t, node.Decode(&head), "%s: case %d", path, i)
	require.NotEmptyf(t, head.Name, "%s: case %d: missing name", path, i)
	return head.Name
}

func requireAssertionKey(t *testing.T, path, name string, node *yaml.Node) {
	t.Helper()
	for key := range yamlcfg.MapPairs(node) {
		switch key.Value {
		case "expected", "notExpected", "contains":
			return
		}
	}
	t.Fatalf("%s: case %q: missing expected/notExpected/contains", path, name)
}

func mergeCaseContext(t *testing.T, path, name string, fileCtx, node *yaml.Node) {
	t.Helper()
	merged := yamlcfg.MergeNodes(cloneNode(t, fileCtx), mapValue(node, "context"))
	var ctx Context
	if merged != nil {
		require.NoErrorf(t, StrictDecodeNode(merged, &ctx), "%s: case %q: context", path, name)
	}
	if ctx.Function == "" && ctx.Command == "" {
		t.Fatalf("%s: case %q: context names neither function nor command", path, name)
	}
	if ctx.Pwd != "" && !strings.HasPrefix(ctx.Pwd, "testdata/") {
		t.Fatalf("%s: case %q: context.pwd %q must start with testdata/", path, name, ctx.Pwd)
	}
	setMapValue(node, "context", merged)
}

func cloneNode(t *testing.T, node *yaml.Node) *yaml.Node {
	t.Helper()
	if node == nil || node.Kind == 0 {
		return nil
	}
	encoded, err := yaml.Marshal(node)
	require.NoError(t, err)
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal(encoded, &doc))
	if len(doc.Content) == 0 {
		return nil
	}
	return doc.Content[0]
}

func mapValue(mapping *yaml.Node, key string) *yaml.Node {
	for k, val := range yamlcfg.MapPairs(mapping) {
		if k.Value == key {
			return val
		}
	}
	return nil
}

func setMapValue(mapping *yaml.Node, key string, val *yaml.Node) {
	if val == nil {
		return
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = val
			return
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key}, val)
}

func StrictDecode(raw []byte, out any) error {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	return dec.Decode(out)
}

func StrictDecodeNode(node *yaml.Node, out any) error {
	enc, err := yaml.Marshal(node)
	if err != nil {
		return err
	}
	return StrictDecode(enc, out)
}

func (c Context) CommandArgs() []string {
	fields := strings.Fields(c.Command)
	if len(fields) <= 1 {
		return nil
	}
	return fields[1:]
}

func (c Context) CommandArgsExpanded(vars map[string]string) []string {
	args := c.CommandArgs()
	for i, a := range args {
		args[i] = Expand(a, vars)
	}
	return args
}

func (a *Args) UnmarshalYAML(node *yaml.Node) error {
	var items []yaml.Node
	if err := node.Decode(&items); err != nil {
		return err
	}
	args := make(Args, 0, len(items))
	for i := range items {
		if items[i].Kind == yaml.MappingNode && len(items[i].Content) == 2 {
			args = append(args, arg{name: items[i].Content[0].Value, node: *items[i].Content[1]})
			continue
		}
		args = append(args, arg{node: items[i]})
	}
	*a = args
	return nil
}

func (a Args) Name(i int) string {
	if i >= len(a) {
		return ""
	}
	return a[i].name
}

func (a Args) To(t *testing.T, i int, out any) {
	t.Helper()
	if i >= len(a) {
		return
	}
	require.NoErrorf(t, a[i].node.Decode(out), "args[%d] (%s)", i, a[i].name)
}

func (a Args) String(t *testing.T, i int) string {
	t.Helper()
	var v string
	a.To(t, i, &v)
	return v
}

func (a Args) Bool(t *testing.T, i int) bool {
	t.Helper()
	var v bool
	a.To(t, i, &v)
	return v
}

func (a Args) Int(t *testing.T, i int) int {
	t.Helper()
	var v int
	a.To(t, i, &v)
	return v
}

func (a Args) Strings(t *testing.T, i int) []string {
	t.Helper()
	var v []string
	a.To(t, i, &v)
	return v
}

func (e Expected[W]) IsErrorWanted() bool {
	return e.ExitCode != 0 || len(e.ErrorOutput) > 0 || len(e.StdErr) > 0
}

func (e Expected[W]) Check(t *testing.T, err error) bool {
	t.Helper()
	if !e.IsErrorWanted() {
		require.NoError(t, err)
		return false
	}
	require.Error(t, err)
	for _, matcher := range append(append(Matchers{}, e.ErrorOutput...), e.StdErr...) {
		if !IsMatch(err.Error(), matcher) {
			t.Errorf("error %q missing %q", err.Error(), matcher)
		}
	}
	if e.ExitCode != 0 {
		assert.Equal(t, e.ExitCode, yamlcfg.ExitCode(err), "exit code (%v)", err)
	}
	return true
}

func (m *Matchers) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var single string
		if err := node.Decode(&single); err != nil {
			return err
		}
		*m = Matchers{single}
		return nil
	}
	var list []string
	if err := node.Decode(&list); err != nil {
		return err
	}
	*m = list
	return nil
}

var matcherRegexpEscape = regexp.MustCompile(`\{\{/(.*?)/\}\}`)

func matcherPattern(matcher string) string {
	var pattern strings.Builder
	literalStart := 0
	for _, loc := range matcherRegexpEscape.FindAllStringSubmatchIndex(matcher, -1) {
		pattern.WriteString(regexp.QuoteMeta(matcher[literalStart:loc[0]]))
		pattern.WriteString(matcher[loc[2]:loc[3]])
		literalStart = loc[1]
	}
	pattern.WriteString(regexp.QuoteMeta(matcher[literalStart:]))
	return pattern.String()
}

func IsMatch(s, matcher string) bool {
	return regexp.MustCompile(matcherPattern(matcher)).MatchString(s)
}

func IsMatchFull(s, matcher string) bool {
	return regexp.MustCompile(`\A` + matcherPattern(matcher) + `\z`).MatchString(s)
}

func MustMatch(t *testing.T, s, matcher string) {
	t.Helper()
	if !IsMatch(s, matcher) {
		t.Errorf("output missing %q:\n--- got ---\n%s", matcher, s)
	}
}

func MustNotMatch(t *testing.T, s, matcher string) {
	t.Helper()
	if IsMatch(s, matcher) {
		t.Errorf("output unexpectedly matches %q:\n--- got ---\n%s", matcher, s)
	}
}

func MustCount(t *testing.T, s, substr string, want int) {
	t.Helper()
	if got := strings.Count(s, substr); got != want {
		t.Errorf("output contains %q %d times, want %d:\n--- got ---\n%s", substr, got, want, s)
	}
}

func Expand(s string, vars map[string]string) string {
	for name, val := range vars {
		s = strings.ReplaceAll(s, "${"+name+"}", val)
	}
	return s
}

func Swap[T any](t testing.TB, ptr *T, v T) {
	t.Helper()
	prev := *ptr
	*ptr = v
	t.Cleanup(func() { *ptr = prev })
}

func ReadFile(t *testing.T, fsys fs.FS, path string) string {
	t.Helper()
	data, err := fs.ReadFile(fsys, path)
	require.NoErrorf(t, err, "read fixture %s", path)
	return string(data)
}

func EqualExpected(t *testing.T, fsys fs.FS, path, got string) {
	t.Helper()
	assert.Equal(t, ReadFile(t, fsys, path), got, "mismatch vs expected %s", path)
}

func CopyDir(t *testing.T, fsys fs.FS, src, dest string) {
	t.Helper()
	err := fs.WalkDir(fsys, src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(path, src), "/")
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	require.NoErrorf(t, err, "copy fixture dir %s -> %s", src, dest)
}

func ConfigDir(t *testing.T, configName, raw string) string {
	t.Helper()
	Swap(t, &yamlcfg.SystemConfigDir, filepath.Join(t.TempDir(), "no-system"))
	dir := t.TempDir()
	if raw != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, configName), []byte(raw), 0o644))
	}
	return dir
}

type helpVersionWant struct {
	Usage bool `yaml:"usage"`
	Done  bool `yaml:"done"`
}

func RunHelpVersionSpec(t *testing.T, fsys fs.FS, path, usage, name, version string) {
	t.Helper()
	Eq(t, fsys, path, func(t *testing.T, c Case[helpVersionWant]) (helpVersionWant, error) {
		out, done := climain.HelpVersion(c.Input.Args.Strings(t, 0), usage, name, version)
		return helpVersionWant{Usage: out == usage, Done: done}, nil
	})
}

// [<] 🤖🤖
