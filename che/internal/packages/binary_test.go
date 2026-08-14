package packages

// [>] 🤖🤖

import (
	"os"
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
	requireCalls(t, m,
		"curl -fsSL",
		"kubectx_v0.11.0_linux_x86_64.tar.gz https://example.com/v0.11.0/kubectx_v0.11.0_linux_x86_64.tar.gz",
		"rm -rf /home/u/.local/opt/kubectx",
		"tar -x -C /home/u/.local/opt/kubectx",
		"ln -sf /home/u/.local/opt/kubectx/kubectx /home/u/.local/bin/kubectx")
	refuteCalls(t, m, "sudo", "install -m 0755")
}

func TestInstallBinaryShaMismatchAborts(t *testing.T) {
	in, m := newInstaller(t, kubectxYaml, "linux", cmdMap([]string{"sha256sum"}), Options{})
	m.Stub = shaStub("badsha")
	err := in.Install([]string{"kubectx"})
	require.ErrorContains(t, err, "sha256 mismatch")
	refuteCalls(t, m, "tar -x")
}

func TestInstallBinaryBareAsset(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum"}), Options{})
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"kind"}))
	requireCalls(t, m,
		"kind-linux-amd64 https://example.com/v0.32.0/kind-linux-amd64",
		"install -m 0755",
		"/home/u/.local/bin/kind")
	refuteCalls(t, m, "tar -x")
}

func curlWrites(t *testing.T) func(argv []string) ([]byte, error) {
	return func(argv []string) ([]byte, error) {
		if argv[0] != "curl" {
			return nil, nil
		}
		for i, a := range argv {
			if a == "-o" {
				require.NoError(t, os.WriteFile(argv[i+1], []byte("asset"), 0o644))
			}
		}
		return nil, nil
	}
}

func TestInstallBinaryDownloadCacheReusesAsset(t *testing.T) {
	opts := Options{DownloadCacheDir: t.TempDir()}
	in, m := newInstaller(t, kubectxYaml, "linux", cmdMap([]string{"sha256sum"}), opts)
	m.Stub = chainStubs(curlWrites(t), shaStub("goodsha"))
	require.NoError(t, in.Install([]string{"kubectx"}))
	requireCalls(t, m, "curl -fsSL")
	in, m = newInstaller(t, kubectxYaml, "linux", cmdMap([]string{"sha256sum"}), opts)
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"kubectx"}))
	refuteCalls(t, m, "curl")
	requireCalls(t, m, "tar -x")
}

func TestInstallBinaryDownloadCacheEvictsOnShaMismatch(t *testing.T) {
	cache := t.TempDir()
	in, m := newInstaller(t, kubectxYaml, "linux", cmdMap([]string{"sha256sum"}), Options{DownloadCacheDir: cache})
	m.Stub = chainStubs(curlWrites(t), shaStub("badsha"))
	err := in.Install([]string{"kubectx"})
	require.ErrorContains(t, err, "sha256 mismatch")
	entries, err := os.ReadDir(cache)
	require.NoError(t, err)
	require.Empty(t, entries)
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
	requireCalls(t, m,
		"zig-x86_64-linux-0.16.0.tar.xz https://example.com/0.16.0/zig-x86_64-linux-0.16.0.tar.xz",
		"mkdir -p /home/u/.local/opt/zig",
		"tar -x -C /home/u/.local/opt/zig",
		"ln -sf /home/u/.local/opt/zig/zig-x86_64-linux-0.16.0/zig /home/u/.local/bin/zig")
	refuteCalls(t, m, "install -m 0755")
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
	requireCalls(t, m,
		"cli-572.0.0-darwin-arm.tar.gz https://example.com/cli-572.0.0-darwin-arm.tar.gz",
		"ln -sf /home/u/.local/opt/gcloud/sdk/bin/gcloud /home/u/.local/bin/gcloud",
		"ln -sf /home/u/.local/opt/gcloud/sdk/bin/gsutil /home/u/.local/bin/gsutil")
}

func TestInstallBinarySkipsWhenPinPresent(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum", "kind"}), Options{})
	m.Stub = stubOutputs("kind ", "kind version 0.32.0\n")
	require.NoError(t, in.Install([]string{"kind"}))
	refuteCalls(t, m, "curl")
}

func TestInstallBinaryReinstallsOnPinDrift(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum", "kind"}), Options{})
	m.Stub = chainStubs(stubOutputs("kind ", "kind version 0.30.0\n"), shaStub("goodsha"))
	require.NoError(t, in.Install([]string{"kind"}))
	requireCalls(t, m, "curl -fsSL")
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
	m.Stub = stubOutputs("curl ", "echo install-aws")
	require.NoError(t, in.Install([]string{"aws"}))
	requireCalls(t, m, "https://awscli.amazonaws.com/v2/install.sh", "che-script-")
}

func TestInstallAwsSkipsWhenPresent(t *testing.T) {
	in, m := newInstaller(t, awsYaml, "darwin", cmdMap([]string{"aws"}), Options{})
	require.NoError(t, in.Install([]string{"aws"}))
	require.Empty(t, m.Calls())
}

func TestInstallAwsLinuxArchive(t *testing.T) {
	in, m := newInstaller(t, awsYaml, "linux", cmdMap(nil), Options{})
	require.NoError(t, in.Install([]string{"aws"}))
	requireCalls(t, m,
		"awscli-exe-linux-x86_64.zip",
		"unzip -oq",
		"ln -sf /home/u/.local/opt/aws/aws/dist/aws /home/u/.local/bin/aws",
		"ln -sf /home/u/.local/opt/aws/aws/dist/aws_completer /home/u/.local/bin/aws_completer")
	refuteCalls(t, m, "install --update")
}

func TestInstallBinaryCustomDestination(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum"}), Options{BinariesRemoteArchiveDestinationCandidates: []string{"~/bin"}})
	m.Stub = shaStub("goodsha")
	require.NoError(t, in.Install([]string{"kind"}))
	requireCalls(t, m, "mkdir -p /home/u/bin", "/home/u/bin/kind")
}

func TestInstallBinaryPicksFirstCandidateOnPath(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum"}),
		Options{BinariesRemoteArchiveDestinationCandidates: []string{"/custom/bin", "~/bin"}, BinariesRemoteArchiveCheckPresentOnPath: true})
	in.Host.PathDirs = func() []string { return []string{"/usr/bin", "/home/u/bin"} }
	m.Stub = shaStub("goodsha")
	out, err := captureStdout(t, func() error { return in.Install([]string{"kind"}) })
	require.NoError(t, err)
	requireCalls(t, m, "/home/u/bin/kind")
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
	requireCalls(t, m, "/custom/bin/kind")
}

func TestInstallBinariesRemoteArchiveCheckPresentOnPathOffUsesFirstCandidate(t *testing.T) {
	in, m := newInstaller(t, kindYaml, "linux", cmdMap([]string{"sha256sum"}),
		Options{BinariesRemoteArchiveDestinationCandidates: []string{"/custom/bin", "~/bin"}})
	in.Host.PathDirs = func() []string { return []string{"/home/u/bin"} }
	m.Stub = shaStub("goodsha")
	out, err := captureStdout(t, func() error { return in.Install([]string{"kind"}) })
	require.NoError(t, err)
	notLine(t, out, "not on path")
	requireCalls(t, m, "/custom/bin/kind")
}

// [<] 🤖🤖
