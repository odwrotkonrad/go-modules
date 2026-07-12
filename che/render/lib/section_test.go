package lib

// [>] 🤖🤖🤖

import (
	"embed"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

// sectionWant is section_open's expected.output.
type sectionWant struct {
	Label string `yaml:"label"`
	Depth int    `yaml:"depth"`
	OK    bool   `yaml:"ok"`
}

func TestSectionOpen(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/section_open.test.spec.yml", func(t *testing.T, c testyml.Case[sectionWant]) (sectionWant, error) {
		label, depth, ok := sectionOpen(c.Input.Args.String(t, 0))
		if !ok {
			depth = 0
		}
		return sectionWant{Label: label, Depth: depth, OK: ok}, nil
	})
}

// valsWant is vals_comment's expected.output.
type valsWant struct {
	Vals string `yaml:"vals"`
	OK   bool   `yaml:"ok"`
}

func TestValsComment(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/tag_comment.test.spec.yml", func(t *testing.T, c testyml.Case[valsWant]) (valsWant, error) {
		vals, ok := tagComment(c.Input.Args.String(t, 0), "vals")
		return valsWant{Vals: vals, OK: ok}, nil
	})
}

// [<] 🤖🤖🤖
