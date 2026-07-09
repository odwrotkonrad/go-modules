package spec

// [>] 🤖🤖

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

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

// plugins entries collect into Resolved.Plugins (declaration order, deduped
// on url+profile, first occurrence's env wins), local mixins still merge.
func TestResolvePluginRefs(t *testing.T) {
	s, dir := specFile(t, `
base:
  include:
    services: [svc]
main:
  mixinProfiles: [base]
  plugins:
    - "@git@gitlab.com:g/r.git::p"
    - ref: "@https://gitlab.com/g/r2.git::q"
      env:
        GITLAB_GROUP: konradodwrot
    - ref: "@git@gitlab.com:g/r.git::p"
      env:
        IGNORED: dup
`)
	res, err := s.Resolve([]string{"main"}, filepath.Join(dir, "root"))
	if err != nil {
		t.Fatal(err)
	}
	want := []PluginRef{
		{URL: "git@gitlab.com:g/r.git", Profile: "p"},
		{URL: "https://gitlab.com/g/r2.git", Profile: "q", Env: map[string]string{"GITLAB_GROUP": "konradodwrot"}},
	}
	if len(res.Plugins) != len(want) {
		t.Fatalf("Plugins = %v, want %v", res.Plugins, want)
	}
	for i, w := range want {
		got := res.Plugins[i]
		if got.URL != w.URL || got.Profile != w.Profile || !maps.Equal(got.Env, w.Env) {
			t.Errorf("Plugins[%d] = %v, want %v", i, got, w)
		}
	}
	if !slices.Contains(res.Services, "svc") {
		t.Errorf("local mixin not merged: Services = %v", res.Services)
	}
}

// a ref missing ::<profile> or the url/dir part errors.
func TestResolvePluginRefMalformed(t *testing.T) {
	for _, entry := range []string{"@nourl", "nodir", "@::p", "::p", "@url::", "dir::"} {
		s, dir := specFile(t, "main:\n  plugins: [\""+entry+"\"]\n")
		if _, err := s.Resolve([]string{"main"}, filepath.Join(dir, "root")); err == nil {
			t.Errorf("Resolve with plugins entry %q: expected error", entry)
		}
	}
}

// a @url::profile ref in mixinProfiles is no longer special: it errors as an
// undefined profile.
func TestMixinPluginRefUndefined(t *testing.T) {
	s, dir := specFile(t, "main:\n  mixinProfiles: [\"@git@gitlab.com:g/r.git::p\"]\n")
	_, err := s.Resolve([]string{"main"}, filepath.Join(dir, "root"))
	if err == nil || !strings.Contains(err.Error(), "undefined profile") {
		t.Errorf("err = %v, want undefined profile error", err)
	}
}

// String round-trips the canonical entry forms (remote @-prefixed, dir bare).
func TestPluginRefString(t *testing.T) {
	for _, entry := range []string{"@git@gitlab.com:g/r.git::p", "./rel::p", "/abs/dir::p"} {
		ref, err := parsePluginRef(entry)
		if err != nil {
			t.Fatalf("parsePluginRef(%q) errored: %v", entry, err)
		}
		if got := ref.String(); got != entry {
			t.Errorf("String() = %q, want %q", got, entry)
		}
	}
}

// the @ prefix decides the ref kind: bare -> local dir path, @ -> remote git.
func TestPluginRefIsPath(t *testing.T) {
	cases := map[string]bool{
		"/abs/dir::p":                      true,
		"./rel::p":                         true,
		"rel/dir::p":                       true,
		"~/x::p":                           true,
		"$HOME/x::p":                       true,
		"@https://gitlab.com/g/r.git::p":   false,
		"@ssh://git@gitlab.com/g/r.git::p": false,
		"@git@gitlab.com:g/r.git::p":       false,
		"@file:///tmp/x::p":                false,
	}
	for entry, want := range cases {
		ref, err := parsePluginRef(entry)
		if err != nil {
			t.Fatalf("parsePluginRef(%q) errored: %v", entry, err)
		}
		if ref.IsPath != want {
			t.Errorf("parsePluginRef(%q).IsPath = %v, want %v", entry, ref.IsPath, want)
		}
	}
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
