package host

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/config"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

func fixtureHost(t *testing.T) Host {
	t.Helper()
	dir := testutil.Tree(t, map[string]string{
		"root/Library/LaunchDaemons/otelcol.plist.ontoHost.cp":                   "<plist/>\n",
		"root/Library/LaunchAgents/gitlab-runner.plist.ontoHost.tpl":             "<plist/>\n",
		"root/HOME/Library/LaunchAgents/load-defaults-config.plist.ontoHost.tpl": "<plist/>\n",
	})
	return New(dir, "/Users/x", "desktop/macos", config.Config{})
}

func TestResolveDomains(t *testing.T) {
	h := fixtureHost(t)
	svcs, err := h.ResolveServices([]string{"otelcol", "gitlab-runner", "load-defaults-config"})
	require.NoError(t, err)
	gui := fmt.Sprintf("gui/%d", os.Getuid())

	// system daemon
	assert.Equal(t, "system", svcs[0].Domain)
	assert.True(t, svcs[0].Sudo, "otelcol wants sudo")
	assert.Equal(t, "/Library/LaunchDaemons/otelcol.plist", svcs[0].Plist)
	// system launch agent (root/Library/LaunchAgents)
	assert.Equal(t, gui, svcs[1].Domain)
	assert.False(t, svcs[1].Sudo, "gitlab-runner wants no sudo")
	assert.Equal(t, "/Library/LaunchAgents/gitlab-runner.plist", svcs[1].Plist)
	// user launch agent (root/HOME/Library/LaunchAgents)
	assert.Equal(t, gui, svcs[2].Domain)
	assert.False(t, svcs[2].Sudo, "load-defaults-config wants no sudo")
	assert.Equal(t, "/Users/x/Library/LaunchAgents/load-defaults-config.plist", svcs[2].Plist)
	for _, s := range svcs {
		assert.True(t, s.LongRunning, "%s not marked long-running", s.Name)
	}
}

func TestResolveUnknown(t *testing.T) {
	h := fixtureHost(t)
	_, err := h.ResolveServices([]string{"nope"})
	assert.Error(t, err, "unknown service must error")
}

func TestParsePID(t *testing.T) {
	out := "system/otelcol = {\n\tactive count = 1\n\tpid = 4242\n\tstate = running\n}\n"
	pid, ok := ParsePID(out)
	require.True(t, ok)
	assert.Equal(t, 4242, pid)
	_, ok = ParsePID("no pid here")
	assert.False(t, ok, "ParsePID must not match a pid-less output")
}

// [<] 🤖🤖
