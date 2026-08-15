package e2e

// [>] 🤖🤖

import (
	"embed"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

type runCfgGot struct {
	Mode      string `yaml:"mode"`
	DarwinVM  bool   `yaml:"darwinVM,omitempty"`
	LinuxArch string `yaml:"linuxArch,omitempty"`
}

func TestResolveRunCfg(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_run_cfg.test.spec.yml", func(t *testing.T, c testyml.Case[runCfgGot]) (runCfgGot, error) {
		a := c.Input.Args
		got, err := resolveRunCfg(a.String(t, 0), a.String(t, 1), a.String(t, 2), a.String(t, 3), a.Bool(t, 4), a.Bool(t, 5))
		if err != nil {
			return runCfgGot{}, err
		}
		return runCfgGot{Mode: got.mode, DarwinVM: got.darwinVM, LinuxArch: got.linuxArch}, nil
	})
}

// [<] 🤖🤖
