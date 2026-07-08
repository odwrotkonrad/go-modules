package host

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

func fixtureHost(t *testing.T) Host {
	t.Helper()
	dir := t.TempDir()
	testutil.WriteTree(t, dir, map[string]string{
		"root/Library/LaunchDaemons/otelcol.plist.ontoHost.cp":                   "<plist/>\n",
		"root/Library/LaunchAgents/gitlab-runner.plist.ontoHost.tpl":             "<plist/>\n",
		"root/HOME/Library/LaunchAgents/load-defaults-config.plist.ontoHost.tpl": "<plist/>\n",
	})
	return New(dir, "/Users/x", "desktop/macos", DryRunOff)
}

func TestResolveDomains(t *testing.T) {
	h := fixtureHost(t)
	svcs, err := h.ResolveServices([]string{"otelcol", "gitlab-runner", "load-defaults-config"})
	if err != nil {
		t.Fatal(err)
	}
	gui := fmt.Sprintf("gui/%d", os.Getuid())

	// system daemon
	if svcs[0].Domain != "system" || !svcs[0].Sudo {
		t.Errorf("otelcol = %+v, want system+sudo", svcs[0])
	}
	if svcs[0].Plist != "/Library/LaunchDaemons/otelcol.plist" {
		t.Errorf("otelcol plist = %q", svcs[0].Plist)
	}
	// system launch agent (root/Library/LaunchAgents)
	if svcs[1].Domain != gui || svcs[1].Sudo {
		t.Errorf("gitlab-runner = %+v, want %s no-sudo", svcs[1], gui)
	}
	if svcs[1].Plist != "/Library/LaunchAgents/gitlab-runner.plist" {
		t.Errorf("gitlab-runner plist = %q", svcs[1].Plist)
	}
	// user launch agent (root/HOME/Library/LaunchAgents)
	if svcs[2].Domain != gui || svcs[2].Sudo {
		t.Errorf("load-defaults-config = %+v, want %s no-sudo", svcs[2], gui)
	}
	if svcs[2].Plist != "/Users/x/Library/LaunchAgents/load-defaults-config.plist" {
		t.Errorf("load-defaults-config plist = %q", svcs[2].Plist)
	}
	for _, s := range svcs {
		if !s.LongRunning {
			t.Errorf("%s not marked long-running", s.Name)
		}
	}
}

func TestResolveUnknown(t *testing.T) {
	h := fixtureHost(t)
	if _, err := h.ResolveServices([]string{"nope"}); err == nil {
		t.Fatal("expected unknown-service error")
	}
}

func TestParsePID(t *testing.T) {
	out := "system/otelcol = {\n\tactive count = 1\n\tpid = 4242\n\tstate = running\n}\n"
	pid, ok := ParsePID(out)
	if !ok || pid != 4242 {
		t.Errorf("ParsePID = (%d, %v), want (4242, true)", pid, ok)
	}
	if _, ok := ParsePID("no pid here"); ok {
		t.Error("ParsePID matched a pid-less output")
	}
}

// [<] 🤖🤖
