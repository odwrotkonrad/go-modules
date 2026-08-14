package e2e

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
)

func parseEntry(t *testing.T, yml string) packages.Entry {
	t.Helper()
	var e packages.Entry
	require.NoError(t, yaml.Unmarshal([]byte(yml), &e))
	return e
}

func TestResolveVerifyDefaultsToVersionCmd(t *testing.T) {
	e := parseEntry(t, `[apt]`)
	pv := resolveVerify(t, e, "foo", "apt")
	require.Len(t, pv.cmds, 1)
	require.Equal(t, "foo --version 2>/dev/null || foo version", pv.cmds[0].cmd)
	require.True(t, pv.cmds[0].wantOut)
}

func TestResolveVerifyUsesEntryCommand(t *testing.T) {
	e := parseEntry(t, `{command: foox, installers: [apt]}`)
	pv := resolveVerify(t, e, "foo", "apt")
	require.Equal(t, "foox --version 2>/dev/null || foox version", pv.cmds[0].cmd)
}

func TestResolveVerifyEntryStrategyAppliesToEveryMethod(t *testing.T) {
	e := parseEntry(t, `{verify: pkgMgrVersionCheck, installers: [apt, brew]}`)
	require.Equal(t, `dpkg-query -W -f '${Version}\n' foo`, resolveVerify(t, e, "foo", "apt").cmds[0].cmd)
	require.Equal(t, "brew list --versions foo", resolveVerify(t, e, "foo", "brew").cmds[0].cmd)
}

func TestResolveVerifyItemOverridesEntry(t *testing.T) {
	e := parseEntry(t, `
verify: pkgMgrVersionCheck
installers:
  - brew
  - apt:
      verify: {cmd: test -x /usr/bin/foo}
`)
	require.Equal(t, "test -x /usr/bin/foo", resolveVerify(t, e, "foo", "apt").cmds[0].cmd)
	require.Equal(t, "brew list --versions foo", resolveVerify(t, e, "foo", "brew").cmds[0].cmd)
}

func TestResolveVerifyCmdChecksExitCodeOnly(t *testing.T) {
	e := parseEntry(t, `{verify: {cmd: test -e /etc/foo}, installers: [apt]}`)
	pv := resolveVerify(t, e, "foo", "apt")
	require.Len(t, pv.cmds, 1)
	require.Equal(t, "test -e /etc/foo", pv.cmds[0].cmd)
	require.False(t, pv.cmds[0].wantOut)
}

func TestResolveVerifyPkgMgrVersionCheckUsesManagerPackageName(t *testing.T) {
	e := parseEntry(t, `
verify: pkgMgrVersionCheck
installers:
  - apt:
      packageName: libfoo-dev
  - brew:
      packageName: foox
`)
	require.Equal(t, `dpkg-query -W -f '${Version}\n' libfoo-dev`, resolveVerify(t, e, "foo", "apt").cmds[0].cmd)
	require.Equal(t, "brew list --versions foox", resolveVerify(t, e, "foo", "brew").cmds[0].cmd)
}

func TestResolveVerifyLegacyVersionCommandStillWins(t *testing.T) {
	e := parseEntry(t, `{versionCommand: foo status, installers: [apt]}`)
	pv := resolveVerify(t, e, "foo", "apt")
	require.Equal(t, "foo status", pv.cmds[0].cmd)
	require.True(t, pv.cmds[0].wantOut)
}

func TestResolveVerifyCombinesMultipleStrategyKeys(t *testing.T) {
	e := parseEntry(t, `{verify: {pkgMgrVersionCheck: true, cmd: test -e /etc/foo}, installers: [apt]}`)
	pv := resolveVerify(t, e, "foo", "apt")
	require.Len(t, pv.cmds, 2)
	require.Equal(t, `dpkg-query -W -f '${Version}\n' foo`, pv.cmds[0].cmd)
	require.True(t, pv.cmds[0].wantOut)
	require.Equal(t, "test -e /etc/foo", pv.cmds[1].cmd)
	require.False(t, pv.cmds[1].wantOut)
}

func TestPkgMgrVersionCheckRejectsUnsupportedManager(t *testing.T) {
	e := parseEntry(t, `[go]`)
	_, err := pkgMgrVersionCheck(e, "foo", "go")
	require.ErrorContains(t, err, "no version query for method go")
}

// [<] 🤖🤖
