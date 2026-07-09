package config

// [>] 🤖🤖

import (
	"embed"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func TestIsOptionEqualTo(t *testing.T) {
	type cfgIn struct {
		Dir         string
		DryRun      int `yaml:"dryRun"`
		Profile     string
		SkipExecIf  bool `yaml:"skipExecIf"`
		SkipPlugins bool `yaml:"skipPlugins"`
		Debug       bool
	}
	type in struct {
		Config cfgIn
		Option string
		Val    any
	}
	type want struct {
		Equal bool
		Panic bool
	}
	type c struct {
		Name string
		In   in
		Want want
	}
	testyml.Run(t, td, "testdata/spec/is_option_equal_to.spec.yml", func(t *testing.T, c c) {
		cfg := Config{
			Dir:         c.In.Config.Dir,
			DryRun:      DryRunMode(c.In.Config.DryRun),
			Profile:     c.In.Config.Profile,
			SkipExecIf:  c.In.Config.SkipExecIf,
			SkipPlugins: c.In.Config.SkipPlugins,
			Debug:       c.In.Config.Debug,
		}
		opt := Option(c.In.Option)
		val := c.In.Val
		if opt == OptionDryRun {
			val = DryRunMode(val.(int))
		}
		if c.Want.Panic {
			defer func() {
				if recover() == nil {
					t.Errorf("IsOptionEqualTo(%q) did not panic", opt)
				}
			}()
		}
		if got := cfg.IsOptionEqualTo(opt, val); got != c.Want.Equal {
			t.Errorf("IsOptionEqualTo(%q, %v) = %v, want %v", opt, val, got, c.Want.Equal)
		}
	})
}

// [<] 🤖🤖
