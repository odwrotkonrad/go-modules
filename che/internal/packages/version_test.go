package packages

// [>] 🤖🤖

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPinMatches(t *testing.T) {
	cases := []struct {
		out, pin string
		want     bool
	}{
		{"kind version 0.32.0", "0.32.0", true},
		{"kind version 0.32.0", "0.31.0", false},
		{"Terraform v1.15.8", "1.15.8", true},
		{"go version go1.26.4 darwin/arm64", "1.26.4", true},
		{"Python 3.14.5", "3.14.5", true},
		{"Python 3.14.5", "3.14.4", false},
		{"prometheus, version 3.5.5 (branch: HEAD, revision: dcd3d551ced8)", "3.5.5", true},
		{"glab 1.108.0 (5de20850)", "1.107.0", false},
		{"jq-1.7.1-3", "1.7.1-3", true},
		{"1:1.7.1-3", "1.7.1-3", true},
		{"anything", "", true},
		{"no version here", "1.0.0", false},
	}
	for _, c := range cases {
		require.Equalf(t, c.want, PinMatches(c.out, c.pin), "out=%q pin=%q", c.out, c.pin)
	}
}

const pinnedBatYaml = `packages:
  bat:
    version: "0.25.0"
    installMethods: [brew, apt]
`

func TestEntryVersionSkipsWhenMatching(t *testing.T) {
	in, m := newInstaller(t, pinnedBatYaml, "darwin", cmdMap([]string{"brew", "bat"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "bat" {
			return []byte("bat 0.25.0 (871abd2)\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"bat"}))
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "brew install")
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "brew upgrade")
}

func TestEntryVersionDriftRunsManagerUpdate(t *testing.T) {
	in, m := newInstaller(t, pinnedBatYaml, "darwin", cmdMap([]string{"brew", "bat"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "bat" {
			return []byte("bat 0.24.0\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"bat"}))
	require.Contains(t, m.Calls(), "brew upgrade bat")
}

func TestEntryVersionPinsArchiveVersion(t *testing.T) {
	const y = `packages:
  kind:
    version: "0.32.0"
    installMethods:
      - prebuiltArchive:
          url: https://example.com/kind-{version}
          sha256: {linux-amd64: goodsha}
`
	in, m := newInstaller(t, y, "linux", cmdMap([]string{"sha256sum"}), Options{})
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"kind"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "https://example.com/kind-0.32.0")
}

func TestArchiveWithoutVersionNeedsNoPin(t *testing.T) {
	const y = `packages:
  aws:
    - prebuiltArchive:
        url: https://example.com/aws-latest.zip
        platforms: [linux-amd64]
        bin: aws/dist/aws
`
	in, m := newInstaller(t, y, "linux", cmdMap(nil), Options{})
	require.NoError(t, in.Install([]string{"aws"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "aws-latest.zip")
}

// [<] 🤖🤖

func TestVscodeExtensionPinInstallsVersionedAndSkips(t *testing.T) {
	const y = `packages:
  golang.go:
    version: "0.50.0"
    installMethods: [{brew: {vscode: golang.go}}]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("")
	require.NoError(t, in.Install([]string{"golang.go"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "code --install-extension golang.go@0.50.0")

	in2, m2 := newInstaller(t, y, "darwin", cmdMap([]string{"code"}), Options{})
	m2.Stub = codeListStub("golang.go@0.50.0\n")
	require.NoError(t, in2.Install([]string{"golang.go"}))
	require.NotContains(t, strings.Join(m2.Calls(), "\n"), "--install-extension")
}

func TestVscodeExtensionPinDriftReinstalls(t *testing.T) {
	const y = `packages:
  golang.go:
    version: "0.50.0"
    installMethods: [{brew: {vscode: golang.go}}]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("golang.go@0.49.0\n")
	require.NoError(t, in.Install([]string{"golang.go"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "code --install-extension golang.go@0.50.0")
}

func TestBrewVersionedFormulaIsNamedDirectly(t *testing.T) {
	const y = `packages:
  node:
    version: "24"
    installMethods: [{brew: node@24}]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"brew"}), Options{})
	m.Stub = failOn("brew list")
	require.NoError(t, in.Install([]string{"node"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "brew install node@24")
	require.NotContains(t, calls, "node@24@24")
}

func TestUnversionedLatestSentinelSkipsPinning(t *testing.T) {
	const y = `packages:
  jq:
    version: __unversioned_latest__
    installMethods: [brew, apt]
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"brew", "jq"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "jq" {
			return []byte("jq-1.7.1\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"jq"}))
	calls := strings.Join(m.Calls(), "\n")
	require.NotContains(t, calls, "__unversioned_latest__")
	require.NotContains(t, calls, "brew install")
	require.NotContains(t, calls, "brew upgrade")
}

func TestUnversionedLatestSentinelNotUsedAsArchiveVersion(t *testing.T) {
	const y = `packages:
  kind:
    version: __unversioned_latest__
    installMethods:
      - prebuiltArchive:
          url: https://example.com/kind-{version}
          sha256: {linux-amd64: goodsha}
`
	in, _ := newInstaller(t, y, "linux", cmdMap([]string{"sha256sum"}), Options{})
	require.ErrorContains(t, in.Install([]string{"kind"}), "no version pinned")
}

func TestPerOsCommandResolvesHostBinary(t *testing.T) {
	const y = `packages:
  bat:
    version: __unversioned_latest__
    installMethods: [brew, apt]
    command:
      linux: batcat
`
	in, m := newInstaller(t, y, "linux", cmdMap([]string{"apt-get", "batcat"}), Options{})
	require.NoError(t, in.Install([]string{"bat"}))
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "apt-get install")

	mac, mm := newInstaller(t, y, "darwin", cmdMap([]string{"brew"}), Options{})
	mm.Stub = failOn("brew list")
	require.NoError(t, mac.Install([]string{"bat"}))
	require.Contains(t, strings.Join(mm.Calls(), "\n"), "brew install bat")
}

func TestAliasBinaryLinksRenamedBinary(t *testing.T) {
	const y = `packages:
  bat:
    version: __unversioned_latest__
    installMethods: [brew, apt]
    command:
      linux: batcat
    aliasBinary:
      batcat: bat
`
	cmds := cmdMap([]string{"apt-get"})
	in, m := newInstaller(t, y, "linux", cmds, Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		if strings.HasPrefix(joined, "dpkg -s") {
			return nil, errNotInstalled
		}
		if strings.Contains(joined, "apt-get install") {
			cmds["batcat"] = "/usr/bin/batcat"
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"bat"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "--no-install-recommends bat")
	require.Contains(t, calls, "ln -sf /usr/bin/batcat /home/u/.local/bin/bat")
}

func TestAliasBinarySkippedWhenSourceAbsent(t *testing.T) {
	const y = `packages:
  bat:
    version: __unversioned_latest__
    installMethods: [brew]
    aliasBinary:
      batcat: bat
`
	in, m := newInstaller(t, y, "darwin", cmdMap([]string{"brew", "bat"}), Options{})
	require.NoError(t, in.Install([]string{"bat"}))
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "ln -sf")
}
