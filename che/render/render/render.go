// Package render is the shared gomplate engine: template funcs, op:// (1Password) and gcp:// (GCP Secret Manager) secret resolution, remoteFile inclusion, frontmatter, markdown transforms, doc generators.
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

// secretRetryDelays: backoff between secret-resolve attempts that hit a
// backend rate limit.
var secretRetryDelays = []time.Duration{
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
}

// secretSchemes: the URI schemes the secret func dispatches on, single source
// of truth for the dispatcher and IsSecretRefPresent.
var secretSchemes = []string{"op://", "gcp://"}

// Exec renders body via the gomplate library, built-ins plus the funcs below.
// name: error messages only.
func Exec(name string, body []byte, repoRoot string) ([]byte, error) {
	return ExecWithCtx(name, body, repoRoot, nil)
}

// ExecWithCtx is Exec with an optional template context: a non-empty itemCtx
// becomes the template's root context (`.key`), fed to gomplate as a temp
// JSON context datasource aliased `.`.
func ExecWithCtx(name string, body []byte, repoRoot string, itemCtx map[string]string) ([]byte, error) {
	ctx := context.Background()
	funcs := template.FuncMap{
		"secret":               secretFunc(ctx),
		"renderDirsTree":       func() (string, error) { return DirsTree(repoRoot) },
		"renderRepoGroupIndex": RepoGroupIndexDir,
		"renderMakefileDoc":    MakefileDoc,
		"frontmatter":          func(path string) (string, error) { return ReadFrontmatter(repoRoot, path) },
		"readBody":             func(path string) (string, error) { return ReadBody(repoRoot, path) },
		"renderMarkdown":       func(path string, opts ...string) (string, error) { return RenderMarkdown(repoRoot, path, opts...) },
		"remoteFile":           NewRemoteFetcher(),
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

// writeCtxFile: gomplate context datasources are URL-addressed, .json drives MIME.
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

// isRateLimitErr: 1Password SDK surfaces vault rate limiting only in the error text.
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

// --- op backend (1Password, op:// refs) ---

func (r opBackend) Resolve(ctx context.Context, ref string) (string, error) {
	return r.client.Secrets().Resolve(ctx, ref)
}

// newOpBackend lazily builds the op backend: OP_SERVICE_ACCOUNT_TOKEN gates it,
// so it only fires when an op:// ref appears. Tests swap in a mock.
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

// --- gcp backend (GCP Secret Manager, gcp:// refs) ---

// Resolve parses gcp://<project>/<secret>[/<version>] (version default latest),
// accesses the version, returns its payload bytes.
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

// parseGCPRef splits gcp://<project>/<secret>[/<version>], version default latest.
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

// newGCPBackend lazily builds the GCP backend via Application Default
// Credentials (google ADC chain, no explicit creds). Tests swap in a mock.
var newGCPBackend = func(ctx context.Context) (secretResolver, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return gcpBackend{client}, nil
}

// --- dispatcher ---

// secretSleep paces secret-resolve retries; tests stub it to a no-op.
var secretSleep = time.Sleep

// backendFactory builds the secret backend for a scheme; the two factory vars
// are swappable in tests.
func backendFactory(scheme string) func(context.Context) (secretResolver, error) {
	switch scheme {
	case "op://":
		return newOpBackend
	case "gcp://":
		return newGCPBackend
	}
	return nil
}

// secretFunc returns a secret(ref) template func: the ref's URI scheme picks the
// backend (op:// 1Password, gcp:// GCP Secret Manager). Each backend is inited
// lazily on its first ref and cached for the render. Resolves retry on rate limits.
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

// schemeOf: the secretSchemes prefix ref carries, "" if none.
func schemeOf(ref string) string {
	for _, s := range secretSchemes {
		if strings.HasPrefix(ref, s) {
			return s
		}
	}
	return ""
}

// JSONSchema: Options che.yml schema fragment.
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

// Compose shapes a rendered Body into the final dest bytes per its options.
// Pure: no file writes (che owns writing).
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

// mergeUpsertEnv merges rendered KEY=VALUE defaults under existing (existing
// wins), skipping blank/# lines, and prints the union sorted by key. Port of
// tpl-render-merge-upsert-env.zsh.
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
	readInto(rendered) // defaults first
	readInto(existing) // existing wins
	var out bytes.Buffer
	for _, k := range slices.Sorted(maps.Keys(merged)) {
		fmt.Fprintf(&out, "%s=%s\n", k, merged[k])
	}
	return out.Bytes()
}

// resolveAtIncludes inlines '@path' lines as repoRoot/<path> contents, '~/' -> root/HOME/. Port of fn-tpl-inline-includes.
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

// IsSecretRefPresent: body contains any secretSchemes ref (op:// or gcp://).
func IsSecretRefPresent(body []byte) bool {
	for _, s := range secretSchemes {
		if bytes.Contains(body, []byte(s)) {
			return true
		}
	}
	return false
}

// isAtIncludeLine: line is exactly '@<no-space>', no whitespace.
func isAtIncludeLine(line string) bool {
	if !strings.HasPrefix(line, "@") || len(line) < 2 {
		return false
	}
	return !strings.ContainsAny(line, " \t")
}

// autogenHeader: autogen comment by extension. Port of fn-tpl-make-header.
func autogenHeader(out, tmplPath string) string {
	if strings.HasSuffix(out, ".md") {
		return fmt.Sprintf("<!-- autogenerated using %s -->", tmplPath)
	}
	return fmt.Sprintf("# autogenerated using %s", tmplPath)
}

// SplitFrontmatter splits a leading '---\n...\n---\n' YAML frontmatter block
// from content: returns (frontmatterYAML, body). No leading block -> ("", content).
func SplitFrontmatter(content string) (front, body string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", content
	}
	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) < 3 { // malformed: no closing '---' -> treat as body only
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

// ReadFrontmatter reads path (relative -> under repoRoot) and returns its
// leading frontmatter YAML ("" if none).
func ReadFrontmatter(repoRoot, path string) (string, error) {
	content, err := os.ReadFile(resolveUnder(repoRoot, path))
	if err != nil {
		return "", err
	}
	front, _ := SplitFrontmatter(string(content))
	return front, nil
}

// ReadBody reads path (relative -> under repoRoot) with any leading
// frontmatter block stripped.
func ReadBody(repoRoot, path string) (string, error) {
	content, err := os.ReadFile(resolveUnder(repoRoot, path))
	if err != nil {
		return "", err
	}
	_, body := SplitFrontmatter(string(content))
	return body, nil
}

// mdComment matches an HTML comment, including multi-line and a trailing newline.
var mdComment = regexp.MustCompile(`(?s)<!--.*?-->\n?`)

// mdHeading matches an ATX heading marker of 1-5 '#' before its space (6 stays 6).
var mdHeading = regexp.MustCompile(`(?m)^(#{1,5})( )`)

// RenderMarkdown reads a markdown file (path: absolute, '~/'-expanded via HOME,
// or relative to repoRoot), applies each opt in order, and trims surrounding
// whitespace. Opts:
//
//	"remove-frontmatter":  drop a leading '---\n...\n---\n' YAML block.
//	"strip-comments":      drop HTML comments (incl. multi-line).
//	"normalize-headings":  demote every ATX heading one level (capped at 6).
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

// --- native generators ---

// DirsTree prints the plain nested dir tree of repoRoot's git-tracked files:
// index paths, file leaves dropped, dirs nested + sorted, 2-space indented.
func DirsTree(repoRoot string) (string, error) {
	paths, err := fsutil.ListTrackedFiles(repoRoot)
	if err != nil {
		return "", err
	}
	return renderTree(buildTree(paths), 0), nil
}

// MakefileDoc emits makefile.agents.md from a Makefile's [genai-include] sections.
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
