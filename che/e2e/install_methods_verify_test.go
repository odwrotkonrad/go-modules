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
	v := resolveVerify(t, e, "foo", "apt")
	require.Equal(t, "foo --version 2>/dev/null || foo version", v.cmd)
	require.True(t, v.wantOut)
}

func TestResolveVerifyUsesEntryCommand(t *testing.T) {
	e := parseEntry(t, `{command: foox, installers: [apt]}`)
	v := resolveVerify(t, e, "foo", "apt")
	require.Equal(t, "foox --version 2>/dev/null || foox version", v.cmd)
}

func TestResolveVerifyEntryStrategyAppliesToEveryMethod(t *testing.T) {
	e := parseEntry(t, `{verify: pkgVersionCmd, installers: [apt, brew]}`)
	require.Equal(t, `dpkg-query -W -f '${Version}\n' foo`, resolveVerify(t, e, "foo", "apt").cmd)
	require.Equal(t, "brew list --versions foo", resolveVerify(t, e, "foo", "brew").cmd)
}

func TestResolveVerifyItemOverridesEntry(t *testing.T) {
	e := parseEntry(t, `
verify: pkgVersionCmd
installers:
  - brew
  - apt:
      verify: {cmd: test -x /usr/bin/foo}
`)
	require.Equal(t, "test -x /usr/bin/foo", resolveVerify(t, e, "foo", "apt").cmd)
	require.Equal(t, "brew list --versions foo", resolveVerify(t, e, "foo", "brew").cmd)
}

func TestResolveVerifyCmdChecksExitCodeOnly(t *testing.T) {
	e := parseEntry(t, `{verify: {cmd: test -e /etc/foo}, installers: [apt]}`)
	v := resolveVerify(t, e, "foo", "apt")
	require.Equal(t, "test -e /etc/foo", v.cmd)
	require.False(t, v.wantOut)
}

func TestResolveVerifyPkgVersionCmdUsesManagerPackageName(t *testing.T) {
	e := parseEntry(t, `
verify: pkgVersionCmd
installers:
  - apt:
      packageName: libfoo-dev
  - brew:
      packageName: foox
`)
	require.Equal(t, `dpkg-query -W -f '${Version}\n' libfoo-dev`, resolveVerify(t, e, "foo", "apt").cmd)
	require.Equal(t, "brew list --versions foox", resolveVerify(t, e, "foo", "brew").cmd)
}

func TestResolveVerifyLegacyVersionCommandStillWins(t *testing.T) {
	e := parseEntry(t, `{versionCommand: foo status, installers: [apt]}`)
	v := resolveVerify(t, e, "foo", "apt")
	require.Equal(t, "foo status", v.cmd)
	require.True(t, v.wantOut)
}

func TestPkgVersionCmdRejectsUnsupportedManager(t *testing.T) {
	e := parseEntry(t, `[go]`)
	_, err := pkgVersionCmd(e, "foo", "go")
	require.ErrorContains(t, err, "no version query for method go")
}

// [<] 🤖🤖
