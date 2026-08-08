package packages

// [>] 🤖🤖

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const kubectxYaml = `packages:
  kubectx:
    - binariesRemoteArchive:
        archScheme: odd
        extractBinaries: [kubectx]
        platformEligibility:
          - linux-amd64: sha256:goodsha
        version: 0.11.0
        url: https://example.com/v{version}/kubectx_v{version}_{os}_{arch}.tar.gz
`

const kindYaml = `packages:
  kind:
    - binariesRemoteArchive:
        platformEligibility:
          - linux-amd64: sha256:goodsha
        version: 0.32.0
        url: https://example.com/v{version}/kind-{os}-{arch}
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
    - binariesRemoteArchive:
        archScheme: uname
        extractBinaries: ["zig-{arch}-linux-{version}/zig"]
        platformEligibility:
          - linux-amd64: sha256:goodsha
        version: 0.16.0
        url: https://example.com/{version}/zig-{arch}-linux-{version}.tar.xz
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

const gcloudArchiveYaml = `archSchemes:
  vendor: {arm64: arm, amd64: x86_64}
packages:
  gcloud:
    - binariesRemoteArchive:
        archScheme: vendor
        extractBinaries: [sdk/bin/gcloud, sdk/bin/gsutil]
        platformEligibility:
          - darwin-arm64: sha256:goodsha
        version: 572.0.0
        url: https://example.com/cli-{version}-{os}-{arch}.tar.gz
`

func TestInstallBinariesRemoteArchiveArchScheme(t *testing.T) {
	in, m := newInstaller(t, gcloudArchiveYaml, "darwin", cmdMap([]string{"sha256sum"}), Options{})
	in.Host.Arch = "arm64"
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"gcloud"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "cli-572.0.0-darwin-arm.tar.gz https://example.com/cli-572.0.0-darwin-arm.tar.gz")
	require.Contains(t, calls, "ln -sf /home/u/.local/opt/gcloud/sdk/bin/gcloud /home/u/.local/bin/gcloud")
	require.Contains(t, calls, "ln -sf /home/u/.local/opt/gcloud/sdk/bin/gsutil /home/u/.local/bin/gsutil")
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

const awsYaml = `packages:
  aws:
    - script:
        os: darwin
        url: https://awscli.amazonaws.com/v2/install.sh
    - binariesRemoteArchive:
        archScheme: uname
        platformEligibility: [linux-amd64, linux-arm64]
        extractBinaries: [aws/dist/aws, aws/dist/aws_completer]
        url: https://awscli.amazonaws.com/awscli-exe-linux-{arch}.zip
`

func TestInstallAwsScriptOnDarwin(t *testing.T) {
	in, m := newInstaller(t, awsYaml, "darwin", cmdMap(nil), Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		if argv[0] == "curl" {
			return []byte("echo install-aws"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"aws"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "https://awscli.amazonaws.com/v2/install.sh")
	require.Contains(t, calls, "che-script-")
}

func TestInstallAwsSkipsWhenPresent(t *testing.T) {
	in, m := newInstaller(t, awsYaml, "darwin", cmdMap([]string{"aws"}), Options{})
	require.NoError(t, in.Install([]string{"aws"}))
	require.Empty(t, m.Calls())
}

func TestInstallAwsLinuxArchive(t *testing.T) {
	in, m := newInstaller(t, awsYaml, "linux", cmdMap(nil), Options{})
	require.NoError(t, in.Install([]string{"aws"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "awscli-exe-linux-x86_64.zip")
	require.Contains(t, calls, "unzip -oq")
	require.Contains(t, calls, "ln -sf /home/u/.local/opt/aws/aws/dist/aws /home/u/.local/bin/aws")
	require.Contains(t, calls, "ln -sf /home/u/.local/opt/aws/aws/dist/aws_completer /home/u/.local/bin/aws_completer")
	require.NotContains(t, calls, "install --update")
}

func TestInstallBinaryCustomDestination(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum"}), Options{BinariesRemoteArchiveDestinationCandidates: []string{"~/bin"}})
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"kind"}))
	calls := strings.Join(m.Calls(), "\n")
	require.Contains(t, calls, "mkdir -p /home/u/bin")
	require.Contains(t, calls, "/home/u/bin/kind")
}

func TestInstallBinaryPicksFirstCandidateOnPath(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum"}),
		Options{BinariesRemoteArchiveDestinationCandidates: []string{"/custom/bin", "~/bin"}, BinariesRemoteArchiveCheckPresentOnPath: true})
	in.Host.PathDirs = func() []string { return []string{"/usr/bin", "/home/u/bin"} }
	m.Stub = shaStub("goodsha")
	out, err := captureStdout(t, func() error { return in.Install([]string{"kind"}) })
	require.NoError(t, err)
	require.Contains(t, strings.Join(m.Calls(), "\n"), "/home/u/bin/kind")
	notLine(t, out, "not on path")
}

func TestInstallBinaryWarnsWhenNoCandidateOnPath(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum"}),
		Options{BinariesRemoteArchiveDestinationCandidates: []string{"/custom/bin", "/other/bin"}, BinariesRemoteArchiveCheckPresentOnPath: true})
	in.Host.PathDirs = func() []string { return []string{"/usr/bin"} }
	m.Stub = shaStub("goodsha")
	out, err := captureStdout(t, func() error { return in.Install([]string{"kind"}) })
	require.NoError(t, err)
	wantLines(t, out, "no packages.binariesRemoteArchive.installDestinationCandidates entry is on PATH (/custom/bin, /other/bin), using /custom/bin")
	require.Contains(t, strings.Join(m.Calls(), "\n"), "/custom/bin/kind")
}

func TestInstallBinariesRemoteArchiveCheckPresentOnPathOffUsesFirstCandidate(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum"}),
		Options{BinariesRemoteArchiveDestinationCandidates: []string{"/custom/bin", "~/bin"}})
	in.Host.PathDirs = func() []string { return []string{"/home/u/bin"} }
	m.Stub = shaStub("goodsha")
	out, err := captureStdout(t, func() error { return in.Install([]string{"kind"}) })
	require.NoError(t, err)
	notLine(t, out, "not on path")
	require.Contains(t, strings.Join(m.Calls(), "\n"), "/custom/bin/kind")
}

// [<] 🤖🤖
