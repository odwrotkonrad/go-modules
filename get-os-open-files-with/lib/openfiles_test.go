package lib

// [>] 🤖🤖

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func TestRenderDocumentNode(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/render_document_node.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		var node *yaml.Node
		switch raw := c.Input.Args.String(t, 0); raw {
		case "nil":
		case "emptyDocument":
			node = &yaml.Node{Kind: yaml.DocumentNode}
		default:
			node = &yaml.Node{}
			require.NoError(t, yaml.Unmarshal([]byte(raw), node))
		}
		return Render(node)
	})
}

//[<] 🤖🤖
