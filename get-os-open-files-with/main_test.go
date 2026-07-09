// [>] 🤖🤖
package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/climain"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

func writeConfig(t *testing.T, raw string) string {
	t.Helper()
	yamlcfg.SystemDir = filepath.Join(t.TempDir(), "no-system")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, configName), []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func codeOf(err error) int {
	var ce *yamlcfg.CodedError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return -1
}

func TestPositive(t *testing.T) {
	cases := []struct {
		name string
		cfg  string
		want string
	}{
		{
			"single_app_single_role",
			"com.example.Editor:\n  all: [public.python-script, public.json]\n",
			"com.example.Editor public.python-script all\ncom.example.Editor public.json all",
		},
		{
			"one_app_multiple_roles",
			"com.example.Viewer:\n  all: [com.adobe.pdf]\n  viewer: [com.microsoft.word.doc]\n",
			"com.example.Viewer com.adobe.pdf all\ncom.example.Viewer com.microsoft.word.doc viewer",
		},
		{
			"multiple_apps",
			"com.a.One:\n  all: [public.json]\ncom.b.Two:\n  editor: [public.yaml]\n",
			"com.a.One public.json all\ncom.b.Two public.yaml editor",
		},
		{"empty_config", "{}\n", ""},
		{"app_with_empty_role_list", "com.example.Editor:\n  all: []\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := writeConfig(t, c.cfg)
			out, err := run(nil, dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out != c.want {
				t.Errorf("got %q want %q", out, c.want)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	t.Run("too_many_args", func(t *testing.T) {
		dir := writeConfig(t, "{}\n")
		_, err := run([]string{"one", "two"}, dir)
		if codeOf(err) != 11 {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("three_args", func(t *testing.T) {
		dir := writeConfig(t, "{}\n")
		_, err := run([]string{"one", "two", "three"}, dir)
		if codeOf(err) != 11 {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("missing_config", func(t *testing.T) {
		yamlcfg.SystemDir = filepath.Join(t.TempDir(), "no-system")
		dir := t.TempDir()
		_, err := run(nil, dir)
		if codeOf(err) != 13 {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("invalid_config_syntax", func(t *testing.T) {
		dir := writeConfig(t, "handlers: [unclosed\n")
		_, err := run(nil, dir)
		if codeOf(err) != 12 {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("invalid_config_null_roles", func(t *testing.T) {
		dir := writeConfig(t, "com.example.Editor: null\n")
		_, err := run(nil, dir)
		if codeOf(err) != 12 {
			t.Fatalf("got %v", err)
		}
	})
}

func TestHelp(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		out, done := climain.HelpVersion([]string{flag}, usage, "get-os-open-files-with", version)
		if !done {
			t.Fatalf("%s: not handled", flag)
		}
		if out != usage {
			t.Errorf("%s: usage mismatch", flag)
		}
	}
}

//[<] 🤖🤖
