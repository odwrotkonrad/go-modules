package spec

// [>] 🤖🤖

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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

// a ref missing the @ prefix, ::<profile>, or the url part errors.
func TestResolvePluginRefMalformed(t *testing.T) {
	for _, entry := range []string{"@nourl", "@::p", "@url::", "url::p"} {
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

// String renders the canonical entry form.
func TestPluginRefString(t *testing.T) {
	ref := PluginRef{URL: "git@gitlab.com:g/r.git", Profile: "p"}
	if got := ref.String(); got != "@git@gitlab.com:g/r.git::p" {
		t.Errorf("String() = %q", got)
	}
}

// ExecIfPass gates on the named profile's execIf; undefined profile errors.
func TestExecIfPass(t *testing.T) {
	s, _ := specFile(t, "p:\n  options:\n    execIf: ['env:X']\n")
	eval := NewEvaluator().EvalExecIf
	t.Setenv("X", "")
	if ok, err := s.ExecIfPass("p", false, eval); err != nil || ok {
		t.Errorf("unset env: pass = %v, err = %v, want false, nil", ok, err)
	}
	t.Setenv("X", "1")
	if ok, err := s.ExecIfPass("p", false, eval); err != nil || !ok {
		t.Errorf("set env: pass = %v, err = %v, want true, nil", ok, err)
	}
	if _, err := s.ExecIfPass("nope", false, eval); err == nil {
		t.Error("undefined profile: expected error")
	}
}

// [<] 🤖🤖
