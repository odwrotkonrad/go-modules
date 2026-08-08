package render

// [>] 🤖🤖

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	onepassword "github.com/1password/onepassword-sdk-go"
	"github.com/hairyhenderson/gomplate/v4"
	"github.com/invopop/jsonschema"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/render/lib"
)

var secretRetryDelays = []time.Duration{
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
}

var secretSchemes = []string{"op://", "gcp://"}

func Exec(name string, body []byte, repoRoot string) ([]byte, error) {
	return ExecWithCtx(name, body, repoRoot, nil)
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
		secret = secretFunc(ctx)
	}
	funcs := template.FuncMap{
		"secret": secret,
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

func isRateLimitErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "rate limit exceeded")
}

func retry[T any](delays []time.Duration, sleep func(time.Duration), shouldRetry func(error) bool, op func() (T, error)) (T, error) {
	v, err := op()
	for _, d := range delays {
		if !shouldRetry(err) {
			break
		}
		sleep(d)
		v, err = op()
	}
	return v, err
}

type secretResolver interface {
	Resolve(ctx context.Context, ref string) (string, error)
}

func (r opBackend) Resolve(ctx context.Context, ref string) (string, error) {
	return r.client.Secrets().Resolve(ctx, ref)
}

var newOpBackend = func(ctx context.Context) (secretResolver, error) {
	token := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("OP_SERVICE_ACCOUNT_TOKEN unset")
	}
	client, err := onepassword.NewClient(ctx,
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo("che", "1.0.0"),
	)
	if err != nil {
		return nil, err
	}
	return opBackend{client}, nil
}

func (r gcpBackend) Resolve(ctx context.Context, ref string) (string, error) {
	project, secret, version, err := parseGCPRef(ref)
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", project, secret, version)
	resp, err := r.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{Name: name})
	if err != nil {
		return "", err
	}
	return string(resp.GetPayload().GetData()), nil
}

func parseGCPRef(ref string) (project, secret, version string, err error) {
	rest := strings.TrimPrefix(ref, "gcp://")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("malformed gcp ref %q: want gcp://<project>/<secret>[/<version>]", ref)
	}
	version = "latest"
	if len(parts) >= 3 && parts[2] != "" {
		version = parts[2]
	}
	return parts[0], parts[1], version, nil
}

var newGCPBackend = func(ctx context.Context) (secretResolver, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return gcpBackend{client}, nil
}

var secretSleep = time.Sleep

func backendFactory(scheme string) func(context.Context) (secretResolver, error) {
	switch scheme {
	case "op://":
		return newOpBackend
	case "gcp://":
		return newGCPBackend
	}
	return nil
}

func secretFunc(ctx context.Context) func(string) (string, error) {
	cache := map[string]secretResolver{}
	return func(ref string) (string, error) {
		scheme := schemeOf(ref)
		factory := backendFactory(scheme)
		if factory == nil {
			return "", fmt.Errorf("secret %q: unknown scheme (want %s)", ref, strings.Join(secretSchemes, ", "))
		}
		backend := cache[scheme]
		if backend == nil {
			b, err := factory(ctx)
			if err != nil {
				return "", fmt.Errorf("secret %q: %w", ref, err)
			}
			backend = b
			cache[scheme] = b
		}
		secret, err := retry(secretRetryDelays, secretSleep, isRateLimitErr, func() (string, error) {
			return backend.Resolve(ctx, ref)
		})
		if err != nil {
			return "", fmt.Errorf("secret resolve %q: %w", ref, err)
		}
		return secret, nil
	}
}

func schemeOf(ref string) string {
	for _, s := range secretSchemes {
		if strings.HasPrefix(ref, s) {
			return s
		}
	}
	return ""
}

func (Options) JSONSchema() *jsonschema.Schema {
	s := &jsonschema.Schema{
		Description:          "per-dest render options",
		Type:                 "object",
		AdditionalProperties: jsonschema.FalseSchema,
		Properties:           jsonschema.NewProperties(),
	}
	s.Properties.Set("writeType", &jsonschema.Schema{
		Description: "how the rendered body lands: overwrite (default: header + body) | mergeUpsert (env KEY=VALUE union under the existing dest)",
		Enum:        []any{"", WriteTypeMergeUpsert},
	})
	s.Properties.Set("skipAutoGeneratedHeader", &jsonschema.Schema{
		Description: "drop the autogen header on the overwrite path (dests whose format forbids comments)",
		Type:        "boolean",
	})
	s.Properties.Set("renderReferencedFiles", &jsonschema.Schema{
		Description: "inline @-includes into the rendered body (overwrite path)",
		Type:        "boolean",
	})
	return s
}

func Compose(c Composition) []byte {
	if c.Opts.WriteType == WriteTypeMergeUpsert {
		return mergeUpsertEnv(c.Existing, c.Body)
	}
	var out bytes.Buffer
	if !c.Opts.SkipAutoGeneratedHeader {
		out.WriteString(autogenHeader(c.HeaderDest, c.TmplName))
		out.WriteByte('\n')
	}
	if c.Opts.RenderReferencedFiles {
		out.Write(resolveAtIncludes(c.RepoRoot, c.Body))
	} else {
		out.Write(c.Body)
	}
	return out.Bytes()
}

func mergeUpsertEnv(existing, rendered []byte) []byte {
	merged := map[string]string{}
	readInto := func(b []byte) {
		for line := range strings.Lines(string(b)) {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, val, _ := strings.Cut(line, "=")
			merged[key] = val
		}
	}
	readInto(rendered)
	readInto(existing)
	var out bytes.Buffer
	for _, k := range slices.Sorted(maps.Keys(merged)) {
		fmt.Fprintf(&out, "%s=%s\n", k, merged[k])
	}
	return out.Bytes()
}

func resolveAtIncludes(repoRoot string, body []byte) []byte {
	var out bytes.Buffer
	for line := range strings.Lines(string(body)) {
		line = strings.TrimSuffix(line, "\n")
		if isAtIncludeLine(line) {
			path := strings.TrimPrefix(line, "@")
			if rest, ok := strings.CutPrefix(path, "~/"); ok {
				path = "root/HOME/" + rest
			}
			if content, err := os.ReadFile(filepath.Join(repoRoot, path)); err == nil {
				out.Write(bytes.TrimRight(content, "\n"))
				out.WriteByte('\n')
				continue
			}
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.Bytes()
}

func IsSecretRefPresent(body []byte) bool {
	for _, s := range secretSchemes {
		if bytes.Contains(body, []byte(s)) {
			return true
		}
	}
	return false
}

func isAtIncludeLine(line string) bool {
	if !strings.HasPrefix(line, "@") || len(line) < 2 {
		return false
	}
	return !strings.ContainsAny(line, " \t")
}

func autogenHeader(out, tmplPath string) string {
	if strings.HasSuffix(out, ".md") {
		return fmt.Sprintf("<!-- autogenerated using %s -->", tmplPath)
	}
	return fmt.Sprintf("# autogenerated using %s", tmplPath)
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

func resolveUnder(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

func readSplit(repoRoot, path string) (front, body string, err error) {
	content, err := os.ReadFile(resolveUnder(repoRoot, path))
	if err != nil {
		return "", "", err
	}
	front, body = SplitFrontmatter(string(content))
	return front, body, nil
}

func ReadFrontmatter(repoRoot, path string) (string, error) {
	front, _, err := readSplit(repoRoot, path)
	return front, err
}

func ReadBody(repoRoot, path string) (string, error) {
	_, body, err := readSplit(repoRoot, path)
	return body, err
}

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

func DirsTree(repoRoot string) (string, error) {
	paths, err := fsutil.ListTrackedFiles(repoRoot)
	if err != nil {
		return "", err
	}
	return renderTree(buildTree(paths), 0), nil
}

func MakefileDoc(path string) (string, error) {
	return lib.Generate(path)
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
