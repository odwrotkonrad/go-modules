// [>] 🤖🤖
package testyml

import (
	"embed"
	"os"
	"path/filepath"
	"testing"
)

//go:embed all:testdata
var td embed.FS

func TestRun(t *testing.T) {
	type in struct {
		Args []int
	}
	type c struct {
		Name string
		In   in
		Want int
	}
	ran := map[string]bool{}
	Run(t, td, "testdata/spec/run.spec.yml", func(t *testing.T, c c) {
		ran[c.Name] = true
		sum := 0
		for _, n := range c.In.Args {
			sum += n
		}
		if sum != c.Want {
			t.Errorf("sum(%v) = %d, want %d", c.In.Args, sum, c.Want)
		}
	})
	if !ran["add"] || !ran["negatives"] {
		t.Errorf("ran = %v, want both cases", ran)
	}
}

func TestMatchers(t *testing.T) {
	type in struct {
		Args []string
	}
	type c struct {
		Name string
		In   in
		Want Want
	}
	Run(t, td, "testdata/spec/matchers.spec.yml", func(t *testing.T, c c) {
		for _, m := range c.Want.StdOut {
			MustMatch(t, c.In.Args[0], m)
		}
	})
}

func TestMatch(t *testing.T) {
	if !Match("ln(create): /etc/zshrc", "ln(create)") {
		t.Error("substring matcher failed")
	}
	if !Match("ln(create): /etc/zshrc", `re:^ln\(create\): /etc/\w+$`) {
		t.Error("regex matcher failed")
	}
	if Match("cp: /x", "re:^ln") {
		t.Error("regex matcher matched wrongly")
	}
}

func TestExpand(t *testing.T) {
	got := Expand("${HOME}/x ${REPO}/y ${MISS}", map[string]string{"HOME": "/h", "REPO": "/r"})
	if got != "/h/x /r/y ${MISS}" {
		t.Errorf("Expand = %q", got)
	}
}

func TestReadFileExpected(t *testing.T) {
	if got := ReadFile(t, td, "testdata/fixture/common/expected.txt"); got != "expected body\n" {
		t.Errorf("ReadFile = %q", got)
	}
	EqualExpected(t, td, "testdata/fixture/common/expected.txt", "expected body\n")
}

func TestCopyDir(t *testing.T) {
	dest := t.TempDir()
	CopyDir(t, td, "testdata/fixture/common/tree", dest)
	for rel, want := range map[string]string{"top.txt": "top\n", "dir/inner.txt": "inner\n"} {
		b, err := os.ReadFile(filepath.Join(dest, rel))
		if err != nil || string(b) != want {
			t.Errorf("%s = %q, err %v, want %q", rel, b, err, want)
		}
	}
}

// [<] 🤖🤖
