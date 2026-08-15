package lib

// [>] 🤖🤖🤖

import (
	"embed"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

type sectionWant struct {
	Label string `yaml:"label"`
	Depth int    `yaml:"depth"`
	OK    bool   `yaml:"ok"`
}

func TestParseSectionOpen(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/parse_section_open.test.spec.yml", func(t *testing.T, c testyml.Case[sectionWant]) (sectionWant, error) {
		label, depth, ok := parseSectionOpen(c.Input.Args.String(t, 0))
		if !ok {
			depth = 0
		}
		return sectionWant{Label: label, Depth: depth, OK: ok}, nil
	})
}

type valsWant struct {
	Vals string `yaml:"vals"`
	OK   bool   `yaml:"ok"`
}

func TestParseTagComment(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/parse_tag_comment.test.spec.yml", func(t *testing.T, c testyml.Case[valsWant]) (valsWant, error) {
		vals, ok := parseTagComment(c.Input.Args.String(t, 0), "vals")
		return valsWant{Vals: vals, OK: ok}, nil
	})
}

// [<] 🤖🤖🤖
