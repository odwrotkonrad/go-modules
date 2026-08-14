package packages

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func parseFile(t *testing.T, yml string) *File {
	t.Helper()
	f := &File{}
	require.NoError(t, yaml.Unmarshal([]byte(yml), f))
	return f
}

func TestVerifyParsesStrategyOnEntry(t *testing.T) {
	f := parseFile(t, `packages:
  foo:
    verify: pkgVersionCmd
    installers: [apt]
`)
	v := f.Packages["foo"].Verify
	require.NotNil(t, v)
	require.True(t, v.PkgVersionCmd)
	require.Empty(t, v.Cmd)
}

func TestVerifyParsesStrategyOnItem(t *testing.T) {
	f := parseFile(t, `packages:
  foo:
    - apt:
        verify: versionCmd
  bar:
    - brew:
        packageName: barx
        verify: pkgVersionCmd
  baz:
    - binariesRemoteArchive:
        url: https://example.com/{version}.tar.gz
        version: "1.0"
        verify: {cmd: baz doctor}
`)
	require.True(t, f.Packages["foo"].Items[0].Verify.VersionCmd)
	require.True(t, f.Packages["bar"].Items[0].Verify.PkgVersionCmd)
	require.Equal(t, "baz doctor", f.Packages["baz"].Items[0].Verify.Cmd)
}

func TestVerifyParsesCmdObject(t *testing.T) {
	f := parseFile(t, `packages:
  foo:
    verify:
      cmd: test -e /etc/foo.conf
    installers: [apt]
`)
	v := f.Packages["foo"].Verify
	require.NotNil(t, v)
	require.False(t, v.VersionCmd)
	require.False(t, v.PkgVersionCmd)
	require.Equal(t, "test -e /etc/foo.conf", v.Cmd)
}

func TestVerifyRejectsUnknownStrategy(t *testing.T) {
	err := yaml.Unmarshal([]byte(`packages:
  foo:
    verify: nope
    installers: [apt]
`), &File{})
	require.ErrorContains(t, err, "verify: want versionCmd, pkgVersionCmd or {versionCmd|pkgVersionCmd|cmd|checkInPath}")
}

func TestVerifyRejectsEmptyCmd(t *testing.T) {
	err := yaml.Unmarshal([]byte(`packages:
  foo:
    verify: {cmd: ""}
    installers: [apt]
`), &File{})
	require.ErrorContains(t, err, "verify: object form requires versionCmd, pkgVersionCmd, cmd, or checkInPath")
}

func TestVerifyRoundTrips(t *testing.T) {
	const yml = `packages:
    bar:
        - apt:
            verify:
                cmd: test -x /usr/bin/bar
    foo:
        verify: pkgVersionCmd
        installers:
            - apt
`
	f := parseFile(t, yml)
	out, err := f.YAML()
	require.NoError(t, err)
	require.YAMLEq(t, yml, out)
}

// [<] 🤖🤖
