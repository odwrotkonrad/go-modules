// [>] 🤖🤖
package main

import (
	"embed"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/climain"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

//go:embed all:testdata
var td embed.FS

func configDir(t *testing.T, raw string) string {
	t.Helper()
	yamlcfg.SystemDir = filepath.Join(t.TempDir(), "no-system")
	dir := t.TempDir()
	if raw != "" {
		if err := os.WriteFile(filepath.Join(dir, configName), []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func checkCoded(t *testing.T, w testyml.Want, err error) bool {
	t.Helper()
	if !w.IsErrorWanted() {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return false
	}
	w.CheckErr(t, err)
	if got := yamlcfg.Code(err); w.ExitCode != 0 && got != w.ExitCode {
		t.Fatalf("Code = %d (%v), want %d", got, err, w.ExitCode)
	}
	return true
}

func TestRun(t *testing.T) {
	type in struct {
		Config string
		Args   []string
	}
	type c struct {
		Name string
		In   in
		Want testyml.Want
	}
	testyml.Run(t, td, "testdata/spec/run.spec.yml", func(t *testing.T, c c) {
		raw := ""
		if c.In.Config != "" {
			raw = testyml.ReadFile(t, td, c.In.Config)
		}
		out, err := run(c.In.Args, configDir(t, raw))
		if checkCoded(t, c.Want, err) {
			return
		}
		testyml.EqualExpected(t, td, c.Want.FilesOut, out)
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
