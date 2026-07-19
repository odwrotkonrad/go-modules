package che

// [>] 🤖🤖

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

// renderCachePath is a dest's stored mock-render hash file:
// $XDG_STATE_HOME/che/render-cache/<sha256(dest)>, most recent hash only.
func (p *ProfileReady) renderCachePath(dest string) string {
	return filepath.Join(fsutil.ResolveStateHome(p.home), "render-cache", hashHex([]byte(dest)))
}

func hashHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (p *ProfileReady) readRenderHash(dest string) string {
	b, err := os.ReadFile(p.renderCachePath(dest))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (p *ProfileReady) writeRenderHash(dest, hash string) {
	path := p.renderCachePath(dest)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(hash+"\n"), 0o644)
}

// renderSettled maps each of the item's dest paths to whether a render would
// leave it unchanged, the render-delta signal. Secret-free templates compare
// the mock render's composed output against the dest's current bytes (exact:
// mock == real without secrets); secret-bearing templates compare the
// mock-render hash against the cache ([why] mock values differ from resolved
// ones, content comparison would always differ). A failing read or mock render
// marks every dest unsettled.
func (p *ProfileReady) renderSettled(item spec.FileItem) map[string]bool {
	dests := p.resolveTemplateDests(item)
	settled := make(map[string]bool, len(dests))
	for _, d := range dests {
		settled[d.path] = false
	}
	src, tmplPath, err := p.readTemplateSrc(item)
	if err != nil {
		return settled
	}
	body, err := render.ExecWithCtxMockSecrets(tmplPath, src, p.resolveRepoRoot(), item.Ctx)
	if err != nil {
		return settled
	}
	if render.IsSecretRefPresent(src) {
		hash := hashHex(body)
		for _, d := range dests {
			settled[d.path] = p.readRenderHash(d.path) == hash
		}
		return settled
	}
	for _, d := range dests {
		cur, err := p.readExistingDest(d)
		settled[d.path] = err == nil && bytes.Equal(cur, p.composeDest(item, d, body))
	}
	return settled
}

// renderCounts is the render-templates discovery numbers: all counts every
// resolved dest, delta the unsettled ones (renderSettled).
func (p *ProfileReady) renderCounts(templates []spec.FileItem) (int, int) {
	all, delta := 0, 0
	for _, item := range templates {
		for _, ok := range p.renderSettled(item) {
			all++
			if !ok {
				delta++
			}
		}
	}
	return all, delta
}

// storeRenderHashes refreshes the render-delta cache after a real render: the
// mock-render hash of src (mock re-render only when secrets are present, else
// the rendered body hashes directly), written per dest.
func (p *ProfileReady) storeRenderHashes(item spec.FileItem, dests []tmplDest, tmplPath string, src, body []byte) {
	hash := hashHex(body)
	if render.IsSecretRefPresent(src) {
		mocked, err := render.ExecWithCtxMockSecrets(tmplPath, src, p.resolveRepoRoot(), item.Ctx)
		if err != nil {
			return
		}
		hash = hashHex(mocked)
	}
	for _, d := range dests {
		p.writeRenderHash(d.path, hash)
	}
}

// [<] 🤖🤖
