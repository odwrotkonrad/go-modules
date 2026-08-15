package e2e

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

type verifyCmdGot struct {
	Cmd     string `yaml:"cmd"`
	WantOut bool   `yaml:"wantOut"`
}

func TestResolveVerify(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_verify.test.spec.yml", func(t *testing.T, c testyml.Case[[]verifyCmdGot]) ([]verifyCmdGot, error) {
		var entry packages.Entry
		require.NoError(t, yaml.Unmarshal([]byte(c.Input.Args.String(t, 0)), &entry))
		pkg, method := c.Input.Args.String(t, 1), c.Input.Args.String(t, 2)
		if c.Input.Args.Bool(t, 3) {
			if _, err := pkgMgrVersionCheck(entry, pkg, method); err != nil {
				return nil, err
			}
			t.Fatal("pkgMgrVersionCheck cases assert errors only")
		}
		pv := resolveVerify(t, entry, pkg, method)
		got := make([]verifyCmdGot, 0, len(pv.cmds))
		for _, vc := range pv.cmds {
			got = append(got, verifyCmdGot{Cmd: vc.cmd, WantOut: vc.wantOut})
		}
		return got, nil
	})
}

// [<] 🤖🤖
