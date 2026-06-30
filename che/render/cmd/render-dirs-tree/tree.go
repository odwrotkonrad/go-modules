// [>] 🤖🤖
package main

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	git "github.com/go-git/go-git/v5"
)

type treeNode map[string]treeNode

func trackedFiles(repoPath string) ([]string, error) {
	repo, err := git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("not a git repo: %s: %w", repoPath, err)
	}
	idx, err := repo.Storer.Index()
	if err != nil {
		return nil, fmt.Errorf("read git index at %s: %w", repoPath, err)
	}
	files := make([]string, len(idx.Entries))
	for i, e := range idx.Entries {
		files[i] = e.Name
	}
	return files, nil
}

func buildTree(paths []string) treeNode {
	root := treeNode{}
	for _, path := range paths {
		dir := filepath.Dir(path)
		if dir == "." {
			continue
		}
		node := root
		for part := range strings.SplitSeq(dir, string(filepath.Separator)) {
			child, ok := node[part]
			if !ok {
				child = treeNode{}
				node[part] = child
			}
			node = child
		}
	}
	return root
}

func renderTree(tree treeNode, depth int) string {
	var b strings.Builder
	for _, name := range slices.Sorted(maps.Keys(tree)) {
		fmt.Fprintf(&b, "%s%s\n", strings.Repeat("  ", depth), name)
		b.WriteString(renderTree(tree[name], depth+1))
	}
	return b.String()
}

func Generate(repoPath string) (string, error) {
	paths, err := trackedFiles(repoPath)
	if err != nil {
		return "", err
	}
	return renderTree(buildTree(paths), 0), nil
}

//[<] 🤖🤖
