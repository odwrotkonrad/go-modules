package host

// [>] 🤖🤖

import (
	"os"
	"strings"

	"gitlab.com/konradodwrot/go/che/internal/spec"
	"gitlab.com/konradodwrot/go/render-files/render"
)

// RenderTemplates renders each *.host.tpl in the resolved set onto the host.
// Glob-form items (no explicit dest) render to the derived live path; rich items
// fan out across their dests, inlining @-includes per RenderReferencedFiles.
func (h Host) RenderTemplates(templates []spec.FileItem) error {
	skipSecret := os.Getenv("CHE_RENDER_TEMPLATES_DRY_RUN_SECRETS") != ""
	var keep []spec.FileItem
	var dests []string
	for _, item := range templates {
		if skipSecret && srcHasSecretRef(h.Src(item.Rel)) {
			for _, dest := range h.templateDests(item) {
				h.fs.Log("render(dry-run-render-secrets)", dest)
			}
			continue
		}
		keep = append(keep, item)
		dests = append(dests, h.templateDests(item)...)
	}
	if err := h.archiveBefore("render", dests); err != nil {
		return err
	}
	if h.DryRun() { // [why] dry-run logs dests only: no gomplate render, no @-include resolve
		for _, item := range keep {
			for _, dest := range h.templateDests(item) {
				h.fs.Log("render(create)", dest)
				if err := h.fixPerms("render", dest, item); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, item := range keep {
		if err := h.renderTemplate(item); err != nil {
			return err
		}
	}
	return nil
}

// srcHasSecretRef reports whether the template source at path carries an op://
// secret reference (a render-time vault fetch). Unreadable source -> false
// (render proceeds, errors there).
func srcHasSecretRef(path string) bool {
	src, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return render.HasSecretRef(src)
}

// templateDests returns an item's live dests: derived path (glob-form, no
// explicit dest) else its expanded dests.
func (h Host) templateDests(item spec.FileItem) []string {
	if len(item.Dests) == 0 {
		return []string{h.ToDest(strings.TrimSuffix(item.Rel, spec.TmplExt))}
	}
	out := make([]string, len(item.Dests))
	for i, d := range item.Dests {
		out[i] = h.expandHome(d.Path)
	}
	return out
}

func (h Host) renderTemplate(item spec.FileItem) error {
	tmplPath := h.Src(item.Rel)
	src, err := os.ReadFile(tmplPath)
	if err != nil {
		return err
	}
	body, err := render.Exec(tmplPath, src, h.RepoRoot)
	if err != nil {
		return err
	}
	if len(item.Dests) == 0 {
		rel := strings.TrimSuffix(item.Rel, spec.TmplExt)
		return h.placeFile(h.ToDest(rel), body, item)
	}
	for _, d := range item.Dests {
		dest := h.expandHome(d.Path)
		out := render.Compose(render.Composition{
			Body:       body,
			Opts:       d.Options,
			HeaderDest: dest,
			TmplName:   tmplPath,
			RepoRoot:   h.RepoRoot,
		})
		if err := h.placeFile(dest, out, item); err != nil {
			return err
		}
	}
	return nil
}

// placeFile installs body with spec perms (mode 0 -> install default, no chown).
func (h Host) placeFile(dest string, body []byte, item spec.FileItem) error {
	mode, _ := parseMode(item.Chmod)
	return h.fs.Install(dest, body, mode, ownerSpec(item))
}

// [<] 🤖🤖
