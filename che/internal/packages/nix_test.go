package packages

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
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
	requireCalls(t, m, "nix --extra-experimental-features nix-command flakes profile install github:NixOS/nixpkgs/nixpkgs-unstable#jq")
}

func TestInstallNixFromRegistryUsesItsRef(t *testing.T) {
	const yml = `installerRegistries:
  nix:
    - name: stable
      url: github:NixOS/nixpkgs
      ref: nixos-26.05
packages:
  bat:
    installers:
      - nix: {fromRegistry: stable}
`
	in, m := newInstaller(t, yml, "linux", cmdMap([]string{"nix"}), Options{})
	m.Stub = failOn(nixListPrefix)
	require.NoError(t, in.Install([]string{"bat"}))
	requireCalls(t, m, "profile install github:NixOS/nixpkgs/nixos-26.05#bat")
}

func TestInstallNixUnknownRegistryErrors(t *testing.T) {
	const yml = `installerRegistries:
  nix:
    - name: stable
      url: github:NixOS/nixpkgs
packages:
  bat:
    installers:
      - nix: {fromRegistry: nowhere}
`
	in, m := newInstaller(t, yml, "linux", cmdMap([]string{"nix"}), Options{})
	m.Stub = failOn(nixListPrefix)
	require.ErrorContains(t, in.Install([]string{"bat"}), `unknown nix registry "nowhere"`)
	refuteCalls(t, m, "profile install")
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
	wantLines(t, out, "install jq 1.8.2 via nix (nixpkgs) (dry run)")
	refuteCalls(t, m, "profile install")
}

type nixEntryGot struct {
	Found   bool   `yaml:"found"`
	Version string `yaml:"version,omitempty"`
}

func TestNixProfileEntry(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/nix_profile_entry.test.spec.yml", func(t *testing.T, c testyml.Case[nixEntryGot]) (nixEntryGot, error) {
		found, ver := nixProfileEntry(c.Input.Args.String(t, 0), c.Input.Args.String(t, 1))
		return nixEntryGot{Found: found, Version: ver}, nil
	})
}

// [<] 🤖🤖
