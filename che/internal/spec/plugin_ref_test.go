package spec

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestParsePluginRef(t *testing.T) {
	type in struct {
		Args []string
	}
	type want struct {
		URL     string `yaml:"url"`
		Profile string
		IsPath  bool `yaml:"isPath"`
		Error   bool
	}
	type c struct {
		Name string
		In   in
		Want want
	}
	testyml.Run(t, td, "testdata/spec/parse_plugin_ref.spec.yml", func(t *testing.T, c c) {
		entry := c.In.Args[0]
		ref, err := parsePluginRef(entry)
		if c.Want.Error {
			if err == nil {
				t.Fatalf("parsePluginRef(%q) = %+v, want error", entry, ref)
			}
			return
		}
		if err != nil {
			t.Fatalf("parsePluginRef(%q) errored: %v", entry, err)
		}
		if ref.URL != c.Want.URL || ref.Profile != c.Want.Profile || ref.IsPath != c.Want.IsPath {
			t.Errorf("parsePluginRef(%q) = %+v, want %+v", entry, ref, c.Want)
		}
		if got := ref.String(); got != entry {
			t.Errorf("String() = %q, want %q", got, entry)
		}
	})
}

// specFile writes body as che.yml in a temp dir, returns the loaded Raw.
func specFile(t *testing.T, body string) (*Raw, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "che.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(filepath.Join(dir, "che.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

// ExecIfPass gates on the named profile's execIf; undefined profile errors.
// A pass logs at normal level, a reject only at debug level.
func TestExecIfPass(t *testing.T) {
	s, _ := specFile(t, "p:\n  options:\n    execIf: ['env:X']\n")
	eval := NewEvaluator().EvalExecIf
	t.Setenv("X", "")
	out, _ := testutil.CaptureStdout(t, func() error {
		if ok, err := s.ExecIfPass("p", false, eval); err != nil || ok {
			t.Errorf("unset env: pass = %v, err = %v, want false, nil", ok, err)
		}
		return nil
	})
	testutil.NotLine(t, out, "execIf(reject)")
	log.SetDebug(true)
	t.Cleanup(func() { log.SetDebug(false) })
	out, _ = testutil.CaptureStdout(t, func() error {
		_, err := s.ExecIfPass("p", false, eval)
		return err
	})
	testutil.WantLines(t, out, "execIf(reject): profile p: env:X")
	t.Setenv("X", "1")
	out, _ = testutil.CaptureStdout(t, func() error {
		if ok, err := s.ExecIfPass("p", false, eval); err != nil || !ok {
			t.Errorf("set env: pass = %v, err = %v, want true, nil", ok, err)
		}
		return nil
	})
	testutil.WantLines(t, out, "execIf(pass): profile p: env:X")
	if _, err := s.ExecIfPass("nope", false, eval); err == nil {
		t.Error("undefined profile: expected error")
	}
}

// [<] 🤖🤖
