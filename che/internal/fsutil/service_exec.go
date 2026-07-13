package fsutil

// [>] 🤖🤖

import (
	"os"
	"regexp"
	"strconv"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

// SettleSeconds: wait before post-bootstrap pid check, services take time to spawn.
const SettleSeconds = 15

// Sleep paces the post-bootstrap settle and the bootout-gone poll, tests stub
// it to a no-op.
var Sleep = time.Sleep

// Lctl builds a launchctl command, prefixing sudo iff sudo is set and not
// already root. Explicit argv (avoids the zsh empty-runner word-split bug).
func Lctl(sudo bool, args ...string) execx.Cmd {
	argv := append([]string{"launchctl"}, args...)
	if sudo && os.Geteuid() != 0 {
		argv = append([]string{"sudo"}, argv...)
	}
	return execx.Cmd{Argv: argv}
}

// IsLoaded reports whether target is registered in its launchd domain.
func IsLoaded(sudo bool, target string) bool {
	return execx.Default.Exec(Lctl(sudo, "print", target)) == nil
}

// WaitGone blocks until target is gone (bootout is async).
func WaitGone(sudo bool, target string) {
	for i := 0; IsLoaded(sudo, target); i++ {
		if i >= 50 {
			log.Msg("warn", target+" still present after bootout", log.Off)
			return
		}
		Sleep(100 * time.Millisecond)
	}
}

// PID reads target's live pid from `launchctl print`, or (0,false) if none.
func PID(sudo bool, target string) (int, bool) {
	out, err := execx.Default.Output(Lctl(sudo, "print", target))
	if err != nil {
		return 0, false
	}
	return ParsePID(string(out))
}

var pidRe = regexp.MustCompile(`(?m)^\s*pid = ([0-9]+)`)

// ParsePID extracts a pid from `launchctl print` output (exposed for tests).
func ParsePID(printOut string) (int, bool) {
	m := pidRe.FindStringSubmatch(printOut)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// [<] 🤖🤖
