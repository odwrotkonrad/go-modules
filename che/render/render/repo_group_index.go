// [>] 🤖🤖
package render

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
)

// purposeRelPath: a repo's purpose file, relative to the repo root.
const purposeRelPath = "assets/docs-agents/purpose.md"

// noPurposePlaceholder: emitted when a repo has no purpose.md.
const noPurposePlaceholder = "_(no purpose.md)_"

// indexHeadingLevel: level of the top index's section headings (Repositories,
// Subgroups). Repo headings nest one below, inlined purposes below that, each
// nested child index two below its subgroup heading.
const indexHeadingLevel = 1

// indexIntroMD: the Repositories section intro snippet ({{.Section}} heading
// marker, {{.Label}} group name, {{.Tree}} directory structure).
//
//go:embed snippets/repo-group-index-intro.md
var indexIntroMD string

var indexIntroTpl = template.Must(template.New("repo-group-index-intro").Parse(indexIntroMD))

// heading renders an ATX heading marker of the given level.
func heading(level int) string { return strings.Repeat("#", level) }

// demoteHeadings demotes every ATX heading in body by levels (capped at 6 per pass).
func demoteHeadings(body string, levels int) string {
	for range levels {
		body = mdHeading.ReplaceAllString(body, "$1#$2")
	}
	return body
}

// groupNode: one subgroup dir's direct children. childRepos/childSubgroups are
// names (basenames) relative to this node's dir. Repos stop recursion (.git present).
type groupNode struct {
	childRepos     []string
	childSubgroups []string
}

// isRepo: dir holds a .git entry (leaf; recursion stops here).
func isRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// hasRepoBelow: dir contains ≥1 repo at any depth (so it is a subgroup).
func hasRepoBelow(dir string) bool {
	if isRepo(dir) {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(entries, func(e os.DirEntry) bool {
		return e.IsDir() && hasRepoBelow(filepath.Join(dir, e.Name()))
	})
}

// scanGroup classifies dir's direct children: repos (.git) vs child subgroups
// (dirs with a repo below). Non-subgroup dirs (no repo anywhere below) are dropped.
func scanGroup(dir string) (groupNode, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return groupNode{}, err
	}
	var node groupNode
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		child := filepath.Join(dir, e.Name())
		switch {
		case isRepo(child):
			node.childRepos = append(node.childRepos, e.Name())
		case hasRepoBelow(child):
			node.childSubgroups = append(node.childSubgroups, e.Name())
		}
	}
	slices.Sort(node.childRepos)
	slices.Sort(node.childSubgroups)
	return node, nil
}

// repoPurpose reads a repo's purpose.md body (frontmatter stripped, headings
// demoted so its `# Purpose` nests under the repo heading), or the placeholder
// when the file is missing/unreadable.
func repoPurpose(repoDir string, repoLevel int) string {
	content, err := os.ReadFile(filepath.Join(repoDir, purposeRelPath))
	if err != nil {
		return noPurposePlaceholder
	}
	_, body := SplitFrontmatter(string(content))
	return strings.TrimSpace(demoteHeadings(body, repoLevel))
}

// groupTree renders the recursive directory structure below a group dir:
// repos bare, subgroups suffixed "/" with children indented two spaces.
func groupTree(dir string) string {
	var b strings.Builder
	var walk func(dir, indent string)
	walk = func(dir, indent string) {
		node, err := scanGroup(dir)
		if err != nil {
			return
		}
		for _, name := range slices.Sorted(slices.Values(slices.Concat(node.childRepos, node.childSubgroups))) {
			if slices.Contains(node.childSubgroups, name) {
				fmt.Fprintf(&b, "%s%s/\n", indent, name)
				walk(filepath.Join(dir, name), indent+"  ")
				continue
			}
			fmt.Fprintf(&b, "%s%s\n", indent, name)
		}
	}
	walk(dir, "")
	return b.String()
}

// groupLabel: the group's display name, base of its absolute path.
func groupLabel(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return filepath.Base(dir)
	}
	return filepath.Base(abs)
}

// renderGroupBody emits a group's body: each repo as a `Repo: ./<rel-path>`
// heading + purpose, then each child subgroup as a `Subgroup: ./<rel-path>`
// heading + its own body, recursively (no intro, no tree: the top index's
// tree already covers the whole structure). Headings at level+1.
func renderGroupBody(dir string, node groupNode, level int, rel string) string {
	child := heading(level + 1)
	var b strings.Builder
	for _, name := range node.childRepos {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s Repo: ./%s\n\n", child, filepath.Join(rel, name))
		b.WriteString(repoPurpose(filepath.Join(dir, name), level+1))
		b.WriteByte('\n')
	}
	for _, name := range node.childSubgroups {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		childRel := filepath.Join(rel, name)
		fmt.Fprintf(&b, "%s Subgroup: ./%s\n\n", child, childRel)
		childDir := filepath.Join(dir, name)
		childNode, err := scanGroup(childDir)
		if err != nil {
			continue
		}
		b.WriteString(renderGroupBody(childDir, childNode, level+1, childRel))
	}
	return b.String()
}

// renderGroupIndex emits the markdown index for one subgroup dir at the given
// heading level: the Repositories intro (where you are, the directory
// structure tree), then the group body via renderGroupBody (each direct repo
// as a `Repo: ./<rel-path>` heading + inlined purpose, each child subgroup as
// a `Subgroup: ./<rel-path>` heading + its inlined body). rel is this group's
// path relative to the index root (pwd), "" at the top. Deterministic order.
func renderGroupIndex(dir string, node groupNode, level int, rel string) string {
	var b strings.Builder
	if len(node.childRepos) > 0 {
		_ = indexIntroTpl.Execute(&b, struct{ Section, Label, Tree string }{heading(level), groupLabel(dir), groupTree(dir)})
		b.WriteByte('\n')
	}
	b.WriteString(renderGroupBody(dir, node, level, rel))
	return b.String()
}

// RepoGroupIndexDir renders the index for a single subgroup dir (the CLI + template
// entry point): scan direct children, emit the index markdown.
func RepoGroupIndexDir(dir string) (string, error) {
	node, err := scanGroup(dir)
	if err != nil {
		return "", fmt.Errorf("scan group %s: %w", dir, err)
	}
	return renderGroupIndex(dir, node, indexHeadingLevel, ""), nil
}

// RepoGroupIndex walks workspaceRoot and returns each subgroup's rel-dir →
// rendered index markdown. A subgroup is any dir with ≥1 repo below; each
// index inlines its direct repos and every child subgroup's index recursively.
func RepoGroupIndex(workspaceRoot string) (map[string]string, error) {
	out := map[string]string{}
	var walk func(dir string) error
	walk = func(dir string) error {
		if isRepo(dir) {
			return nil
		}
		node, err := scanGroup(dir)
		if err != nil {
			return err
		}
		if len(node.childRepos) == 0 && len(node.childSubgroups) == 0 {
			return nil
		}
		rel, err := filepath.Rel(workspaceRoot, dir)
		if err != nil {
			return err
		}
		out[rel] = renderGroupIndex(dir, node, indexHeadingLevel, "")
		for _, name := range node.childSubgroups {
			if err := walk(filepath.Join(dir, name)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(workspaceRoot); err != nil {
		return nil, fmt.Errorf("walk workspace %s: %w", workspaceRoot, err)
	}
	return out, nil
}

//[<] 🤖🤖
