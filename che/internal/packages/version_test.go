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
		{"kind version 0.32.0", "0.32.*", true},
		{"kind version 0.32.0", "0.31.*", false},
		{"Terraform v1.15.0", "1.*", true},
		{"Terraform v11.2.0", "1.*", false},
		{"go version go1.26.4 darwin/arm64", "1.26.*", true},
		{"Python 3.14.5", "3.14.*", true},
		{"Python 3.14.5", "3.1.*", false},
		{"prometheus, version 3.5.3 (branch: HEAD, revision: dcd3d551ced8)", "3.5.*", true},
		{"glab 1.108.0 (5de20850)", "1.107.*", false},
		{"jq-1.7.1-3", "1.7.1-3", true},
		{"1:1.7.1-3", "1.7.*", true},
		{"anything", "", true},
		{"no version here", "1.0.0", false},
	}
	for _, c := range cases {
		require.Equalf(t, c.want, PinMatches(c.out, c.pin), "out=%q pin=%q", c.out, c.pin)
	}
}

const pinnedBatYaml = `packages:
  bat:
    version: "0.25.*"
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

func TestEntryVersionWildcardOverridesArchivePin(t *testing.T) {
	const y = `packages:
  kind:
    version: "0.32.*"
    installMethods:
      - prebuiltArchive:
          version: 0.32.0
          url: https://example.com/kind
          sha256: {linux-amd64: goodsha}
`
	in, m := newInstaller(t, y, "linux", cmdMap([]string{"kind", "sha256sum"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "kind" {
			return []byte("kind version 0.32.9\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"kind"}))
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "curl")
}

const dynCiliumYaml = `packages:
  cilium:
    version: "0.19.*"
    installMethods:
      - prebuiltArchive:
          url: https://github.com/cilium/cilium-cli/releases/download/v{version}/cilium-{os}-{arch}.tar.gz
          bin: cilium
          platforms: [linux-amd64, linux-arm64]
`

func tagsStub(tags string) func(argv []string) ([]byte, error) {
	return func(argv []string) ([]byte, error) {
		if argv[0] == "git" && argv[1] == "ls-remote" {
			return []byte(tags), nil
		}
		if argv[0] == "sha256sum" {
			return []byte("x  " + argv[1] + "\n"), nil
		}
		return nil, nil
	}
}

func TestResolveArchiveVersionFromTags(t *testing.T) {
	in, m := newInstaller(t, dynCiliumYaml, "linux", cmdMap(nil), Options{})
	m.Stub = tagsStub("aaa\trefs/tags/v0.18.9\nbbb\trefs/tags/v0.19.5\nccc\trefs/tags/v0.19.12\nddd\trefs/tags/v0.19.12^{}\neee\trefs/tags/v0.20.0-rc.1\nfff\trefs/tags/v1.0.0\n")
	out, err := captureStdout(t, func() error { return in.Install([]string{"cilium"}) })
	require.NoError(t, err)
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "git ls-remote --tags https://github.com/cilium/cilium-cli")
	require.Contains(t, calls, "cilium-cli/releases/download/v0.19.12/cilium-linux-amd64.tar.gz")
	wantLines(t, out, "no sha256 declared for linux-amd64, skipping verification")
}

func TestResolveArchiveVersionNoMatchErrors(t *testing.T) {
	in, m := newInstaller(t, dynCiliumYaml, "linux", cmdMap(nil), Options{})
	m.Stub = tagsStub("aaa\trefs/tags/v1.0.0\n")
	err := in.Install([]string{"cilium"})
	require.ErrorContains(t, err, `no version matching "0.19.*"`)
}

func TestResolveArchiveVersionExplicitVersionsFrom(t *testing.T) {
	const y = `packages:
  kubectl:
    version: "1.36.*"
    installMethods:
      - prebuiltArchive:
          url: https://dl.k8s.io/release/v{version}/bin/{os}/{arch}/kubectl
          versionsFrom: https://github.com/kubernetes/kubernetes
          platforms: [linux-amd64]
`
	in, m := newInstaller(t, y, "linux", cmdMap(nil), Options{})
	m.Stub = tagsStub("aaa\trefs/tags/v1.36.4\nbbb\trefs/tags/v1.36.5-rc.0\n")
	require.NoError(t, in.Install([]string{"kubectl"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "ls-remote --tags https://github.com/kubernetes/kubernetes")
	require.Contains(t, calls, "dl.k8s.io/release/v1.36.4/bin/linux/amd64/kubectl")
}

func TestResolveArchiveVersionNoSourceErrors(t *testing.T) {
	const y = `packages:
  x:
    version: "1.*"
    installMethods:
      - prebuiltArchive:
          url: https://example.com/{version}/x.tar.gz
          platforms: [linux-amd64]
`
	in, _ := newInstaller(t, y, "linux", cmdMap(nil), Options{})
	require.ErrorContains(t, in.Install([]string{"x"}), "cannot resolve a concrete version")
}

func TestArchivePlatformsGateApplicability(t *testing.T) {
	in, m := newInstaller(t, dynCiliumYaml, "darwin", cmdMap(nil), Options{})
	out, err := captureStdout(t, func() error { return in.Install([]string{"cilium"}) })
	require.NoError(t, err)
	wantLines(t, out, "will not install cilium: no applicable manager")
	require.Empty(t, m.Calls())
}

// [<] 🤖🤖
