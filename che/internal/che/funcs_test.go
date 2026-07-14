package che

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

func TestToDest(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/to_dest.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		p := &ProfileReady{home: "/Users/x"}
		return p.toDest(c.Input.Args.String(t, 0)), nil
	})
}

func TestSrc(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/src.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		a := c.Input.Args
		p := newProfile(a.String(t, 0), "/Users/x", options.Options{}).withDir(a.String(t, 0))
		return p.resolveSrc(a.String(t, 1)), nil
	})
}

// TestResolveScripts: args name the fixture scripts to create and the rels to
// resolve; expected.output rels are joined under the fixture dir.
func TestResolveScripts(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/resolve_scripts.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) {
		a := c.Input.Args
		files := map[string]string{}
		for _, rel := range a.Strings(t, 0) {
			files[rel] = "#!/bin/sh\n"
		}
		dir := testutil.Tree(t, files)
		p := newProfile(dir, "/Users/x", options.Options{}).withDir(dir)
		got, err := p.resolveScripts(a.Strings(t, 1))
		if c.Expected.Check(t, err) {
			return
		}
		want := make([]string, len(c.Expected.Output))
		for i, rel := range c.Expected.Output {
			want[i] = filepath.Join(dir, rel)
		}
		assert.Equal(t, want, got)
	})
}

// svcWant mirrors Service for spec equality; "${GUI}" expands to gui/<uid>.
type svcWant struct {
	Domain      string `yaml:"domain"`
	Sudo        bool   `yaml:"sudo"`
	Plist       string `yaml:"plist"`
	LongRunning bool   `yaml:"longRunning"`
}

// servicesProfile: one plist per service kind (system daemon, system agent, user agent).
func servicesProfile(t *testing.T) *ProfileReady {
	t.Helper()
	dir := testutil.Tree(t, map[string]string{
		"root/Library/LaunchDaemons/otelcol.plist.ontoHost.cp":                    "<plist/>\n",
		"root/Library/LaunchAgents/gitlab-runner.plist.ontoHost.tpl":              "<plist/>\n",
		"root/_home/Library/LaunchAgents/load-defaults-config.plist.ontoHost.tpl": "<plist/>\n",
	})
	p := newProfile(dir, "/Users/x", options.Options{}).withDir(dir)
	p.Source.ProfileName = "desktop/macos"
	p.ref = "desktop/macos"
	return p
}

func TestResolveServices(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/resolve_services.test.spec.yml", func(t *testing.T, c testyml.Case[[]svcWant]) {
		svcs, err := servicesProfile(t).resolveServices(c.Input.Args.Strings(t, 0))
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

// [<] 🤖🤖
