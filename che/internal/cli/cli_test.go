package cli

// [>] 🤖🤖

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/che"
	"gitlab.com/konradodwrot/go-modules/che/internal/fetchx"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func repoEnv(t *testing.T, pwd string) (*app, *cobra.Command, string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("non-root path only; build resolves home from $HOME")
	}
	dir := t.TempDir()
	testyml.CopyDir(t, td, pwd, dir)
	testutil.GitRepo(t, dir)
	home := filepath.Join(dir, "home")
	require.NoError(t, os.MkdirAll(home, 0o755))
	a := New()
	root := a.Root()
	a.flags.CheWorkingDirectory = dir
	t.Setenv("CHE_SKIP_RUN_IF", "1")
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local/state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return a, root, home
}

func PackagesMockHost() packages.Host {
	cmds := map[string]bool{"apt-get": true, "npm": true, "sha256sum": true, "jq": true, "curl": true}
	return packages.Host{
		OS: "linux", Arch: "amd64", Distro: "debian", Euid: 501,
		LookPath: func(name string) (string, error) {
			if cmds[name] {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New(name + " not found")
		},
		PathDirs: func() []string { return nil },
		Getenv: func(k string) string {
			if k == "HOME" {
				return os.Getenv("HOME")
			}
			return ""
		},
		ReadFile: func(path string) ([]byte, error) { return nil, errors.New("read " + path + ": not stubbed") },
	}
}

func mockPackagesFetch(t *testing.T, home string) {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "kubectx", Mode: 0o755, Size: 3}))
	_, err := tw.Write([]byte("bin"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	var body bytes.Buffer
	gz := gzip.NewWriter(&body)
	_, err = gz.Write(tarBuf.Bytes())
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	yml := filepath.Join(home, ".config", "packages", "packages.yml")
	if raw, err := os.ReadFile(yml); err == nil {
		raw = bytes.ReplaceAll(raw, []byte("mocksha"), fmt.Appendf(nil, "%x", sha256.Sum256(body.Bytes())))
		require.NoError(t, os.WriteFile(yml, raw, 0o644))
	}
	fetchx.Swap(t, &fetchx.Mock{Bodies: map[string][]byte{
		"https://example.com/v0.11.0/kubectx_v0.11.0_linux_x86_64.tar.gz": body.Bytes(),
	}})
}

func setupMock(t *testing.T, pwd, profile string, decl map[string]string, env map[string]string) (*app, *cobra.Command, string) {
	t.Helper()
	a, root, home := repoEnv(t, pwd)
	if _, ok := env["CHE_DRY_RUN"]; !ok {
		t.Setenv("CHE_DRY_RUN", "")
	}
	if profile != "" {
		a.flags.Profiles = strings.Split(profile, ",")
	}

	m := testutil.ApplyMocks(t, decl)
	if _, ok := decl["packages.Host"]; ok {
		testyml.Swap(t, &che.NewPackagesHost, PackagesMockHost)
		mockPackagesFetch(t, home)
	}
	realSeams := che.NewSeams
	testyml.Swap(t, &che.NewSeams, func(home string) che.Seams {
		s := realSeams(home)
		s.FS = m.FS
		s.Reader = &testutil.FileSystemMockReader{Roots: []string{a.flags.CheWorkingDirectory, home}}
		s.Ledger = nil // [why] record-only tests: no ledger side effects
		return s
	})

	require.NoError(t, a.init("all"))
	return a, root, home
}

type buildWant struct {
	Profiles  string `yaml:"profiles"`
	DryRunAll bool   `yaml:"dryRunAll"`
}

func TestInit(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/init.test.spec.yml",
		func(t *testing.T, c testyml.Case[buildWant]) {
			a, _, _ := repoEnv(t, c.Context.Pwd)
			t.Setenv("CHE_DRY_RUN", "")
			t.Setenv("CHE_VALIDATE_SPEC", "")
			for k, v := range c.Context.Env {
				t.Setenv(k, v)
			}
			if p := c.Input.Args.String(t, 0); p != "" {
				a.flags.Profiles = []string{p}
			}
			a.flags.ValidateSpec = options.ValidateSpecMode(c.Input.Args.String(t, 1))
			if extra := c.Input.Args.String(t, 2); extra != "" {
				f, err := os.OpenFile(filepath.Join(a.flags.CheWorkingDirectory, "che.yml"), os.O_APPEND|os.O_WRONLY, 0o644)
				require.NoError(t, err)
				_, err = f.WriteString("\n" + extra)
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}
			err := a.init("all")
			if c.Expected.Check(t, err) {
				return
			}
			var names []string
			for _, p := range a.root.AllProfiles() {
				names = append(names, p.Ref())
			}
			got := buildWant{Profiles: strings.Join(names, ","), DryRunAll: a.opts.DryRun == options.DryRun.All}
			assert.Equal(t, c.Expected.Output, got)
		})
}

// [<] 🤖🤖
