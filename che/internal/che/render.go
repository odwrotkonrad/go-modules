package che

// [>] 🤖🤖

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

const repoFileMode = 0o660

type templateDest struct {
	path   string
	host   bool
	opts   render.Options
	header string
}

type templateItem struct {
	item  spec.FileItem
	dests []templateDest
}

// [why] a dest can @-include another dest this same pass renders, and che renders in spec order,
//
//	which says nothing about that dependency. AGENTS.md includes four generated files: two happen
//	to be listed earlier and resolve, two are listed later and do not. Every one is gitignored, so
//	a working copy carries them from an earlier run and only a clean checkout fails
func (p *ProfileReady) orderByIncludes(keep []templateItem) ([]templateItem, error) {
	producedBy := map[string]int{}
	for i, tpl := range keep {
		for _, d := range tpl.dests {
			producedBy[d.path] = i
		}
	}

	needs := make([][]int, len(keep))
	for i, tpl := range keep {
		if !slices.ContainsFunc(tpl.dests, func(d templateDest) bool { return d.opts.RenderReferencedFiles }) {
			continue
		}
		src, templatePath, err := p.readTemplateSrc(tpl.item)
		if err != nil {
			continue
		}
		body, err := render.ExecWithData(templatePath, src, p.templateAnchor(tpl.item), p.templateData(tpl.item))
		if err != nil {
			continue
		}
		for _, path := range render.IncludePaths(p.templateAnchor(tpl.item), body) {
			if j, ok := producedBy[path]; ok && j != i {
				needs[i] = append(needs[i], j)
			}
		}
	}

	const (
		unvisited = 0
		active    = 1
		done      = 2
	)
	state := make([]int, len(keep))
	out := make([]templateItem, 0, len(keep))
	var visit func(int, []string) error
	visit = func(i int, trail []string) error {
		switch state[i] {
		case done:
			return nil
		case active:
			return fmt.Errorf("render-templates: include cycle: %s", strings.Join(append(trail, keep[i].item.Rel), " -> "))
		}
		state[i] = active
		for _, j := range needs[i] {
			if err := visit(j, append(trail, keep[i].item.Rel)); err != nil {
				return err
			}
		}
		state[i] = done
		out = append(out, keep[i])
		return nil
	}
	for i := range keep {
		if err := visit(i, nil); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (p *ProfileReady) renderTemplates(templates []spec.FileItem, skips renderSkips) error {
	var keep []templateItem
	var hostDests []string
	for _, item := range templates {
		dests := p.resolveTemplateDests(item)
		if reason := p.skipReason(item, skips); reason != "" {
			for _, d := range dests {
				p.emit(log.Levels.Debug, "render-templates", p.wouldAction(d.path), d.path, reason)
			}
			continue
		}
		keep = append(keep, templateItem{item, dests})
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
	// [why] before both loops, so a dry run predicts the order the real render takes
	ordered, err := p.orderByIncludes(keep)
	if err != nil {
		return err
	}
	keep = ordered
	var errs []error
	if p.isDryRun() { // [why] dry-run predicts via the mock-render cache: no real render
		for _, tpl := range keep {
			settledMap := p.renderSettled(tpl.item)
			for _, d := range tpl.dests {
				settled := settledMap[d.path]
				switch {
				case settled && p.isDryRunAll():
					p.emit(log.Levels.Info, "render-templates", p.wouldAction(d.path), d.path, "same content")
				case settled: // [why] delta mode: an unchanged render logs nothing
				default:
					p.emitDryRun("render-templates", p.wouldAction(d.path), d.path)
				}
				if d.host {
					if err := p.fixPerms("render-templates", d.path, tpl.item); err != nil {
						errs = append(errs, p.failItem("render-templates", d.path, err))
					}
				}
			}
		}
		return errors.Join(errs...)
	}
	for _, tpl := range keep {
		if err := p.renderTemplate(tpl.item, tpl.dests); err != nil {
			errs = append(errs, p.failItem("render-templates", tpl.item.Rel, err))
		}
	}
	errs = append(errs, p.sweepStale("render", hostDests)) // [why] host dests only: repo-doc renders are git-tracked, never swept
	return errors.Join(errs...)
}

func (p *ProfileReady) templateSrcPath(item spec.FileItem) string {
	return filepath.Join(p.resolveRoot(), item.Rel)
}

func (p *ProfileReady) templateData(item spec.FileItem) render.Data {
	return render.Data{Env: p.env, Var: fsutil.MergeMap(fsutil.MergeMap(p.vars, item.Vars), p.refVars)}
}

func (p *ProfileReady) templateAnchor(item spec.FileItem) string {
	if p.isHostTemplate(item) {
		return p.resolveRepoRoot()
	}
	return p.resolveRoot()
}

func (p *ProfileReady) isHostTemplate(item spec.FileItem) bool {
	if len(item.Dests) == 0 {
		return true
	}
	for _, d := range item.Dests {
		if isGitRootDest(d.Path) {
			continue
		}
		if strings.HasPrefix(p.expandHome(d.Path), "/") {
			return true
		}
	}
	return false
}

func isGitRootDest(path string) bool {
	return strings.HasPrefix(path, "${invokingSpecGitRoot}/")
}

type renderSkips struct {
	Variables bool
}

func (p *ProfileReady) skipReason(item spec.FileItem, skips renderSkips) string {
	if !skips.Variables {
		return ""
	}
	body, ok := p.templateBody(item)
	if !ok {
		return ""
	}
	//[why] a template pulling another in with localFile hides that file's shell calls from
	//   a scan of its own body, so the skip must see the included bodies too
	bodies := append([][]byte{body}, render.LocalFileBodies(p.templateAnchor(item), body)...)
	if slices.ContainsFunc(bodies, render.IsShellCallPresent) {
		return "options.renderTemplates.skipVariables"
	}
	return ""
}

func (p *ProfileReady) templateBody(item spec.FileItem) ([]byte, bool) {
	if spec.IsRemoteSrc(item.Rel) {
		if p.isDryRun() {
			return nil, false
		}
		content, err := p.fetchRemote(item.Rel)
		if err != nil {
			return nil, false
		}
		return []byte(content), true
	}
	src, err := os.ReadFile(p.templateSrcPath(item))
	if err != nil {
		return nil, false
	}
	return src, true
}

func (p *ProfileReady) resolveTemplateDests(item spec.FileItem) []templateDest {
	if len(item.Dests) == 0 {
		return []templateDest{{path: p.toDest(spec.TrimTemplateExt(item.Rel)), host: true}}
	}
	if item.Derived {
		return []templateDest{{path: p.toDest(item.Dests[0].Path), host: true}}
	}
	out := make([]templateDest, len(item.Dests))
	for i, d := range item.Dests {
		if rest, ok := strings.CutPrefix(d.Path, "${invokingSpecGitRoot}/"); ok {
			out[i] = templateDest{path: filepath.Join(p.expandEnv("${invokingSpecGitRoot}"), rest), opts: d.Options, header: rest}
			continue
		}
		// [why] expand env / ~ before the host-vs-repo decision so $HOME/... and ~/... land on the host
		path := p.expandHome(d.Path)
		if strings.HasPrefix(path, "/") {
			out[i] = templateDest{path: path, host: true, opts: d.Options, header: path}
		} else {
			out[i] = templateDest{path: filepath.Join(p.templateAnchor(item), path), opts: d.Options, header: d.Path}
		}
	}
	return out
}

func (p *ProfileReady) readTemplateSrc(item spec.FileItem) ([]byte, string, error) {
	if spec.IsRemoteSrc(item.Rel) {
		content, err := p.fetchRemote(item.Rel)
		if err != nil {
			return nil, item.Rel, err
		}
		return []byte(content), item.Rel, nil
	}
	templatePath := p.templateSrcPath(item)
	src, err := os.ReadFile(templatePath)
	return src, templatePath, err
}

func (p *ProfileReady) renderTemplate(item spec.FileItem, dests []templateDest) error {
	src, templatePath, err := p.readTemplateSrc(item)
	if err != nil {
		return err
	}
	body, err := render.ExecWithData(templatePath, src, p.templateAnchor(item), p.templateData(item))
	if err != nil {
		return err
	}
	if len(item.Dests) == 0 || item.Derived {
		return p.placeFile(dests[0].path, render.StripMergeActions(body), item)
	}
	for _, d := range dests {
		out, err := p.composeDest(item, d, body)
		if err != nil {
			return err
		}
		p.warnUnresolvedIncludes(item, d, body)
		if d.host {
			if err := p.placeFile(d.path, out, item); err != nil {
				return err
			}
			continue
		}
		current, err := os.ReadFile(d.path)
		if err == nil && bytes.Equal(current, out) {
			p.emit(log.Levels.Debug, "render-templates", "overwrite", d.path, "same content")
			continue
		}
		p.emit(log.Levels.Info, "render-templates", resolvePastAction("create", err == nil), d.path)
		if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(repoFileMode)
		if parsed, ok := fsutil.ParseMode(item.Chmod); ok {
			mode = parsed
		} else if item.Chmod != "" {
			return fmt.Errorf("render %s: malformed chmod %q: want an octal string", d.path, item.Chmod)
		}
		if err := os.WriteFile(d.path, out, mode); err != nil {
			return err
		}
		//[why] WriteFile leaves an existing file's mode untouched: chmod so a re-render fixes perms too
		if err := os.Chmod(d.path, mode); err != nil {
			return err
		}
	}
	return nil
}

func (p *ProfileReady) warnUnresolvedIncludes(item spec.FileItem, d templateDest, body []byte) {
	if d.opts.RenderReferencedFiles || len(item.Dests) == 0 || item.Derived {
		return
	}
	for _, line := range render.UnresolvedIncludes(p.templateAnchor(item), body) {
		p.emit(log.Levels.Warn, "render-templates", "", d.path+": unresolved include "+line)
	}
}

func (p *ProfileReady) composeDest(item spec.FileItem, d templateDest, body []byte) ([]byte, error) {
	if len(item.Dests) == 0 || item.Derived {
		return render.StripMergeActions(body), nil
	}
	existing, _ := p.readExistingDest(d)
	return render.Compose(render.Composition{
		Body:       body,
		Opts:       d.opts,
		HeaderDest: d.header,
		TmplName:   item.Rel,
		Existing:   existing,
		RepoRoot:   p.templateAnchor(item),
	})
}

func (p *ProfileReady) readExistingDest(d templateDest) ([]byte, error) {
	if d.host {
		return p.Reader.ReadFileBytes(d.path)
	}
	return os.ReadFile(d.path)
}

func (p *ProfileReady) placeFile(dest string, body []byte, item spec.FileItem) error {
	if cur, err := p.Reader.ReadFileBytes(dest); err == nil && bytes.Equal(cur, body) {
		p.emit(log.Levels.Debug, "render-templates", "overwrite", dest, "same content")
		return p.fixPerms("render-templates", dest, item)
	}
	mode, _ := fsutil.ParseMode(item.Chmod)
	info := opInfo{kind: "render", srcRel: item.Rel, mode: item.Chmod, owner: formatOwnerSpec(item)}
	return p.mutate("render-templates", "create", dest, dest, info, func() error {
		return p.FS.InstallFile(dest, body, mode, formatOwnerSpec(item))
	})
}

// [<] 🤖🤖
