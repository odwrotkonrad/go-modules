package che

// [>] 🤖🤖

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"

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

func (s Service) formatTarget() string { return s.Domain + "/" + s.Name }

// plistSource is one candidate template path under root/.
type plistSource struct {
	relativePath string // repo-relative under root/, with marker
	marker       string // ".ontoHost.cp" or ".ontoHost.tpl"
	system       bool   // LaunchDaemons -> system, LaunchAgents -> gui
	home         string // rel prefix to rewrite to $HOME ("" -> system-root dest)
}

// resolveServices maps each name to its live Service via the plist under root/.
// Unknown name -> error. All current services are KeepAlive (LongRunning).
func (p *ProfileReady) resolveServices(names []string) ([]Service, error) {
	guiDomain := fmt.Sprintf("gui/%d", os.Getuid())
	out := make([]Service, 0, len(names))
	for _, name := range names {
		src, ok := p.locatePlist(name)
		if !ok {
			return nil, fmt.Errorf("unknown service %q: no plist under root/", name)
		}
		rel := strings.TrimSuffix(src.relativePath, src.marker)
		svc := Service{
			Name:        name,
			Plist:       p.plistDest(rel, src.home),
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

// plistDest maps a plist source rel to its live dest. A home candidate rewrites
// its leading tree folder (e.g. HOME/) onto $HOME; otherwise system-root.
func (p *ProfileReady) plistDest(rel, home string) string {
	if home != "" {
		return p.toDest("$HOME/" + strings.TrimPrefix(rel, home))
	}
	return p.toDest(rel)
}

func (p *ProfileReady) locatePlist(name string) (plistSource, bool) {
	candidates := []plistSource{
		{"Library/LaunchDaemons/" + name + ".plist.ontoHost.cp", ".ontoHost.cp", true, ""},
		{"Library/LaunchAgents/" + name + ".plist.ontoHost.tpl", ".ontoHost.tpl", false, ""},
		{"_home/Library/LaunchAgents/" + name + ".plist.ontoHost.tpl", ".ontoHost.tpl", false, "_home/"},
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(p.resolveRoot(), c.relativePath)); err == nil {
			return c, true
		}
	}
	return plistSource{}, false
}

// bootout unloads each loaded service, then polls until it is gone.
func (p *ProfileReady) bootout(services []Service) error {
	for _, s := range services {
		if p.isDryRun() {
			p.logMsg("bootout", s.formatTarget())
			continue
		}
		if !fsutil.IsLoaded(s.Sudo, s.formatTarget()) {
			continue
		}
		p.logMsg("bootout", s.formatTarget())
		bctx, span := p.tel.Span(p.opContext(), "service-bootout", attribute.String("service", s.formatTarget()))
		c := fsutil.BuildLctl(s.Sudo, "bootout", s.formatTarget())
		c.Ctx = bctx
		_ = execx.Default.Exec(c) // async, ignore exit
		fsutil.WaitGone(s.Sudo, s.formatTarget())
		p.tel.CountUnit(bctx, "service", "bootout", p.command)
		span.End()
		p.logMsg("bootout(done)", s.formatTarget())
	}
	return nil
}

// bootin bootstraps each service fresh from its plist. Does NOT auto-bootout.
func (p *ProfileReady) bootin(services []Service) error {
	var errs []error
	for _, s := range services {
		if p.isDryRun() {
			p.logMsg("bootstrap", s.formatTarget())
			continue
		}
		p.logMsg("bootstrap", s.formatTarget())
		bctx, span := p.tel.Span(p.opContext(), "service-bootin", attribute.String("service", s.formatTarget()))
		c := fsutil.BuildLctl(s.Sudo, "bootstrap", s.Domain, s.Plist)
		c.Ctx = bctx
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		if err := execx.Default.Exec(c); err != nil {
			err = fmt.Errorf("bootstrap %s: %w", s.formatTarget(), err)
			span.RecordError(err)
			p.logMsg("bootstrap(fail)", err.Error())
			p.tel.CountUnit(bctx, "service", "bootin-fail", p.command)
			span.End()
			errs = append(errs, err)
			continue
		}
		p.tel.CountUnit(bctx, "service", "bootin", p.command)
		span.End()
		p.logMsg("bootstrap(done)", s.formatTarget())
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
				p.logMsg("ensure", s.formatTarget())
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
		if pid, ok := fsutil.ResolvePID(s.Sudo, s.formatTarget()); ok {
			p.tel.CountUnit(p.opContext(), "service", "ensure", p.command)
			p.logMsg("running", fmt.Sprintf("%s (pid %d)", s.formatTarget(), pid))
		} else {
			p.tel.CountUnit(p.opContext(), "service", "ensure-fail", p.command)
			p.logMsg("error", s.formatTarget()+" has no running process")
			missing++
		}
	}
	if missing > 0 {
		return fmt.Errorf("%d service(s) have no running process", missing)
	}
	return nil
}

// [<] 🤖🤖
