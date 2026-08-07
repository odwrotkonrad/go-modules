package packages

// [>] 🤖🤖

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const kubectxYaml = `packages:
  kubectx:
    - binary:
        version: 0.11.0
        url: https://example.com/v{version}/kubectx_v{version}_{os}_{arch_g}.tar.gz
        bin: kubectx
        sha256:
          linux-amd64: goodsha
`

const kindYaml = `packages:
  kind:
    - binary:
        version: 0.32.0
        url: https://example.com/v{version}/kind-{os}-{arch}
        sha256:
          linux-amd64: goodsha
`

func shaStub(sha string) func(argv []string) ([]byte, error) {
	return func(argv []string) ([]byte, error) {
		if argv[0] == "sha256sum" {
			return []byte(sha + "  " + argv[1] + "\n"), nil
		}
		return nil, nil
	}
}

func TestInstallBinaryTarFlow(t *testing.T) {
	in, m := newInstaller(t, kubectxYaml, "linux", cmdMap([]string{"sha256sum"}), Options{})
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"kubectx"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "curl -fsSL")
	require.Contains(t, calls, "kubectx_v0.11.0_linux_x86_64.tar.gz https://example.com/v0.11.0/kubectx_v0.11.0_linux_x86_64.tar.gz")
	require.Contains(t, calls, "rm -rf /home/u/.local/opt/kubectx")
	require.Contains(t, calls, "tar -x -C /home/u/.local/opt/kubectx")
	require.Contains(t, calls, "ln -sf /home/u/.local/opt/kubectx/kubectx /home/u/.local/bin/kubectx")
	require.NotContains(t, calls, "sudo")
	require.NotContains(t, calls, "install -m 0755")
}

func TestInstallBinaryShaMismatchAborts(t *testing.T) {
	in, m := newInstaller(t, kubectxYaml, "linux", cmdMap([]string{"sha256sum"}), Options{})
	m.Stub = shaStub("badsha")
	err := in.Install([]string{"kubectx"})
	require.ErrorContains(t, err, "sha256 mismatch")
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "tar -x")
}

func TestInstallBinaryBareAsset(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum"}), Options{})
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"kind"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "kind-linux-amd64 https://example.com/v0.32.0/kind-linux-amd64")
	require.Contains(t, calls, "install -m 0755")
	require.Contains(t, calls, "/home/u/.local/bin/kind")
	require.NotContains(t, calls, "tar -x")
}

const zigYaml = `packages:
  zig:
    - binary:
        version: 0.16.0
        url: https://example.com/{version}/zig-{arch_x}-linux-{version}.tar.xz
        bin: zig-{arch_x}-linux-{version}/zig
        sha256:
          linux-amd64: goodsha
`

func TestInstallBinaryTreeFlow(t *testing.T) {
	in, m := newInstaller(t, zigYaml, "linux", cmdMap([]string{"sha256sum"}), Options{})
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"zig"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "zig-x86_64-linux-0.16.0.tar.xz https://example.com/0.16.0/zig-x86_64-linux-0.16.0.tar.xz")
	require.Contains(t, calls, "mkdir -p /home/u/.local/opt/zig")
	require.Contains(t, calls, "tar -x -C /home/u/.local/opt/zig")
	require.Contains(t, calls, "ln -sf /home/u/.local/opt/zig/zig-x86_64-linux-0.16.0/zig /home/u/.local/bin/zig")
	require.NotContains(t, calls, "install -m 0755")
}

func TestInstallBinarySkipsWhenPinPresent(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum", "kind"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "kind" {
			return []byte("kind version 0.32.0\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"kind"}))
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "curl")
}

func TestInstallBinaryReinstallsOnPinDrift(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum", "kind"}), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "kind" {
			return []byte("kind version 0.30.0\n"), nil
		}
		return shaStub("goodsha")(argv)
	}
	require.NoError(t, in.Install([]string{"kind"}))
	require.Contains(t, strings.Join(m.Calls(), "\n"), "curl -fsSL")
}

const awsPkgYaml = `packages:
  aws:
    - pkg:
        url: https://awscli.amazonaws.com/AWSCLIV2.pkg
    - script:
        os: linux
        path: scripts/install-aws.sh
`

func TestInstallPkgDownloadsAndRunsInstaller(t *testing.T) {
	in, m := newInstaller(t, awsPkgYaml, "darwin", cmdMap(nil), Options{})
	require.NoError(t, in.Install([]string{"aws"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "curl -fsSL")
	require.Contains(t, calls, "AWSCLIV2.pkg https://awscli.amazonaws.com/AWSCLIV2.pkg")
	require.Contains(t, calls, "sudo installer -pkg")
	require.Contains(t, calls, "-target /")
}

func TestInstallPkgSkipsWhenPresent(t *testing.T) {
	in, m := newInstaller(t, awsPkgYaml, "darwin", cmdMap([]string{"aws"}), Options{})
	require.NoError(t, in.Install([]string{"aws"}))
	require.Empty(t, m.Calls())
}

func TestInstallPkgVerifiesShaWhenPinned(t *testing.T) {
	const yaml = `packages:
  aws:
    - pkg:
        url: https://awscli.amazonaws.com/AWSCLIV2.pkg
        sha256: {darwin-amd64: badsha}
`
	in, m := newInstaller(t, yaml, "darwin", cmdMap([]string{"sha256sum"}), Options{})
	m.Stub = shaStub("othersha")
	err := in.Install([]string{"aws"})
	require.ErrorContains(t, err, "sha256 mismatch")
	require.NotContains(t, strings.Join(m.Calls(), "\n"), "installer -pkg")
}

func TestInstallPkgFallsBackToLinuxScript(t *testing.T) {
	in, m := newInstaller(t, awsPkgYaml, "linux", cmdMap(nil), Options{})
	in.FilePath = BuiltinPath
	require.NoError(t, in.Install([]string{"aws"}))
	calls := m.Calls()
	require.Len(t, calls, 1)
	require.Contains(t, calls[0], "/bin/sh -ec")
	require.Contains(t, calls[0], "awscli-exe-linux")
}

func TestInstallPkgDryRunAnnounces(t *testing.T) {
	in, m := newInstaller(t, awsPkgYaml, "darwin", cmdMap(nil), Options{DryRun: true})
	out, err := captureStdout(t, func() error { return in.Install([]string{"aws"}) })
	require.NoError(t, err)
	wantLines(t, out, "install aws via pkg (dry run)")
	require.Empty(t, m.Calls())
}

func TestInstallBinaryCustomDestination(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum"}), Options{BinaryDestinationCandidates: []string{"~/bin"}})
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"kind"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "mkdir -p /home/u/bin")
	require.Contains(t, calls, "/home/u/bin/kind")
}

func TestInstallBinaryPicksFirstCandidateOnPath(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum"}),
		Options{BinaryDestinationCandidates: []string{"/custom/bin", "~/bin"}, BinaryCheckInPath: true})
	in.Host.PathDirs = func() []string { return []string{"/usr/bin", "/home/u/bin"} }
	m.Stub = shaStub("goodsha")
	out, err := captureStdout(t, func() error { return in.Install([]string{"kind"}) })
	require.NoError(t, err)
	require.Contains(t, strings.Join(m.Calls(), "\n"), "/home/u/bin/kind")
	notLine(t, out, "not on path")
}

func TestInstallBinaryWarnsWhenNoCandidateOnPath(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum"}),
		Options{BinaryDestinationCandidates: []string{"/custom/bin", "/other/bin"}, BinaryCheckInPath: true})
	in.Host.PathDirs = func() []string { return []string{"/usr/bin"} }
	m.Stub = shaStub("goodsha")
	out, err := captureStdout(t, func() error { return in.Install([]string{"kind"}) })
	require.NoError(t, err)
	wantLines(t, out, "no packages.binary.installDestinationCandidates entry is on PATH (/custom/bin, /other/bin), using /custom/bin")
	require.Contains(t, strings.Join(m.Calls(), "\n"), "/custom/bin/kind")
}

func TestInstallBinaryCheckInPathOffUsesFirstCandidate(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum"}),
		Options{BinaryDestinationCandidates: []string{"/custom/bin", "~/bin"}})
	in.Host.PathDirs = func() []string { return []string{"/home/u/bin"} }
	m.Stub = shaStub("goodsha")
	out, err := captureStdout(t, func() error { return in.Install([]string{"kind"}) })
	require.NoError(t, err)
	notLine(t, out, "not on path")
	require.Contains(t, strings.Join(m.Calls(), "\n"), "/custom/bin/kind")
}

// [<] 🤖🤖
