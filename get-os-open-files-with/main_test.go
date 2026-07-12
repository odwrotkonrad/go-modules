package main

// [>] 🤖🤖

import (
	"embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/lib/climain"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

//go:embed all:testdata
var td embed.FS

// configDir writes raw as the user config (system dir absent), returns the dir.
func configDir(t *testing.T, raw string) string {
	t.Helper()
	testyml.Swap(t, &yamlcfg.SystemDir, filepath.Join(t.TempDir(), "no-system"))
	dir := t.TempDir()
	if raw != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, configName), []byte(raw), 0o644))
	}
	return dir
}

func TestRun(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/cmds/get-os-open-files-with.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		raw := ""
		if cfg := c.Input.Args.String(t, 0); cfg != "" {
			raw = testyml.ReadFile(t, td, cfg)
		}
		out, err := run(c.Context.CommandArgs(), configDir(t, raw))
		if c.Expected.Check(t, err) {
			return
		}
		testyml.EqualExpected(t, td, c.Expected.Files, out)
	})
}

type helpVersionWant struct {
	Usage bool `yaml:"usage"`
	Done  bool `yaml:"done"`
}

func TestHelpVersion(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/help_version.test.spec.yml", func(t *testing.T, c testyml.Case[helpVersionWant]) (helpVersionWant, error) {
		out, done := climain.HelpVersion(c.Input.Args.Strings(t, 0), usage, "get-os-open-files-with", version)
		return helpVersionWant{Usage: out == usage, Done: done}, nil
	})
}

//[<] 🤖🤖
