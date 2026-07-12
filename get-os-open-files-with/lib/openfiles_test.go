package lib

// [>] 🤖🤖

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRenderDocumentNode(t *testing.T) {
	if out, err := Render(nil); err != nil || out != "" {
		t.Fatalf("Render(nil) = %q, %v", out, err)
	}
	if out, err := Render(&yaml.Node{Kind: yaml.DocumentNode}); err != nil || out != "" {
		t.Fatalf("Render(empty doc) = %q, %v", out, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte("app:\n  editor: [pdf]\n"), &doc); err != nil {
		t.Fatal(err)
	}
	out, err := Render(&doc)
	if err != nil || out != "app pdf editor" {
		t.Fatalf("Render(doc) = %q, %v", out, err)
	}
}

//[<] 🤖🤖
