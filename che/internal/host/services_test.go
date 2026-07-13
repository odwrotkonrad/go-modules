package host

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// fixtureHost: one plist per service kind (system daemon, system agent, user agent).
func fixtureHost(t *testing.T) Host {
	t.Helper()
	dir := testutil.Tree(t, map[string]string{
		"root/Library/LaunchDaemons/otelcol.plist.ontoHost.cp":                   "<plist/>\n",
		"root/Library/LaunchAgents/gitlab-runner.plist.ontoHost.tpl":             "<plist/>\n",
		"root/HOME/Library/LaunchAgents/load-defaults-config.plist.ontoHost.tpl": "<plist/>\n",
	})
	return New(dir, filepath.Join(dir, "root"), "/Users/x", "desktop/macos", options.Options{})
}

// svcWant mirrors Service for spec equality; "${GUI}" expands to gui/<uid>.
type svcWant struct {
	Domain      string `yaml:"domain"`
	Sudo        bool   `yaml:"sudo"`
	Plist       string `yaml:"plist"`
	LongRunning bool   `yaml:"longRunning"`
}

func TestResolveServices(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/resolve_services.test.spec.yml", func(t *testing.T, c testyml.Case[[]svcWant]) {
		svcs, err := fixtureHost(t).ResolveServices(c.Input.Args.Strings(t, 0))
		if c.Expected.Check(t, err) {
			return
		}
		vars := map[string]string{"GUI": fmt.Sprintf("gui/%d", os.Getuid())}
		want := c.Expected.Output
		for i := range want {
			want[i].Domain = testyml.Expand(want[i].Domain, vars)
		}
		got := make([]svcWant, len(svcs))
		for i, s := range svcs {
			got[i] = svcWant{s.Domain, s.Sudo, s.Plist, s.LongRunning}
		}
		assert.Equal(t, want, got)
	})
}

type pidWant struct {
	Pid int  `yaml:"pid"`
	Ok  bool `yaml:"ok"`
}

func TestParsePID(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/parse_pid.test.spec.yml", func(t *testing.T, c testyml.Case[pidWant]) (pidWant, error) {
		pid, ok := ParsePID(c.Input.Args.String(t, 0))
		return pidWant{pid, ok}, nil
	})
}

// [<] 🤖🤖
