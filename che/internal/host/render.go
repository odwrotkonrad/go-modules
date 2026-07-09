package host

// [>] 🤖🤖

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

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

// RenderTemplates renders each *.tpl in the resolved set. Sources are
// repo-root-relative. Glob-form items (no explicit dest) render raw to the
// derived host path; rich items fan out across their dests through
// render.Compose, host dests placed with spec perms, repo dests written as
// plain repo files. skipSecrets drops sources carrying op:// secret refs
// (logged, dests untouched). A failing item is logged and the rest still
// render; failures join into the returned error.
func (h Host) RenderTemplates(templates []spec.FileItem, skipSecrets bool) error {
	var keep []spec.FileItem
	var hostDests []string
	for _, item := range templates {
		if skipSecrets && isSecretRefInSrc(filepath.Join(h.RepoRoot, item.Rel)) {
			for _, d := range h.templateDests(item) {
				h.log("render(skip-secrets)", d.path)
			}
			continue
		}
		keep = append(keep, item)
		for _, d := range h.templateDests(item) {
			if d.host {
				hostDests = append(hostDests, d.path)
			}
		}
	}
	if len(hostDests) > 0 { // [why] repo-only renders leave no empty backup archives
		if err := h.archiveBefore("render", hostDests); err != nil {
			return err
		}
	}
	var errs []error
	if h.IsDryRun() { // [why] dry-run logs dests only: no gomplate render, no @-include resolve
		for _, item := range keep {
			for _, d := range h.templateDests(item) {
				h.log("render(create)", d.path)
				if d.host {
					if err := h.fixPerms("render", d.path, item); err != nil {
						errs = append(errs, h.failItem("render", d.path, err))
					}
				}
			}
		}
		return errors.Join(errs...)
	}
	for _, item := range keep {
		if err := h.renderTemplate(item); err != nil {
			errs = append(errs, h.failItem("render", item.Rel, err))
		}
	}
	return errors.Join(errs...)
}

// isSecretRefInSrc reports whether the template source at path carries an op://
// secret reference (a render-time vault fetch). Unreadable source -> false
// (render proceeds, errors there).
func isSecretRefInSrc(path string) bool {
	src, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return render.IsSecretRefPresent(src)
}

// templateDests resolves an item's dests: derived host path (no explicit dest,
// root/-prefixed source) else its expanded dests, kind by path shape.
func (h Host) templateDests(item spec.FileItem) []tmplDest {
	if len(item.Dests) == 0 {
		rel := strings.TrimPrefix(item.Rel, spec.RootPrefix)
		return []tmplDest{{path: h.ToDest(spec.TrimTmplExt(rel)), host: true}}
	}
	out := make([]tmplDest, len(item.Dests))
	for i, d := range item.Dests {
		if strings.HasPrefix(d.Path, "~/") || strings.HasPrefix(d.Path, "/") {
			p := h.expandHome(d.Path)
			out[i] = tmplDest{path: p, host: true, opts: d.Options, header: p}
		} else {
			out[i] = tmplDest{path: filepath.Join(h.RepoRoot, d.Path), opts: d.Options, header: d.Path}
		}
	}
	return out
}

func (h Host) renderTemplate(item spec.FileItem) error {
	tmplPath := filepath.Join(h.RepoRoot, item.Rel)
	src, err := os.ReadFile(tmplPath)
	if err != nil {
		return err
	}
	body, err := render.Exec(tmplPath, src, h.RepoRoot)
	if err != nil {
		return err
	}
	if len(item.Dests) == 0 { // derived host dest: raw body, no Compose header
		return h.placeFile(h.templateDests(item)[0].path, body, item)
	}
	for _, d := range h.templateDests(item) {
		existing, _ := h.readExisting(d) // absent -> nil (mergeUpsert: defaults only)
		out := render.Compose(render.Composition{
			Body:       body,
			Opts:       d.opts,
			HeaderDest: d.header,
			TmplName:   item.Rel,
			Existing:   existing,
			RepoRoot:   h.RepoRoot,
		})
		if d.host {
			if err := h.placeFile(d.path, out, item); err != nil {
				return err
			}
			continue
		}
		h.log("render(create)", d.path)
		if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(d.path, out, repoFileMode); err != nil {
			return err
		}
	}
	return nil
}

// readExisting reads a dest's current content for Compose: host dests through
// the reader (mockable), repo dests straight from disk.
func (h Host) readExisting(d tmplDest) ([]byte, error) {
	if d.host {
		return h.reader.ReadFile(d.path)
	}
	return os.ReadFile(d.path)
}

// placeFile installs body with spec perms (mode 0 -> install default, no chown).
func (h Host) placeFile(dest string, body []byte, item spec.FileItem) error {
	mode, _ := parseMode(item.Chmod)
	return h.mutate("render(create)", dest, func() error {
		return h.fs.Install(dest, body, mode, ownerSpec(item))
	})
}

// [<] 🤖🤖
