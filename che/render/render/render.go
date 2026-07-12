// [>] 🤖🤖
package render

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

	onepassword "github.com/1password/onepassword-sdk-go"
	"github.com/hairyhenderson/gomplate/v4"
	"github.com/invopop/jsonschema"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/render/lib"
)

// opRetryDelays: backoff between op-resolve attempts that hit a vault rate limit.
var opRetryDelays = []time.Duration{
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
}

// Exec renders body via the gomplate library. gomplate built-ins (file/data/
// strings/regexp/coll/env) are registered by NewRenderer; op resolves op://
// refs through the 1Password SDK; renderDirsTree/renderMakefileDoc are the two
// native generators (repoRoot bound for the tree). name: error messages only.
func Exec(name string, body []byte, repoRoot string) ([]byte, error) {
	return ExecWithCtx(name, body, repoRoot, nil)
}

// ExecWithCtx is Exec with an optional template context: a non-empty itemCtx
// becomes the template's root context (`.key`), fed to gomplate as a temp
// JSON context datasource aliased `.`.
func ExecWithCtx(name string, body []byte, repoRoot string, itemCtx map[string]string) ([]byte, error) {
	ctx := context.Background()
	funcs := template.FuncMap{
		"op":                   opResolver(ctx),
		"renderDirsTree":       func() (string, error) { return DirsTree(repoRoot) },
		"renderRepoGroupIndex": RepoGroupIndexDir,
		"renderMakefileDoc":    MakefileDoc,
		"frontmatter":          func(path string) (string, error) { return ReadFrontmatter(repoRoot, path) },
		"readBody":             func(path string) (string, error) { return ReadBody(repoRoot, path) },
		"renderMarkdown":       func(path string, opts ...string) (string, error) { return RenderMarkdown(repoRoot, path, opts...) },
		"remoteFile":           remoteFileResolver(),
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

// writeCtxFile marshals itemCtx to a temp JSON file and returns its file://
// URL (gomplate context datasources are URL-addressed, .json drives MIME).
func writeCtxFile(itemCtx map[string]string) (*url.URL, func(), error) {
	b, err := json.Marshal(itemCtx)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.CreateTemp("", "che-tpl-ctx-*.json")
	if err != nil {
		return nil, nil, err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, nil, err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return nil, nil, err
	}
	return &url.URL{Scheme: "file", Path: f.Name()}, func() { os.Remove(f.Name()) }, nil
}

// isRateLimitErr: 1Password SDK surfaces vault rate limiting only in the error text.
func isRateLimitErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "rate limit exceeded")
}

// retry runs op, re-running while shouldRetry(err) holds, sleeping the matching
// delay before each retry. Stops after len(delays) retries or first non-retryable result.
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

// secretResolver resolves one op:// ref to its secret value.
type secretResolver interface {
	Resolve(ctx context.Context, ref string) (string, error)
}

// sdkResolver adapts the 1Password SDK client to secretResolver.
type sdkResolver struct{ client *onepassword.Client }

func (r sdkResolver) Resolve(ctx context.Context, ref string) (string, error) {
	return r.client.Secrets().Resolve(ctx, ref)
}

// newSecretResolver constructs the real 1Password client; tests swap in a mock
// so the SDK never runs.
var newSecretResolver = func(ctx context.Context, token string) (secretResolver, error) {
	client, err := onepassword.NewClient(ctx,
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo("che", "1.0.0"),
	)
	if err != nil {
		return nil, err
	}
	return sdkResolver{client}, nil
}

// opSleep paces op-resolve retries; tests stub it to a no-op.
var opSleep = time.Sleep

// opResolver returns an op(ref) template func that lazily inits one 1Password client
// (OP_SERVICE_ACCOUNT_TOKEN) on first use and reuses it for the render's references.
// Resolves retry on vault rate-limit errors.
func opResolver(ctx context.Context) func(string) (string, error) {
	var client secretResolver
	return func(ref string) (string, error) {
		if client == nil {
			token := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN")
			if token == "" {
				return "", fmt.Errorf("op %q: OP_SERVICE_ACCOUNT_TOKEN unset", ref)
			}
			c, err := newSecretResolver(ctx, token)
			if err != nil {
				return "", fmt.Errorf("op client: %w", err)
			}
			client = c
		}
		secret, err := retry(opRetryDelays, opSleep, isRateLimitErr, func() (string, error) {
			return client.Resolve(ctx, ref)
		})
		if err != nil {
			return "", fmt.Errorf("op resolve %q: %w", ref, err)
		}
		return secret, nil
	}
}

// Options are per-dest render options, unmarshaled directly from che.yml's
// per-dest `options:` mapping. WriteType selects how the rendered body lands:
// "" (overwrite, default: Header + body) | "mergeUpsert" (env KEY=VALUE union
// under Existing). SkipAutoGeneratedHeader drops the autogen header on the
// overwrite path (dests whose format forbids comments, e.g. private keys).
// RenderReferencedFiles inlines @-includes (overwrite path).
type Options struct {
	WriteType               string `yaml:"writeType"`
	SkipAutoGeneratedHeader bool   `yaml:"skipAutoGeneratedHeader"`
	RenderReferencedFiles   bool   `yaml:"renderReferencedFiles"`
}

// JSONSchema is Options' che.yml JSON Schema fragment, mirroring the yaml
// mapping above (writeType enum from the WriteType* consts).
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

// WriteTypeMergeUpsert is the WriteType that merges env KEY=VALUE under the existing dest.
const WriteTypeMergeUpsert = "mergeUpsert"

// Composition is one dest's inputs for Compose: the rendered Body plus how to
// shape it into the final file. Existing is the dest's current content (read by
// che), used only by mergeUpsert.
type Composition struct {
	Body       []byte  // rendered template output (from Exec)
	Opts       Options // per-dest render options
	HeaderDest string  // dest path: picks header comment syntax (.md vs #)
	TmplName   string  // source name shown in the autogen header
	Existing   []byte  // current dest content (mergeUpsert only)
	RepoRoot   string  // base for @-include resolution
}

// Compose shapes a rendered Body into the final dest bytes per its options:
// mergeUpsert merges env KEY=VALUE under Existing (no header); overwrite
// (default) writes Header + Body (header skipped when SkipAutoGeneratedHeader
// is set), inlining @-includes when RenderReferencedFiles is set. Pure: no
// file writes (che owns writing).
func Compose(c Composition) []byte {
	if c.Opts.WriteType == WriteTypeMergeUpsert {
		return mergeUpsertEnv(c.Existing, c.Body)
	}
	var out bytes.Buffer
	if !c.Opts.SkipAutoGeneratedHeader {
		out.WriteString(header(c.HeaderDest, c.TmplName))
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

// IsSecretRefPresent: body contains an op:// secret reference (a render-time vault fetch).
func IsSecretRefPresent(body []byte) bool {
	return bytes.Contains(body, []byte("op://"))
}

// isAtIncludeLine: line is exactly '@<no-space>', no whitespace.
func isAtIncludeLine(line string) bool {
	if !strings.HasPrefix(line, "@") || len(line) < 2 {
		return false
	}
	return !strings.ContainsAny(line, " \t")
}

// header: autogen comment by extension. Port of fn-tpl-make-header.
func header(out, tmplPath string) string {
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

// resolveUnder joins path under base unless it is already absolute.
func resolveUnder(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

// ReadFrontmatter reads path (relative -> under repoRoot) and returns its leading
// frontmatter YAML ("" if none). Template funcs pipe it to data.YAML.
func ReadFrontmatter(repoRoot, path string) (string, error) {
	content, err := os.ReadFile(resolveUnder(repoRoot, path))
	if err != nil {
		return "", err
	}
	front, _ := SplitFrontmatter(string(content))
	return front, nil
}

// ReadBody reads path (relative -> under repoRoot) and returns its content with
// any leading frontmatter block stripped. This is the frontmatter-aware file read.
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
			body = mdHeading.ReplaceAllString(body, "$1#$2")
		default:
			return "", fmt.Errorf("renderMarkdown: unknown opt %q", opt)
		}
	}
	return strings.TrimSpace(body), nil
}

// --- native generators ---

type treeNode map[string]treeNode

// DirsTree prints the plain nested dir tree of repoRoot's git-tracked files:
// index paths, file leaves dropped, dirs nested + sorted, 2-space indented.
func DirsTree(repoRoot string) (string, error) {
	paths, err := fsutil.TrackedFiles(repoRoot)
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
