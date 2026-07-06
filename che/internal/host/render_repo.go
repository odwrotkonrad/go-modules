package host

// [>] 🤖🤖

import (
	"os"
	"path/filepath"

	"gitlab.com/konradodwrot/go/che/internal/spec"
	"gitlab.com/konradodwrot/go/render-files/render"
)

// repoFileMode: repo-rendered dests are plain repo files (git-tracked, group-writable).
const repoFileMode = 0o660

// RenderRepoTemplates renders each *.repo.tpl in the resolved set onto the repo.
// Sources live under RepoRoot; dests are repo-relative. Each dest's final bytes
// are shaped by render.Compose per its options. Mirrors RenderTemplates' dry-run
// + secret skip.
func (h Host) RenderRepoTemplates(items []spec.FileItem) error {
	skipSecret := os.Getenv("CHE_DRY_RUN_RENDER_SECRETS") != ""
	var keep []spec.FileItem
	for _, item := range items {
		if skipSecret && srcHasSecretRef(filepath.Join(h.RepoRoot, item.Rel)) {
			for _, dest := range h.repoDests(item) {
				h.fs.Log("render-repo(dry-run-render-secrets)", dest)
			}
			continue
		}
		keep = append(keep, item)
	}
	if h.DryRun() { // [why] dry-run logs dests only: no gomplate render, no @-include resolve
		for _, item := range keep {
			for _, dest := range h.repoDests(item) {
				h.fs.Log("render-repo(create)", dest)
			}
		}
		return nil
	}
	for _, item := range keep {
		if err := h.renderRepoTemplate(item); err != nil {
			return err
		}
	}
	return nil
}

// repoDests returns an item's absolute repo-relative dests.
func (h Host) repoDests(item spec.FileItem) []string {
	out := make([]string, len(item.Dests))
	for i, d := range item.Dests {
		out[i] = filepath.Join(h.RepoRoot, d.Path)
	}
	return out
}

func (h Host) renderRepoTemplate(item spec.FileItem) error {
	tmplPath := filepath.Join(h.RepoRoot, item.Rel)
	src, err := os.ReadFile(tmplPath)
	if err != nil {
		return err
	}
	body, err := render.Exec(tmplPath, src, h.RepoRoot)
	if err != nil {
		return err
	}
	for _, d := range item.Dests {
		dest := filepath.Join(h.RepoRoot, d.Path)
		existing, _ := os.ReadFile(dest) // absent -> nil (mergeUpsert: defaults only)
		out := render.Compose(render.Composition{
			Body:       body,
			Opts:       d.Options,
			HeaderDest: d.Path,
			TmplName:   item.Rel,
			Existing:   existing,
			RepoRoot:   h.RepoRoot,
		})
		h.fs.Log("render-repo(create)", dest)
		if err := os.WriteFile(dest, out, repoFileMode); err != nil {
			return err
		}
	}
	return nil
}

// [<] 🤖🤖
