package che

// [>] 🤖🤖

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
)

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
	marker string // ".ontoHost.cp" or ".ontoHost.tpl"
	system bool   // LaunchDaemons -> system, LaunchAgents -> gui
}

// resolveServices maps each name to its live Service via the plist under root/.
// Unknown name -> error. All current services are KeepAlive (LongRunning).
func (p *ProfileReady) resolveServices(names []string) ([]Service, error) {
	guiDomain := fmt.Sprintf("gui/%d", os.Getuid())
	out := make([]Service, 0, len(names))
	for _, name := range names {
		src, ok := p.locate(name)
		if !ok {
			return nil, fmt.Errorf("unknown service %q: no plist under root/", name)
		}
		svc := Service{
			Name:        name,
			Plist:       p.toDest(strings.TrimSuffix(src.rel, src.marker)),
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

func (p *ProfileReady) locate(name string) (plistSource, bool) {
	cands := []plistSource{
		{"Library/LaunchDaemons/" + name + ".plist.ontoHost.cp", ".ontoHost.cp", true},
		{"Library/LaunchAgents/" + name + ".plist.ontoHost.tpl", ".ontoHost.tpl", false},
		{"HOME/Library/LaunchAgents/" + name + ".plist.ontoHost.tpl", ".ontoHost.tpl", false},
	}
	for _, c := range cands {
		if _, err := os.Stat(filepath.Join(p.root(), c.rel)); err == nil {
			return c, true
		}
	}
	return plistSource{}, false
}

// bootout unloads each loaded service, then polls until it is gone.
func (p *ProfileReady) bootout(services []Service) error {
	for _, s := range services {
		if p.isDryRun() {
			p.logMsg("bootout", s.target())
			continue
		}
		if !fsutil.IsLoaded(s.Sudo, s.target()) {
			continue
		}
		p.logMsg("bootout", s.target())
		_ = execx.Default.Exec(fsutil.Lctl(s.Sudo, "bootout", s.target())) // async, ignore exit
		fsutil.WaitGone(s.Sudo, s.target())
		p.logMsg("bootout(done)", s.target())
	}
	return nil
}

// bootin bootstraps each service fresh from its plist. Does NOT auto-bootout.
func (p *ProfileReady) bootin(services []Service) error {
	var errs []error
	for _, s := range services {
		if p.isDryRun() {
			p.logMsg("bootstrap", s.target())
			continue
		}
		p.logMsg("bootstrap", s.target())
		c := fsutil.Lctl(s.Sudo, "bootstrap", s.Domain, s.Plist)
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		if err := execx.Default.Exec(c); err != nil {
			err = fmt.Errorf("bootstrap %s: %w", s.target(), err)
			p.logMsg("bootstrap(fail)", err.Error())
			errs = append(errs, err)
			continue
		}
		p.logMsg("bootstrap(done)", s.target())
	}
	return errors.Join(errs...)
}

// ensure settles, then verifies each long-running service has a live pid.
// Errors if any is missing. No mutation.
func (p *ProfileReady) ensure(services []Service) error {
	if p.isDryRun() {
		p.logMsg("settle", fmt.Sprintf("%ds before pid check", fsutil.SettleSeconds))
		for _, s := range services {
			if s.LongRunning {
				p.logMsg("ensure", s.target())
			}
		}
		return nil
	}
	p.logMsg("settle", fmt.Sprintf("%ds before pid check", fsutil.SettleSeconds))
	fsutil.Sleep(fsutil.SettleSeconds * time.Second)
	missing := 0
	for _, s := range services {
		if !s.LongRunning {
			continue
		}
		if pid, ok := fsutil.PID(s.Sudo, s.target()); ok {
			p.logMsg("running", fmt.Sprintf("%s (pid %d)", s.target(), pid))
		} else {
			p.logMsg("error", s.target()+" has no running process")
			missing++
		}
	}
	if missing > 0 {
		return fmt.Errorf("%d service(s) have no running process", missing)
	}
	return nil
}

// [<] 🤖🤖
