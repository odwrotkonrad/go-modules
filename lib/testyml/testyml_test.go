package testyml

// [>] 🤖🤖

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
	if !ran["runShouldDecodeCaseAndInvokeCallback"] || !ran["runShouldDecodeNegativeValues"] {
		t.Errorf("ran = %v, want both cases", ran)
	}
}

func TestMatchers(t *testing.T) {
	type in struct {
		Args    []string
		NoMatch []string `yaml:"noMatch"`
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
		for _, m := range c.In.NoMatch {
			MustNotMatch(t, c.In.Args[0], m)
		}
	})
}

func TestCheckCode(t *testing.T) {
	type in struct {
		Code int
	}
	type c struct {
		Name string
		In   in
		Want Want
	}
	Run(t, td, "testdata/spec/check_code.spec.yml", func(t *testing.T, c c) {
		c.Want.CheckCode(t, c.In.Code)
	})
}

func TestMatch(t *testing.T) {
	if !IsMatch("ln(create): /etc/zshrc", "ln(create)") {
		t.Error("literal matcher failed")
	}
	if !IsMatch("ln(create): /etc/zshrc", `ln(create): /etc/{{/\w+/}}`) {
		t.Error("hole matcher failed")
	}
	if IsMatch("ln(create): /etc/zshrc", "ln.create.") {
		t.Error("literal dots must not act as regex")
	}
	if IsMatch("cp: /x", "ln(create)") {
		t.Error("matcher matched wrongly")
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
	CopyDir(t, td, "testdata/fixture/common/tree-nested", dest)
	for rel, want := range map[string]string{"top.txt": "top\n", "dir/inner.txt": "inner\n"} {
		b, err := os.ReadFile(filepath.Join(dest, rel))
		if err != nil || string(b) != want {
			t.Errorf("%s = %q, err %v, want %q", rel, b, err, want)
		}
	}
}

// [<] 🤖🤖
