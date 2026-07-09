package lib

// [>] 🤖🤖🤖

import (
	"embed"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func TestSectionOpen(t *testing.T) {
	type in struct {
		Args []string
	}
	type want struct {
		Label string
		Depth int
		OK    bool `yaml:"ok"`
	}
	type c struct {
		Name string
		In   in
		Want want
	}
	testyml.Run(t, td, "testdata/spec/section_open.spec.yml", func(t *testing.T, c c) {
		label, depth, ok := sectionOpen(c.In.Args[0])
		if ok != c.Want.OK || label != c.Want.Label || (ok && depth != c.Want.Depth) {
			t.Errorf("sectionOpen(%q) = (%q,%d,%v), want %+v", c.In.Args[0], label, depth, ok, c.Want)
		}
	})
}

func TestValsComment(t *testing.T) {
	type in struct {
		Args []string
	}
	type want struct {
		Vals string
		OK   bool `yaml:"ok"`
	}
	type c struct {
		Name string
		In   in
		Want want
	}
	testyml.Run(t, td, "testdata/spec/vals_comment.spec.yml", func(t *testing.T, c c) {
		vals, ok := valsComment(c.In.Args[0])
		if ok != c.Want.OK || vals != c.Want.Vals {
			t.Errorf("valsComment(%q) = (%q,%v), want %+v", c.In.Args[0], vals, ok, c.Want)
		}
	})
}

// [<] 🤖🤖🤖
