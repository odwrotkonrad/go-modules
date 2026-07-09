package fsutil

// [>] 🤖🤖

import (
	"os"
	"os/user"
	"runtime"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
)

func NormalizeOS(goos string) string {
	if goos == "darwin" {
		return "macos"
	}
	return goos
}

// IsVirtualized: mac via kern.hv_vmm_present==1 (Apple VZ guest); linux via systemd-detect-virt / container markers.
func IsVirtualized() bool {
	switch runtime.GOOS {
	case "darwin":
		out, err := execx.Default.Output(execx.Cmd{Argv: []string{"sysctl", "-n", "kern.hv_vmm_present"}})
		return err == nil && strings.TrimSpace(string(out)) == "1"
	case "linux":
		if execx.Default.Exec(execx.Cmd{Argv: []string{"systemd-detect-virt", "-q"}}) == nil {
			return true
		}
		if _, err := os.Stat("/.dockerenv"); err == nil {
			return true
		}
		if b, err := os.ReadFile("/proc/1/cgroup"); err == nil {
			s := string(b)
			return strings.Contains(s, "docker") || strings.Contains(s, "containerd") || strings.Contains(s, "lxc")
		}
		return false
	default:
		return false
	}
}

// UserHome: named user's home from passwd.
func UserHome(name string) (string, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}

// [<] 🤖🤖
