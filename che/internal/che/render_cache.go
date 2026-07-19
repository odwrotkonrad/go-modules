package che

// [>] 🤖🤖

import (
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

// mockRenderHash renders the item's template with mocked secret values and
// hashes the output, the render-delta signal (secrets never resolved).
func (p *ProfileReady) mockRenderHash(item spec.FileItem) (string, error) {
	src, tmplPath, err := p.readTemplateSrc(item)
	if err != nil {
		return "", err
	}
	body, err := render.ExecWithCtxMockSecrets(tmplPath, src, p.resolveRepoRoot(), item.Ctx)
	if err != nil {
		return "", err
	}
	return hashHex(body), nil
}

// renderCounts is the render-templates discovery numbers: all counts every
// resolved dest, delta the dests whose mock-render hash differs from (or is
// absent in) the cache; a failing mock render counts its dests as delta. The
// cache is read-only here: only real renders write it.
func (p *ProfileReady) renderCounts(templates []spec.FileItem) (int, int) {
	all, delta := 0, 0
	for _, item := range templates {
		dests := p.resolveTemplateDests(item)
		all += len(dests)
		hash, err := p.mockRenderHash(item)
		if err != nil {
			delta += len(dests)
			continue
		}
		for _, d := range dests {
			if p.readRenderHash(d.path) != hash {
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
