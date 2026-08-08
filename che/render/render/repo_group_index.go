package render

// [>] 🤖🤖

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
)

const purposeRelPath = "assets/docs-agents/purpose.md"

const noPurposePlaceholder = "_(no purpose.md)_"

const indexHeadingLevel = 1

//go:embed snippets/repo-group-index-intro.md
var indexIntroMD string

var indexIntroTpl = template.Must(template.New("repo-group-index-intro").Parse(indexIntroMD))

func heading(level int) string { return strings.Repeat("#", level) }

func demoteHeadings(body string, levels int) string {
	for range levels {
		body = mdHeading.ReplaceAllString(body, "$1#$2")
	}
	return body
}

func isRepoDir(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func isRepoWithin(dir string) bool {
	if isRepoDir(dir) {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(entries, func(e os.DirEntry) bool {
		return e.IsDir() && isRepoWithin(filepath.Join(dir, e.Name()))
	})
}

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
		case isRepoDir(child):
			node.childRepos = append(node.childRepos, e.Name())
		case isRepoWithin(child):
			node.childSubgroups = append(node.childSubgroups, e.Name())
		}
	}
	slices.Sort(node.childRepos)
	slices.Sort(node.childSubgroups)
	return node, nil
}

func repoPurpose(repoDir string, repoLevel int) string {
	_, body, err := readSplit(repoDir, purposeRelPath)
	if err != nil {
		return noPurposePlaceholder
	}
	return strings.TrimSpace(demoteHeadings(body, repoLevel))
}

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

func groupLabel(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return filepath.Base(dir)
	}
	return filepath.Base(abs)
}

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

func renderGroupIndex(dir string, node groupNode, level int, rel string) string {
	var b strings.Builder
	if len(node.childRepos) > 0 {
		_ = indexIntroTpl.Execute(&b, struct{ Section, Label, Tree string }{heading(level), groupLabel(dir), groupTree(dir)})
		b.WriteByte('\n')
	}
	b.WriteString(renderGroupBody(dir, node, level, rel))
	return b.String()
}

func RepoGroupIndexDir(dir string) (string, error) {
	node, err := scanGroup(dir)
	if err != nil {
		return "", fmt.Errorf("scan group %s: %w", dir, err)
	}
	return renderGroupIndex(dir, node, indexHeadingLevel, ""), nil
}

func RepoGroupIndex(workspaceRoot string) (map[string]string, error) {
	out := map[string]string{}
	var walk func(dir string) error
	walk = func(dir string) error {
		if isRepoDir(dir) {
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
