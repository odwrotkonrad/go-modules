package che

// [>] 🤖🤖

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

// repoFileMode: repo-rendered dests are plain repo files (git-tracked, group-writable).
const repoFileMode = 0o660

// tmplDest is one resolved template dest: live absolute path, host vs repo
// kind (dest path decides: ~/ or absolute -> host, relative -> repo), the
// per-dest options, and the header path Compose stamps.
type tmplDest struct {
	path   string
	host   bool
	opts   render.Options
	header string
}

// tmplItem pairs a resolved template item with its rendered dests.
type tmplItem struct {
	item  spec.FileItem
	dests []tmplDest
}

// renderTemplates renders each *.tpl in the resolved set. Glob-form items (no
// explicit dest) render raw to the derived host path, rich items fan out
// across their dests through render.Compose, host dests placed with spec
// perms, repo dests written as plain repo files. skipSecrets drops sources
// carrying op:// or gcp:// refs (logged, dests untouched).
func (p *ProfileReady) renderTemplates(templates []spec.FileItem, skipSecrets bool) error {
	var keep []tmplItem
	var hostDests []string
	for _, item := range templates {
		dests := p.resolveTemplateDests(item)
		if skipSecrets && p.isSecretRefInItem(item) {
			for _, d := range dests {
				log.Debug(skipTitle("render-templates", p.wouldAction(d.path), p.skipReasons("config.renderTemplates.skipSecrets")...), d.path, log.Off)
			}
			continue
		}
		keep = append(keep, tmplItem{item, dests})
		for _, d := range dests {
			if d.host {
				hostDests = append(hostDests, d.path)
			}
		}
	}
	if len(hostDests) > 0 { // [why] repo-only renders leave no empty backup archives
		if err := p.archiveBefore("render", hostDests); err != nil {
			return err
		}
	}
	var errs []error
	if p.isDryRun() { // [why] dry-run predicts via the mock-render cache: no real render, no secret resolve
		for _, t := range keep {
			hash, herr := p.mockRenderHash(t.item)
			for _, d := range t.dests {
				settled := herr == nil && p.readRenderHash(d.path) == hash
				switch {
				case settled && p.isDryRunAll():
					p.logMsg(skipTitle("render-templates", p.wouldAction(d.path), p.skipReasons("SameContent")...), d.path)
				case settled: // [why] delta mode: an unchanged render logs nothing
				default:
					p.logMsg(skipTitle("render-templates", p.wouldAction(d.path), p.dryRunReasons()...), d.path)
				}
				if d.host {
					if err := p.fixPerms("render-templates", d.path, t.item); err != nil {
						errs = append(errs, p.failItem("render-templates", d.path, err))
					}
				}
			}
		}
		return errors.Join(errs...)
	}
	for _, t := range keep {
		if err := p.renderTemplate(t.item, t.dests); err != nil {
			errs = append(errs, p.failItem("render-templates", t.item.Rel, err))
		}
	}
	errs = append(errs, p.sweepStale("render", hostDests)) // [why] host dests only: repo-doc renders are git-tracked, never swept
	return errors.Join(errs...)
}

// templateSrcPath resolves a local template source to its absolute path. Host
// templates (derived-dest glob form, or any host dest) anchor at
// workingDirectory (like makeCopies + services); repo-doc templates (repo dests
// only) anchor at the checkout.
func (p *ProfileReady) templateSrcPath(item spec.FileItem) string {
	if p.isHostTemplate(item) {
		return filepath.Join(p.resolveRoot(), item.Rel)
	}
	return filepath.Join(p.resolveRepoRoot(), item.Rel)
}

// isHostTemplate reports whether the item renders to a host dest: derived-dest
// (no explicit dest) is always host, else any explicit host dest (~/ or
// absolute) marks it host.
func (p *ProfileReady) isHostTemplate(item spec.FileItem) bool {
	if len(item.Dests) == 0 {
		return true
	}
	for _, d := range item.Dests {
		if strings.HasPrefix(p.expandHome(d.Path), "/") {
			return true
		}
	}
	return false
}

// isSecretRefInItem: the item's template source carries an op:// or gcp:// ref. Remote
// sources scan fetched content, except under dry-run ([why] dry-run stays
// offline). Unreadable source -> false (render proceeds, errors there).
func (p *ProfileReady) isSecretRefInItem(item spec.FileItem) bool {
	if spec.IsRemoteSrc(item.Rel) {
		if p.isDryRun() {
			return false
		}
		content, err := p.fetchRemote(spec.RemoteSrcRef(item.Rel))
		if err != nil {
			return false
		}
		return render.IsSecretRefPresent([]byte(content))
	}
	src, err := os.ReadFile(p.templateSrcPath(item))
	if err != nil {
		return false
	}
	return render.IsSecretRefPresent(src)
}

func (p *ProfileReady) resolveTemplateDests(item spec.FileItem) []tmplDest {
	if len(item.Dests) == 0 {
		return []tmplDest{{path: p.toDest(spec.TrimTmplExt(item.Rel)), host: true}}
	}
	if item.Derived { // glob dest rewrite: derived host dest, raw body like the bare-glob form
		return []tmplDest{{path: p.toDest(item.Dests[0].Path), host: true}}
	}
	out := make([]tmplDest, len(item.Dests))
	for i, d := range item.Dests {
		// [why] expand env / ~ before the host-vs-repo decision so $HOME/... and
		// $VAR/... dests resolve to their absolute host path, not a repo-relative one.
		path := p.expandHome(d.Path)
		if strings.HasPrefix(path, "/") {
			out[i] = tmplDest{path: path, host: true, opts: d.Options, header: path}
		} else {
			out[i] = tmplDest{path: filepath.Join(p.resolveRepoRoot(), path), opts: d.Options, header: d.Path}
		}
	}
	return out
}

// readTemplateSrc reads the item's template source (remote fetched, local from
// disk), returning the bytes and the template path (error messages, engine name).
func (p *ProfileReady) readTemplateSrc(item spec.FileItem) ([]byte, string, error) {
	if spec.IsRemoteSrc(item.Rel) {
		content, err := p.fetchRemote(spec.RemoteSrcRef(item.Rel))
		if err != nil {
			return nil, item.Rel, err
		}
		return []byte(content), item.Rel, nil
	}
	tmplPath := p.templateSrcPath(item)
	src, err := os.ReadFile(tmplPath)
	return src, tmplPath, err
}

func (p *ProfileReady) renderTemplate(item spec.FileItem, dests []tmplDest) error {
	src, tmplPath, err := p.readTemplateSrc(item)
	if err != nil {
		return err
	}
	body, err := render.ExecWithCtx(tmplPath, src, p.resolveRepoRoot(), item.Ctx)
	if err != nil {
		return err
	}
	p.storeRenderHashes(item, dests, tmplPath, src, body)
	if len(item.Dests) == 0 || item.Derived { // derived host dest: raw body, no Compose header
		return p.placeFile(dests[0].path, body, item)
	}
	for _, d := range dests {
		existing, _ := p.readExistingDest(d) // absent -> nil (mergeUpsert: defaults only)
		out := render.Compose(render.Composition{
			Body:       body,
			Opts:       d.opts,
			HeaderDest: d.header,
			TmplName:   item.Rel,
			Existing:   existing,
			RepoRoot:   p.resolveRepoRoot(),
		})
		if d.host {
			if err := p.placeFile(d.path, out, item); err != nil {
				return err
			}
			continue
		}
		current, err := os.ReadFile(d.path)
		if err == nil && bytes.Equal(current, out) {
			log.Debug(skipTitle("render-templates", "overwrite", "SameContent"), d.path, log.Off)
			continue
		}
		p.logMsg(resolveCreateTitle("render-templates(create)", err == nil), d.path)
		if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(d.path, out, repoFileMode); err != nil {
			return err
		}
	}
	return nil
}

// readExistingDest reads a dest's current content for Compose: host dests
// through the reader (mockable), repo dests straight from disk.
func (p *ProfileReady) readExistingDest(d tmplDest) ([]byte, error) {
	if d.host {
		return p.Reader.ReadFileBytes(d.path)
	}
	return os.ReadFile(d.path)
}

// placeFile installs body with spec perms (mode 0 -> install default, no chown).
// An unchanged dest (byte-identical content) skips the write: a debug
// (overwrite, skippedDue[SameContent]) line, perms drift still corrected.
func (p *ProfileReady) placeFile(dest string, body []byte, item spec.FileItem) error {
	if cur, err := p.Reader.ReadFileBytes(dest); err == nil && bytes.Equal(cur, body) {
		log.Debug(skipTitle("render-templates", "overwrite", "SameContent"), dest, log.Off)
		return p.fixPerms("render-templates", dest, item)
	}
	mode, _ := fsutil.ParseMode(item.Chmod)
	info := opInfo{kind: "render", srcRel: item.Rel, mode: item.Chmod, owner: formatOwnerSpec(item)}
	return p.mutate("render-templates(create)", dest, dest, info, func() error {
		return p.FS.InstallFile(dest, body, mode, formatOwnerSpec(item))
	})
}

// [<] 🤖🤖
