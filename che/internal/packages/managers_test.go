package packages

// [>] 🤖🤖

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/fetchx"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

var testFetch *fetchx.Mock

func newInstaller(t *testing.T, filesYaml, osname string, cmds map[string]string, opts Options) (*Installer, *execx.Mock) {
	t.Helper()
	var f File
	require.NoError(t, yaml.Unmarshal([]byte(filesYaml), &f))
	m := &execx.Mock{}
	execx.Swap(t, m)
	testFetch = &fetchx.Mock{Bodies: map[string][]byte{}}
	fetchx.Swap(t, testFetch)
	return &Installer{File: &f, FilePath: "packages.yml", Host: testHost(osname, "amd64", cmds), Opts: opts}, m
}

func failOn(prefixes ...string) func(argv []string) ([]byte, error) {
	return func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		for _, p := range prefixes {
			if strings.HasPrefix(joined, p) {
				return nil, fmt.Errorf("fail: %s", joined)
			}
		}
		return nil, nil
	}
}

func stubOutputs(pairs ...string) func(argv []string) ([]byte, error) {
	return func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		for i := 0; i+1 < len(pairs); i += 2 {
			if strings.HasPrefix(joined, pairs[i]) {
				return []byte(pairs[i+1]), nil
			}
		}
		return nil, nil
	}
}

func chainStubs(stubs ...func(argv []string) ([]byte, error)) func(argv []string) ([]byte, error) {
	return func(argv []string) ([]byte, error) {
		for _, s := range stubs {
			if out, err := s(argv); out != nil || err != nil {
				return out, err
			}
		}
		return nil, nil
	}
}

func tempHome(t *testing.T, in *Installer) string {
	t.Helper()
	home := t.TempDir()
	in.Host.Getenv = func(k string) string {
		if k == "HOME" {
			return home
		}
		return ""
	}
	return home
}

func shaHex(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

func withSha(yml string, body []byte) string { return strings.ReplaceAll(yml, "goodsha", shaHex(body)) }

func tarBytes(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, name := range names {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: 3}))
		_, err := tw.Write([]byte("bin"))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	return buf.Bytes()
}

func tarGzBody(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(tarBytes(t, names...))
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func tarXzBody(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	xw, err := xz.NewWriter(&buf)
	require.NoError(t, err)
	_, err = xw.Write(tarBytes(t, names...))
	require.NoError(t, err)
	require.NoError(t, xw.Close())
	return buf.Bytes()
}

func zipBody(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range names {
		f, err := zw.CreateHeader(&zip.FileHeader{Name: name})
		require.NoError(t, err)
		_, err = f.Write([]byte("bin"))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func requireCalls(t *testing.T, m *execx.Mock, fragments ...string) {
	t.Helper()
	joined := strings.Join(m.Calls(), "\n")
	for _, f := range fragments {
		require.Contains(t, joined, f)
	}
}

func refuteCalls(t *testing.T, m *execx.Mock, fragments ...string) {
	t.Helper()
	joined := strings.Join(m.Calls(), "\n")
	for _, f := range fragments {
		require.NotContains(t, joined, f)
	}
}

func TestInstallRoundsServeLaterPackages(t *testing.T) {
	cmds := cmdMap([]string{"apt-get"})
	in, m := newInstaller(t, "packages:\n  x: [{npm: {packageName: x}}]\n  npm: [{apt: {packageName: npm}}]", "linux", cmds, Options{})
	m.Stub = func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		if strings.HasPrefix(joined, "dpkg -s") || strings.HasPrefix(joined, "npm ls") {
			return nil, fmt.Errorf("not installed")
		}
		if joined == "sudo apt-get install --yes --no-install-recommends npm" {
			cmds["npm"] = "/usr/bin/npm"
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"x", "npm"}))
	require.Contains(t, m.Calls(), "sudo npm install --global x")
}

func TestInstallScriptPathRunsUserFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scripts", "install-x.sh"), []byte("#!/bin/bash\n"), 0o755))
	in, m := newInstaller(t, "packages:\n  x: [{script: {path: scripts/install-x.sh}}]", "darwin", cmdMap(nil), Options{})
	in.FilePath = filepath.Join(dir, "packages.yml")
	require.NoError(t, in.Install([]string{"x"}))
	require.Equal(t, []string{"/bin/sh -e " + filepath.Join(dir, "scripts", "install-x.sh")}, m.Calls())
}

func TestScriptEnv(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/script_env.test.spec.yml", func(t *testing.T, c testyml.Case[map[string]string]) (map[string]string, error) {
		in, _ := newInstaller(t, c.Input.Args.String(t, 0), c.Input.Args.String(t, 1), cmdMap(nil), Options{})
		in.Host.Arch = c.Input.Args.String(t, 2)
		pkg := c.Input.Args.String(t, 3)
		var s *ScriptSpec
		for _, it := range in.File.Packages[pkg].Items {
			if it.Script != nil {
				s = it.Script
			}
		}
		require.NotNil(t, s)
		got := map[string]string{}
		for _, kv := range in.makeScriptEnv(pkg, s) {
			k, v, _ := strings.Cut(kv, "=")
			if strings.HasPrefix(k, "CHE_PKG_") {
				got[k] = v
			}
		}
		return got, nil
	})
}

func TestInstallPythonPyenvPinFromBuiltin(t *testing.T) {
	f, err := LoadBuiltin()
	require.NoError(t, err)
	m := &execx.Mock{}
	execx.Swap(t, m)
	in := &Installer{File: f, FilePath: BuiltinPath, Host: testHost("darwin", "arm64", cmdMap([]string{"python3", "pyenv", "brew"})), Opts: Options{}}
	m.Stub = func(argv []string) ([]byte, error) {
		if strings.Join(argv, " ") == "pyenv versions --bare" {
			return []byte("3.9.6\n"), nil
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"python3"}))
	requireCalls(t, m, "pyenv install --skip-existing 3.14.6", "pyenv global 3.14.6")

	m.Stub = func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		if joined == "pyenv versions --bare" {
			return []byte("3.14.6\n3.13.14\n"), nil
		}
		if joined == "pyenv global" {
			return []byte("3.14.6\n"), nil
		}
		return nil, nil
	}
	before := len(m.Calls())
	require.NoError(t, in.Install([]string{"python3"}))
	require.NotContains(t, strings.Join(m.Calls()[before:], "\n"), "pyenv install --skip-existing")
}

const kubectlYaml = `packages:
  kubectl:
    installers:
      - binariesRemoteArchive:
          platformEligibility:
            - linux-amd64: sha256:goodsha
          version: 1.36.3
          url: https://example.com/v{version}/kubectl
    versionCommand: kubectl version --client
`

func TestVersionCommandDriftReinstalls(t *testing.T) {
	body := []byte("kubectl-bin")
	in, m := newInstaller(t, withSha(kubectlYaml, body), "linux", cmdMap([]string{"kubectl"}), Options{})
	tempHome(t, in)
	m.Stub = stubOutputs("kubectl ", "Client Version: v1.36.0\n")
	testFetch.Bodies["https://example.com/v1.36.3/kubectl"] = body
	require.NoError(t, in.Install([]string{"kubectl"}))
	require.NotEmpty(t, testFetch.Calls())
}

func codeListStub(installed string) func(argv []string) ([]byte, error) {
	return func(argv []string) ([]byte, error) {
		if argv[0] == "code" && argv[1] == "--list-extensions" {
			return []byte(installed), nil
		}
		return nil, nil
	}
}

// [<] 🤖🤖
