package render

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
)

var mdComment = regexp.MustCompile(`(?s)<!--.*?-->\n?`)

var mdHeading = regexp.MustCompile(`(?m)^(#{1,5})( )`)

func RenderMarkdown(repoRoot, path string, opts ...string) (string, error) {
	content, err := os.ReadFile(resolveUnder(repoRoot, fsutil.ExpandHome(path, os.Getenv("HOME"))))
	if err != nil {
		return "", err
	}
	body := string(content)
	for _, opt := range opts {
		switch opt {
		case "remove-frontmatter":
			_, body = SplitFrontmatter(body)
		case "strip-comments":
			body = mdComment.ReplaceAllString(body, "")
		case "normalize-headings":
			body = demoteHeadings(body, 1)
		default:
			return "", fmt.Errorf("renderMarkdown: unknown opt %q", opt)
		}
	}
	return strings.TrimSpace(body), nil
}

func demoteHeadings(body string, levels int) string {
	for range levels {
		body = mdHeading.ReplaceAllString(body, "$1#$2")
	}
	return body
}

func SplitFrontmatter(content string) (front, body string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", content
	}
	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) < 3 {
		return "", content
	}
	return parts[1], parts[2]
}

func ReadFrontmatter(repoRoot, path string) (string, error) {
	front, _, err := readSplit(repoRoot, path)
	return front, err
}

func ReadBody(repoRoot, path string) (string, error) {
	_, body, err := readSplit(repoRoot, path)
	return body, err
}

func readSplit(repoRoot, path string) (front, body string, err error) {
	content, err := os.ReadFile(resolveUnder(repoRoot, path))
	if err != nil {
		return "", "", err
	}
	front, body = SplitFrontmatter(string(content))
	return front, body, nil
}

func resolveUnder(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

//[<] 🤖🤖
