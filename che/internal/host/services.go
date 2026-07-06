package host

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gitlab.com/konradodwrot/go/che/internal/log"
)

// settleSeconds: wait before post-bootstrap pid check, services take time to spawn.
const settleSeconds = 15

// Service is one resolved launchd job.
type Service struct {
	Name        string // label == plist basename
	Plist       string // live dest plist path
	Domain      string // "system" or "gui/<uid>"
	Sudo        bool   // system domain only
	LongRunning bool   // KeepAlive, expect a live pid after bootstrap
}

func (s Service) target() string { return s.Domain + "/" + s.Name }

// plistSource is one candidate template path under root/.
type plistSource struct {
	rel    string // repo-relative under root/, with marker
	marker string // ".host.cp" or ".host.tpl"
	system bool   // LaunchDaemons -> system, LaunchAgents -> gui
}

// ResolveServices maps each name to its live Service via the plist under root/.
// Unknown name -> error. All current services are KeepAlive (LongRunning).
func (h Host) ResolveServices(names []string) ([]Service, error) {
	guiDomain := fmt.Sprintf("gui/%d", os.Getuid())
	out := make([]Service, 0, len(names))
	for _, name := range names {
		src, ok := h.locate(name)
		if !ok {
			return nil, fmt.Errorf("unknown service %q: no plist under root/", name)
		}
		svc := Service{
			Name:        name,
			Plist:       h.ToDest(strings.TrimSuffix(src.rel, src.marker)),
			Domain:      guiDomain,
			LongRunning: true,
		}
		if src.system {
			svc.Domain, svc.Sudo = "system", true
		}
		out = append(out, svc)
	}
	return out, nil
}

// locate finds name's plist in the three known dirs (first hit).
func (h Host) locate(name string) (plistSource, bool) {
	cands := []plistSource{
		{"Library/LaunchDaemons/" + name + ".plist.host.cp", ".host.cp", true},
		{"Library/LaunchAgents/" + name + ".plist.host.tpl", ".host.tpl", false},
		{"HOME/Library/LaunchAgents/" + name + ".plist.host.tpl", ".host.tpl", false},
	}
	for _, c := range cands {
		if _, err := os.Stat(filepath.Join(h.Root, c.rel)); err == nil {
			return c, true
		}
	}
	return plistSource{}, false
}

// lctl builds a launchctl argv, prefixing sudo iff needed and not already root.
// Explicit argv (avoids the zsh empty-runner word-split bug).
func (s Service) lctl(args ...string) *exec.Cmd {
	argv := append([]string{"launchctl"}, args...)
	if s.Sudo && os.Geteuid() != 0 {
		argv = append([]string{"sudo"}, argv...)
	}
	return exec.Command(argv[0], argv[1:]...)
}

// loaded reports whether the service is registered in its domain.
func (s Service) loaded() bool {
	return s.lctl("print", s.target()).Run() == nil
}

// Bootout unloads each loaded service, then polls until it is gone.
func (h Host) Bootout(services []Service) error {
	for _, s := range services {
		if h.DryRun() {
			log.Msg("bootout", s.target(), h.mode.log())
			continue
		}
		if !s.loaded() {
			continue
		}
		log.Msg("bootout", s.target(), log.Off)
		_ = s.lctl("bootout", s.target()).Run() // async, ignore exit
		s.waitGone()
	}
	return nil
}

// waitGone blocks until the service is gone (bootout is async).
func (s Service) waitGone() {
	for i := 0; s.loaded(); i++ {
		if i >= 50 {
			log.Msg("warn", s.target()+" still present after bootout", log.Off)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Bootin bootstraps each service fresh from its plist. Does NOT auto-bootout.
func (h Host) Bootin(services []Service) error {
	for _, s := range services {
		if h.DryRun() {
			log.Msg("bootstrap", s.target(), h.mode.log())
			continue
		}
		log.Msg("bootstrap", s.target(), log.Off)
		c := s.lctl("bootstrap", s.Domain, s.Plist)
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("bootstrap %s: %w", s.target(), err)
		}
	}
	return nil
}

// Ensure settles, then verifies each long-running service has a live pid.
// Errors if any is missing. No mutation.
func (h Host) Ensure(services []Service) error {
	if h.DryRun() {
		log.Msg("settle", fmt.Sprintf("%ds before pid check", settleSeconds), h.mode.log())
		for _, s := range services {
			if s.LongRunning {
				log.Msg("ensure", s.target(), h.mode.log())
			}
		}
		return nil
	}
	log.Msg("settle", fmt.Sprintf("%ds before pid check", settleSeconds), log.Off)
	time.Sleep(settleSeconds * time.Second)
	missing := 0
	for _, s := range services {
		if !s.LongRunning {
			continue
		}
		if pid, ok := s.pid(); ok {
			log.Msg("running", fmt.Sprintf("%s (pid %d)", s.target(), pid), log.Off)
		} else {
			log.Msg("error", s.target()+" has no running process", log.Off)
			missing++
		}
	}
	if missing > 0 {
		return fmt.Errorf("%d service(s) have no running process", missing)
	}
	return nil
}

var pidRe = regexp.MustCompile(`(?m)^\s*pid = ([0-9]+)`)

// pid reads the service's live pid from `launchctl print`, or (0,false) if none.
func (s Service) pid() (int, bool) {
	out, err := s.lctl("print", s.target()).Output()
	if err != nil {
		return 0, false
	}
	return ParsePID(string(out))
}

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
