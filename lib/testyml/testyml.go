// [>] 🤖🤖
package testyml

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type Want struct {
	StdOut   Matchers `yaml:"stdOut"`
	FilesOut string   `yaml:"filesOut"`
	StdErr   Matchers `yaml:"stdErr"`
	ExitCode int      `yaml:"exitCode"`
	Error    bool     `yaml:"error"`
}

type Matchers []string

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

func Match(s, matcher string) bool {
	return regexp.MustCompile(matcher).MatchString(s)
}

func MustMatch(t *testing.T, s, matcher string) {
	t.Helper()
	if !Match(s, matcher) {
		t.Errorf("output missing %q:\n--- got ---\n%s", matcher, s)
	}
}

func MustNotMatch(t *testing.T, s, matcher string) {
	t.Helper()
	if Match(s, matcher) {
		t.Errorf("output unexpectedly matches %q:\n--- got ---\n%s", matcher, s)
	}
}

func (w Want) WantsError() bool { return w.Error || w.ExitCode != 0 || len(w.StdErr) > 0 }

func (w Want) CheckErr(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("succeeded, want error %+v", w)
	}
	for _, f := range w.StdErr {
		if !Match(err.Error(), f) {
			t.Errorf("error %q missing %q", err.Error(), f)
		}
	}
}

func (w Want) CheckCode(t *testing.T, code int) {
	t.Helper()
	if code != w.ExitCode {
		t.Errorf("exit code = %d, want %d", code, w.ExitCode)
	}
}

func Run[C any](t *testing.T, fsys fs.FS, path string, fn func(t *testing.T, c C)) {
	t.Helper()
	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		t.Fatalf("read cases %s: %v", path, err)
	}
	var nodes []yaml.Node
	if err := yaml.Unmarshal(raw, &nodes); err != nil {
		t.Fatalf("decode cases %s: %v", path, err)
	}
	seen := map[string]bool{}
	for i := range nodes {
		var head struct {
			Name string `yaml:"name"`
		}
		if err := nodes[i].Decode(&head); err != nil {
			t.Fatalf("%s: case %d: %v", path, i, err)
		}
		if head.Name == "" {
			t.Fatalf("%s: case %d: missing name", path, i)
		}
		if seen[head.Name] {
			t.Fatalf("%s: duplicate case name %q", path, head.Name)
		}
		seen[head.Name] = true
		hasIn, hasWant := false, false
		for j := 0; j+1 < len(nodes[i].Content); j += 2 {
			switch nodes[i].Content[j].Value {
			case "in":
				hasIn = true
			case "want", "notWant", "contains":
				hasWant = true
			}
		}
		if !hasIn {
			t.Fatalf("%s: case %q: missing in", path, head.Name)
		}
		if !hasWant {
			t.Fatalf("%s: case %q: missing want", path, head.Name)
		}
		enc, err := yaml.Marshal(&nodes[i])
		if err != nil {
			t.Fatalf("%s: case %q: re-encode: %v", path, head.Name, err)
		}
		dec := yaml.NewDecoder(bytes.NewReader(enc))
		dec.KnownFields(true)
		var c C
		if err := dec.Decode(&c); err != nil {
			t.Fatalf("%s: case %q: %v", path, head.Name, err)
		}
		t.Run(head.Name, func(t *testing.T) { fn(t, c) })
	}
}

func Expand(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "${"+k+"}", v)
	}
	return s
}

func ReadFile(t *testing.T, fsys fs.FS, path string) string {
	t.Helper()
	b, err := fs.ReadFile(fsys, path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return string(b)
}

func EqualExpected(t *testing.T, fsys fs.FS, path, got string) {
	t.Helper()
	want := ReadFile(t, fsys, path)
	if got != want {
		t.Errorf("mismatch vs expected %s:\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

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
	if err != nil {
		t.Fatalf("copy fixture dir %s -> %s: %v", src, dest, err)
	}
}

// [<] 🤖🤖
