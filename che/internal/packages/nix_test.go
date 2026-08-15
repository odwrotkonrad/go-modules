package packages

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const nixListPrefix = "nix --extra-experimental-features nix-command flakes profile list"

const pinnedJqNixYaml = `packages:
  jq:
    installers:
      - nix:
          versionMap: {"1.8.2": "6b5e5b7a6631f065bf6908986990b37d845f847f"}
`

func TestInstallNixUnpinnedInstallsFromNixpkgs(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  jq: [nix]", "linux", cmdMap([]string{"nix"}), Options{})
	m.Stub = failOn(nixListPrefix)
	require.NoError(t, in.Install([]string{"jq"}))
	requireCalls(t, m, "nix --extra-experimental-features nix-command flakes profile install nixpkgs#jq")
}

func TestInstallNixPinnedInstallsRevisionRef(t *testing.T) {
	in, m := newInstaller(t, pinnedJqNixYaml, "linux", cmdMap([]string{"nix"}), Options{})
	m.Stub = failOn(nixListPrefix)
	require.NoError(t, in.Install([]string{"jq"}))
	requireCalls(t, m, "profile install github:NixOS/nixpkgs/6b5e5b7a6631f065bf6908986990b37d845f847f#jq")
}

func TestInstallNixReinstallsOnPinDrift(t *testing.T) {
	in, m := newInstaller(t, pinnedJqNixYaml, "linux", cmdMap([]string{"nix", "jq"}), Options{})
	m.Stub = stubOutputs(nixListPrefix, "Name: jq\nStore paths: /nix/store/abcd1234-jq-1.7.1-bin\n")
	require.NoError(t, in.Install([]string{"jq"}))
	requireCalls(t, m,
		"profile remove jq",
		"profile install github:NixOS/nixpkgs/6b5e5b7a6631f065bf6908986990b37d845f847f#jq")
}

func TestInstallNixSkipsWhenPinInstalled(t *testing.T) {
	in, m := newInstaller(t, pinnedJqNixYaml, "linux", cmdMap([]string{"nix", "jq"}), Options{})
	m.Stub = stubOutputs(nixListPrefix, "Name: jq\nStore paths: /nix/store/abcd1234-jq-1.8.2\n")
	require.NoError(t, in.Install([]string{"jq"}))
	refuteCalls(t, m, "profile install", "profile remove")
}

func TestInstallNixSkipsWhenInstalledUnpinned(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  jq: [nix]", "linux", cmdMap([]string{"nix", "jq"}), Options{})
	m.Stub = stubOutputs(nixListPrefix, "Name: jq\nStore paths: /nix/store/abcd1234-jq-1.8.2\n")
	require.NoError(t, in.Install([]string{"jq"}))
	refuteCalls(t, m, "profile install")
}

func TestInstallNixUpdatesUnpinned(t *testing.T) {
	in, m := newInstaller(t, "packages:\n  jq: [nix]", "linux", cmdMap([]string{"nix", "jq"}), Options{Update: true})
	m.Stub = stubOutputs(nixListPrefix, "Name: jq\nStore paths: /nix/store/abcd1234-jq-1.8.2\n")
	require.NoError(t, in.Install([]string{"jq"}))
	requireCalls(t, m, "profile upgrade jq")
	refuteCalls(t, m, "profile install")
}

func TestInstallNixRequestedVersionWithoutRevisionErrors(t *testing.T) {
	in, m := newInstaller(t, pinnedJqNixYaml, "linux", cmdMap([]string{"nix"}), Options{})
	m.Stub = failOn(nixListPrefix)
	err := in.InstallRequests([]Request{{Name: "jq", Versions: []string{"9.9"}}})
	require.ErrorContains(t, err, "versionMap")
	refuteCalls(t, m, "profile install")
}

func TestInstallNixEntryPinWithoutRevisionErrors(t *testing.T) {
	const yml = `packages:
  jq:
    version: 1.8.2
    installers: [nix]
`
	in, m := newInstaller(t, yml, "linux", cmdMap([]string{"nix"}), Options{})
	m.Stub = failOn(nixListPrefix)
	require.ErrorContains(t, in.Install([]string{"jq"}), "versionMap")
	refuteCalls(t, m, "profile install")
}

func TestInstallNixDryRunAnnounces(t *testing.T) {
	in, m := newInstaller(t, pinnedJqNixYaml, "linux", cmdMap([]string{"nix"}), Options{DryRun: true})
	m.Stub = failOn(nixListPrefix)
	out, err := captureStdout(t, func() error { return in.Install([]string{"jq"}) })
	require.NoError(t, err)
	wantLines(t, out, "install jq 1.8.2 via nix (dry run)")
	refuteCalls(t, m, "profile install")
}

func TestNixItemValidation(t *testing.T) {
	err := yaml.Unmarshal([]byte(`packages:
  x:
    installers:
      - nix:
          versionMap: {"1.0": "aaa", "2.0": "bbb"}
`), &File{})
	require.ErrorContains(t, err, "exactly one binary version")
	err = yaml.Unmarshal([]byte(`packages:
  x:
    installers:
      - nix: {fromRegistry: nixpkgs}
`), &File{})
	require.ErrorContains(t, err, "fromRegistry")
	err = yaml.Unmarshal([]byte(`packages:
  x:
    installers:
      - nix: {platformEligibility: [linux-amd64]}
`), &File{})
	require.ErrorContains(t, err, "platformEligibility")
	err = yaml.Unmarshal([]byte(`packages:
  x:
    installers:
      - nix: {packageName: "jq#1.8"}
`), &File{})
	require.ErrorContains(t, err, "bare nixpkgs attributes")
}

func TestNixProfileEntryParsesFormats(t *testing.T) {
	found, ver := nixProfileEntry("Name: jq\nStore paths: /nix/store/h-jq-1.8.2-bin\n", "jq")
	require.True(t, found)
	require.Equal(t, "1.8.2", ver)
	found, ver = nixProfileEntry("0 flake:nixpkgs#legacyPackages.x86_64-linux.jq github:NixOS/nixpkgs/aaa#legacyPackages.x86_64-linux.jq /nix/store/h-jq-1.7.1\n", "jq")
	require.True(t, found)
	require.Equal(t, "1.7.1", ver)
	found, ver = nixProfileEntry("Store paths: /nix/store/h-jq\n", "jq")
	require.True(t, found)
	require.Empty(t, ver)
	found, _ = nixProfileEntry("Store paths: /nix/store/h-bat-0.24.0\n", "jq")
	require.False(t, found)
}

// [<] 🤖🤖
