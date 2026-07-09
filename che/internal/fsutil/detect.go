package fsutil

// [>] 🤖🤖

import (
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

// DetectReader is the fs read surface virtualization detection consults;
// tests swap in a mock so live host markers never leak into results.
var DetectReader FileSystemReader = OSReader{}

// UserLookup and GroupLookup resolve passwd/group records; tests swap in
// map-backed fakes so the live user db never leaks into results.
var (
	UserLookup  = user.Lookup
	GroupLookup = user.LookupGroup
)

// IsVirtualized: mac via kern.hv_vmm_present==1 (Apple VZ guest); linux via systemd-detect-virt / container markers.
func IsVirtualized() bool {
	return isVirtualized(runtime.GOOS, execx.Default, DetectReader)
}

func isVirtualized(goos string, exec execx.CmdExecutor, read FileSystemReader) bool {
	switch goos {
	case "darwin":
		out, err := exec.Output(execx.Cmd{Argv: []string{"sysctl", "-n", "kern.hv_vmm_present"}})
		return err == nil && strings.TrimSpace(string(out)) == "1"
	case "linux":
		if exec.Exec(execx.Cmd{Argv: []string{"systemd-detect-virt", "-q"}}) == nil {
			return true
		}
		if _, err := read.Stat("/.dockerenv"); err == nil {
			return true
		}
		if b, err := read.ReadFile("/proc/1/cgroup"); err == nil {
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
	u, err := UserLookup(name)
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}

// [<] 🤖🤖
