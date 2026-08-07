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
	body, err := render.ExecWithCtxMockSecrets(tmplPath, src, p.templateAnchor(item), p.mergedCtx(item.Ctx))
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

func (p *ProfileReady) storeRenderHashes(item spec.FileItem, dests []tmplDest, tmplPath string, src, body []byte) {
	hash := hashHex(body)
	if render.IsSecretRefPresent(src) {
		mocked, err := render.ExecWithCtxMockSecrets(tmplPath, src, p.templateAnchor(item), p.mergedCtx(item.Ctx))
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
