package render

// [>] 🤖🤖

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
)

func DirsTree(repoRoot string) (string, error) {
	paths, err := fsutil.ListTrackedFiles(repoRoot)
	if err != nil {
		return "", err
	}
	return renderTree(buildTree(paths), 0), nil
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

//[<] 🤖🤖
