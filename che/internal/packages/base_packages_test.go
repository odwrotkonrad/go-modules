package packages

// [>] 🤖🤖

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const basePackagesYaml = `basePackages:
  common: [curl]
  apt: [ca-certificates, gnupg]
packages:
  curl: [brew, apt]
  ca-certificates: [apt]
  gnupg: [brew, apt]
  jq: [brew, apt]
  bat: [brew, apt]
`

func TestBasePackagesInstallBeforeMethod(t *testing.T) {
	in, m := newInstaller(t, basePackagesYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = failOn("dpkg -s")
	require.NoError(t, in.Install([]string{"jq"}))
	calls := strings.Join(m.Calls(), "\n")
	curlAt := strings.Index(calls, "--no-install-recommends curl")
	gnupgAt := strings.Index(calls, "--no-install-recommends gnupg")
	jqAt := strings.Index(calls, "--no-install-recommends jq")
	require.Positive(t, curlAt)
	require.Positive(t, gnupgAt)
	require.Positive(t, jqAt)
	require.Less(t, curlAt, jqAt)
	require.Less(t, gnupgAt, jqAt)
}

func TestBasePackagesVscodeInstallsCodeFirst(t *testing.T) {
	const y = `basePackages:
  vscode: [code]
packages:
  code:
    installers: [{brew/cask: {packageName: visual-studio-code}}]
  golang.go:
    version: "0.50.0"
    installers: [vscode]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"brew", "code"}), Options{})
	m.Stub = failOn("brew list")
	require.NoError(t, in.Install([]string{"golang.go"}))
	calls := strings.Join(m.Calls(), "\n")
	caskAt := strings.Index(calls, "brew install --cask visual-studio-code")
	extAt := strings.Index(calls, "--install-extension golang.go@0.50.0")
	require.Positive(t, caskAt)
	require.Positive(t, extAt)
	require.Less(t, caskAt, extAt)
}

func TestBasePackagesInstallOncePerRun(t *testing.T) {
	in, m := newInstaller(t, basePackagesYaml, "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = failOn("dpkg -s")
	require.NoError(t, in.Install([]string{"jq", "bat"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Equal(t, 1, strings.Count(calls, "--no-install-recommends ca-certificates"))
	require.Equal(t, 1, strings.Count(calls, "--no-install-recommends curl"))
}

func TestBasePackagesCommonOnlyForOtherMethods(t *testing.T) {
	in, m := newInstaller(t, basePackagesYaml, "darwin", cmdMap([]string{"brew"}), Options{})
	m.Stub = failOn("brew list")
	require.NoError(t, in.Install([]string{"jq"}))
	requireCalls(t, m, "brew install curl")
	refuteCalls(t, m, "brew install ca-certificates")
}

func TestBasePackagesSkippedOnDryRun(t *testing.T) {
	in, m := newInstaller(t, basePackagesYaml, "linux", cmdMap([]string{"apt-get"}), Options{DryRun: true})
	m.Stub = failOn("dpkg -s")
	require.NoError(t, in.Install([]string{"jq"}))
	refuteCalls(t, m, "--no-install-recommends curl")
}

func TestBasePackagesAbsentIsNoop(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  jq: [apt]\n", "linux", cmdMap([]string{"apt-get"}), Options{})
	m.Stub = failOn("dpkg -s")
	require.NoError(t, in.Install([]string{"jq"}))
	requireCalls(t, m, "--no-install-recommends jq")
}

// [<] 🤖🤖
