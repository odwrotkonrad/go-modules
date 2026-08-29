package che

// [>] 🤖🤖

import (
	"bytes"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

func (p *ProfileReady) renderSettled(item spec.FileItem) map[string]bool {
	dests := p.resolveTemplateDests(item)
	settled := make(map[string]bool, len(dests))
	for _, d := range dests {
		settled[d.path] = false
	}
	src, templatePath, err := p.readTemplateSrc(item)
	if err != nil {
		return settled
	}
	body, err := render.ExecWithData(templatePath, src, p.templateAnchor(item), p.templateData(item))
	if err != nil {
		return settled
	}
	for _, d := range dests {
		cur, readErr := p.readExistingDest(d)
		out, composeErr := p.composeDest(item, d, body)
		settled[d.path] = readErr == nil && composeErr == nil && bytes.Equal(cur, out)
	}
	return settled
}

// [<] 🤖🤖
