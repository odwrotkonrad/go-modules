package packages

// [>] 🤖🤖

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestLoadBuiltinParsesShippedFile(t *testing.T) {
	f, err := LoadBuiltin()
	require.NoError(t, err)
	require.Contains(t, f.Packages, "bat")
	brew, err := f.Find("brew", BuiltinPath)
	require.NoError(t, err)
	require.Equal(t, "script", brew.Items[0].Mgr)
	require.Equal(t, "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh", brew.Items[0].Script.URL)
}

func TestLoadReadsFileAndErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "packages.yml")
	require.NoError(t, os.WriteFile(path, []byte("packages:\n  bat: [brew]\n"), 0o644))
	f, err := Load(path)
	require.NoError(t, err)
	require.Contains(t, f.Packages, "bat")

	_, err = Load(filepath.Join(dir, "absent.yml"))
	require.ErrorContains(t, err, "packages file not found")

	require.NoError(t, os.WriteFile(path, []byte(":not yaml ["), 0o644))
	_, err = Load(path)
	require.ErrorContains(t, err, "parse")
}

func TestMergeNilOverrideAndNilBase(t *testing.T) {
	f := &File{Packages: map[string]Entry{"bat": {Items: []Item{{Mgr: "brew"}}}}}
	f.Merge(nil)
	require.Len(t, f.Packages, 1)
	empty := &File{}
	empty.Merge(&File{Packages: map[string]Entry{"fd": {Items: []Item{{Mgr: "brew"}}}}})
	require.Contains(t, empty.Packages, "fd")
}

func TestNewHostCapturesRuntime(t *testing.T) {
	h := NewHost()
	require.NotEmpty(t, h.OS)
	require.NotEmpty(t, h.Arch)
	require.NotNil(t, h.LookPath)
	require.NotEmpty(t, h.PathDirs())
	require.Equal(t, os.Getenv("PATH"), h.Getenv("PATH"))
}

func TestManagerBinDirs(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/manager_bin_dir.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		in, m := newInstaller(t, "packages: {}", "linux", cmdMap(nil), Options{})
		m.Stub = stubOutputs("brew ", "/opt/homebrew\n", "npm ", "/usr/local\n")
		var env map[string]string
		c.Input.Args.To(t, 1, &env)
		in.Host.Getenv = func(k string) string { return env[k] }
		return in.managerBinDir(c.Input.Args.String(t, 0)), nil
	})
}

func TestInstalledVersion(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/installed_version.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		in, m := newInstaller(t, "packages: {}", "linux", cmdMap(nil), Options{})
		var stubs []stubPair
		c.Input.Args.To(t, 2, &stubs)
		makeInstallStub(m, installCfg{Stubs: stubs, FailOn: c.Input.Args.Strings(t, 3)})
		return in.installedVersion(c.Input.Args.String(t, 0), c.Input.Args.String(t, 1)), nil
	})
}

func TestInstallBinaryZipAsset(t *testing.T) {
	const zipYaml = `packages:
  terraform:
    - binariesRemoteArchive:
        platformEligibility:
          - linux-amd64: sha256:goodsha
        version: 1.15.0
        url: https://example.com/terraform_{version}_{os}_{arch}.zip
`
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	zf, err := zw.CreateHeader(&zip.FileHeader{Name: "terraform"})
	require.NoError(t, err)
	_, err = zf.Write([]byte("#!/bin/sh\n"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	in, _ := newInstaller(t, withSha(zipYaml, zbuf.Bytes()), "linux", cmdMap(nil), Options{})
	home := tempHome(t, in)
	testFetch.Bodies["https://example.com/terraform_1.15.0_linux_amd64.zip"] = zbuf.Bytes()
	require.NoError(t, in.Install([]string{"terraform"}))
	require.FileExists(t, filepath.Join(home, ".local", "opt", "terraform", "terraform"))
	requireSymlink(t, filepath.Join(home, ".local", "bin", "terraform"), filepath.Join(home, ".local", "opt", "terraform", "terraform"))
}

func TestDefaultEmittersWriteStdout(t *testing.T) {
	in, _ := newInstaller(t, "packages:\n  bat: [brew]", "linux", cmdMap(nil), Options{DryRun: true})
	out, err := captureStdout(t, func() error {
		in.emit(log.Levels.Info, "installed", "x via brew")
		in.emitSkip(log.Levels.Info, "x", "already installed via brew")
		in.emitDryRun("install", "x via brew")
		return nil
	})
	require.NoError(t, err)
	wantLines(t, out, "installed x via brew", "will not install x: already installed via brew", "install x via brew (dry run)")
}

// [<] 🤖🤖
