package e2e

// [>] 🤖🤖

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
)

// [why] installs real packages into a throwaway HOME, then runs each one:
//
//	the only coverage proving a method works end to end against live sources
//	opt in with E2E_INSTALL_METHOD=<method> (a group's method name, or "all")
type installGroup struct {
	Method          string            `yaml:"method"`
	OS              string            `yaml:"os"`
	RequiresCommand string            `yaml:"requiresCommand"`
	Packages        []string          `yaml:"packages"`
	Verify          map[string]string `yaml:"verify"`
}

func TestE2EInstallMethods(t *testing.T) {
	selected := os.Getenv("E2E_INSTALL_METHOD")
	if selected == "" {
		t.Skip("set E2E_INSTALL_METHOD=<method>|all to run real installs")
	}
	raw, err := os.ReadFile("install_methods.yml")
	require.NoError(t, err)
	var file struct {
		Groups []installGroup `yaml:"groups"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &file))

	ran, matched := 0, 0
	for _, g := range file.Groups {
		if selected != "all" && selected != g.Method && !strings.HasPrefix(g.Method, selected+"/") {
			continue
		}
		matched++
		if g.OS != "" && g.OS != runtime.GOOS {
			t.Logf("skip %s: %s only", g.Method, g.OS)
			continue
		}
		if g.RequiresCommand != "" {
			if _, err := exec.LookPath(g.RequiresCommand); err != nil {
				t.Logf("skip %s: %s absent", g.Method, g.RequiresCommand)
				continue
			}
		}
		ran++
		t.Run(g.Method, func(t *testing.T) { runInstallGroup(t, g) })
	}
	require.Positive(t, matched, "no group matched E2E_INSTALL_METHOD=%s", selected)
	// [why] a group gated off by os/requiresCommand is a valid environment outcome, not a failure:
	//   the runner simply cannot exercise that method
	if ran == 0 {
		t.Skipf("every %s group is gated off on this host", selected)
	}
}

func runInstallGroup(t *testing.T, g installGroup) {
	t.Helper()
	home := t.TempDir()
	bin := filepath.Join(home, ".local", "bin")
	pyenvRoot := filepath.Join(home, ".pyenv")
	// [why] go install writes to GOBIN, nvm/pyenv to their own roots: every destination a
	//   method may use has to be visible to the verify step
	goBin := filepath.Join(home, "go", "bin")
	path := strings.Join([]string{
		bin, goBin,
		filepath.Join(pyenvRoot, "bin"), filepath.Join(pyenvRoot, "shims"),
		os.Getenv("PATH"),
	}, ":")
	for _, d := range []string{bin, ".config", ".cache", ".local/state"} {
		require.NoError(t, os.MkdirAll(filepath.Join(home, d), 0o755))
	}
	env := []string{
		"HOME=" + home,
		"PATH=" + path,
		"PYENV_ROOT=" + pyenvRoot,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"XDG_DATA_HOME=" + filepath.Join(home, ".local", "share"),
		"XDG_BIN_HOME=" + bin,
		"XDG_STATE_HOME=" + filepath.Join(home, ".local", "state"),
		"XDG_CACHE_HOME=" + filepath.Join(home, ".cache"),
		"CHE_E2E=1",
		"CHE_LOG_LEVEL=debug",
		"CHE_PACKAGES_PREBUILT_BINARIES_ARCHIVE_CHECK_PRESENT_ON_PATH=0",
		"NVM_DIR=" + filepath.Join(home, ".config", "nvm"),
		"GOBIN=" + goBin,
		//[why] GOPATH defaults into the throwaway HOME: without this the module cache's
		//  read-only dirs make t.TempDir cleanup fail the passing test
		"GOFLAGS=-modcacherw",
	}
	if v := os.Getenv("SSL_CERT_FILE"); v != "" {
		env = append(env, "SSL_CERT_FILE="+v)
	}
	if dir := os.Getenv("E2E_GOCOVERDIR"); dir != "" {
		env = append(env, "GOCOVERDIR="+dir)
	}

	work := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(work, "che.yml"),
		[]byte("e2e:\n  options: {autoDiscover: true}\n"), 0o644))
	install := exec.Command(binPath(t), "packages", "install", "--packages-file", packages.BuiltinSentinel)
	install.Args = append(install.Args, g.Packages...)
	install.Env = env
	install.Dir = work
	out, err := install.CombinedOutput()
	t.Logf("install %s:\n%s", g.Method, out)
	require.NoError(t, err, "install %s", g.Method)
	// [why] a host copy already on PATH makes che skip: the group would then verify the system
	//   binary and prove nothing about the method under test
	for _, pkg := range g.Packages {
		require.NotContains(t, string(out), "will not install "+pkg+":",
			"%s: %s was already present on the runner, so the method never ran", g.Method, pkg)
	}

	for pkg, cmdline := range g.Verify {
		verify := exec.Command("/bin/bash", "-ec", cmdline)
		verify.Env = env
		verify.Dir = home
		vout, verr := verify.CombinedOutput()
		t.Logf("verify %s: %s\n%s", pkg, cmdline, vout)
		require.NoError(t, verr, "verify %s via %q", pkg, cmdline)
		require.NotEmpty(t, strings.TrimSpace(string(vout)), "verify %s produced no output", pkg)
	}
}

// [<] 🤖🤖
