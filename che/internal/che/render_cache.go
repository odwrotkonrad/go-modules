package che

// [>] 🤖🤖

import (
	"bytes"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

func (p *ProfileReady) renderSettled(item spec.FileItem) map[string]bool {
	dests := p.resolveTemplateDests(item)
	settled := make(map[string]bool, len(dests))
	for _, d := range dests {
		settled[d.path] = false
	}
	body, err := p.execTemplate(item, dests)
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
