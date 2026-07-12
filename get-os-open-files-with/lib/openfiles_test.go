package lib

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRenderDocumentNode(t *testing.T) {
	out, err := Render(nil)
	require.NoError(t, err)
	assert.Empty(t, out)
	out, err = Render(&yaml.Node{Kind: yaml.DocumentNode})
	require.NoError(t, err)
	assert.Empty(t, out)
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("app:\n  editor: [pdf]\n"), &doc))
	out, err = Render(&doc)
	require.NoError(t, err)
	assert.Equal(t, "app pdf editor", out)
}

//[<] 🤖🤖
