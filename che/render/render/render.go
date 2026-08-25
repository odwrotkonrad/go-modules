package render

// [>] 🤖🤖

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"github.com/hairyhenderson/gomplate/v4"

	"gitlab.com/konradodwrot/go-modules/che/render/lib"
)

func Exec(name string, body []byte, repoRoot string) ([]byte, error) {
	out, err := ExecWithCtx(name, body, repoRoot, nil)
	if err != nil {
		return nil, err
	}
	return StripMergeActions(out), nil
}

func ExecWithCtx(name string, body []byte, repoRoot string, itemCtx map[string]string) ([]byte, error) {
	return execWithCtx(name, body, repoRoot, itemCtx, nil)
}

func ExecWithCtxMockSecrets(name string, body []byte, repoRoot string, itemCtx map[string]string) ([]byte, error) {
	return execWithCtx(name, body, repoRoot, itemCtx, func(ref string) (string, error) { return "mock:" + ref, nil })
}

func execWithCtx(name string, body []byte, repoRoot string, itemCtx map[string]string, secret func(string) (string, error)) ([]byte, error) {
	ctx := context.Background()
	if secret == nil {
		secret = newSecretFunc(ctx)
	}
	funcs := template.FuncMap{
		"secret":         withMergeAction(MergeActionAlwaysUpdate, secret),
		"shell":          withMergeAction(MergeActionAlwaysUpdate, newShellFunc(ctx, repoRoot)),
		"alwaysUpdate":   mergeActionFunc(MergeActionAlwaysUpdate),
		"keepIfExisting": mergeActionFunc(MergeActionKeepIfExisting),
		"renderDirsTree": func(rel ...string) (string, error) {
			if len(rel) > 0 {
				return DirsTree(filepath.Join(repoRoot, rel[0]))
			}
			return DirsTree(repoRoot)
		},
		"renderRepoGroupIndex": RepoGroupIndexDir,
		"renderMakefileDoc":    MakefileDoc,
		"frontmatter":          func(path string) (string, error) { return ReadFrontmatter(repoRoot, path) },
		"readBody":             func(path string) (string, error) { return ReadBody(repoRoot, path) },
		"renderMarkdown":       func(path string, opts ...string) (string, error) { return RenderMarkdown(repoRoot, path, opts...) },
		"remoteFile":           NewRemoteFetcher(),
		"localFile":            func(path string) (string, error) { return readLocalFile(repoRoot, path) },
	}
	opts := gomplate.RenderOptions{Funcs: funcs, MissingKey: "error"}
	if len(itemCtx) > 0 {
		ctxURL, cleanup, err := writeCtxFile(itemCtx)
		if err != nil {
			return nil, fmt.Errorf("render template %s: %w", name, err)
		}
		defer cleanup()
		opts.Context = map[string]gomplate.DataSource{".": {URL: ctxURL}}
	}
	r := gomplate.NewRenderer(opts)
	var buf bytes.Buffer
	if err := r.Render(ctx, filepath.Base(name), string(body), &buf); err != nil {
		return nil, fmt.Errorf("render template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

func MakefileDoc(path string) (string, error) {
	return lib.Generate(path)
}

var localFilePattern = regexp.MustCompile(`\{\{-?[^{}]*\blocalFile\s+"([^"]+)"`)

// LocalFileBodies returns the content of every file body pulls in with localFile, skipping unreadable ones.
func LocalFileBodies(repoRoot string, body []byte) [][]byte {
	bodies := [][]byte{}
	for _, m := range localFilePattern.FindAllSubmatch(body, -1) {
		included, err := readLocalFile(repoRoot, string(m[1]))
		if err != nil {
			continue
		}
		bodies = append(bodies, []byte(included))
	}
	return bodies
}

func readLocalFile(repoRoot, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(repoRoot, path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("localFile %q: %w", path, err)
	}
	return string(b), nil
}

func writeCtxFile(itemCtx map[string]string) (*url.URL, func(), error) {
	b, err := json.Marshal(itemCtx)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.CreateTemp("", "che-tpl-ctx-*.json")
	if err != nil {
		return nil, nil, err
	}
	f.Close()
	name := f.Name()
	cleanup := func() { os.Remove(name) }
	if err := os.WriteFile(name, b, 0o600); err != nil {
		cleanup()
		return nil, nil, err
	}
	return &url.URL{Scheme: "file", Path: name}, cleanup, nil
}

//[<] 🤖🤖
