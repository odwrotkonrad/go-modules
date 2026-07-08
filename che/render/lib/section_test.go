package lib

// [>] 🤖🤖🤖

import "testing"

func TestSectionOpen(t *testing.T) {
	cases := []struct {
		text  string
		label string
		depth int
		ok    bool
	}{
		{"##[>] Onto Host [genai-include]", "Onto Host", 0, true},
		{"##[>] Onto Repo (CI) [genai-include] 🤖🤖", "Onto Repo (CI)", 0, true},
		{"###[>] VM", "VM", 1, true},
		{"##[>] go 🤖🤖", "go", 0, true},
		{"#[what] not a section", "", 0, false},
	}
	for _, c := range cases {
		label, depth, ok := sectionOpen(c.text)
		if ok != c.ok || label != c.label || (ok && depth != c.depth) {
			t.Errorf("sectionOpen(%q) = (%q,%d,%v), want (%q,%d,%v)",
				c.text, label, depth, ok, c.label, c.depth, c.ok)
		}
	}
}

func TestValsComment(t *testing.T) {
	cases := []struct {
		text string
		vals string
		ok   bool
	}{
		{"#[vals] delta|all", "delta|all", true},
		{"#[what] not vals", "", false},
		{"#[why] nope", "", false},
	}
	for _, c := range cases {
		vals, ok := valsComment(c.text)
		if ok != c.ok || vals != c.vals {
			t.Errorf("valsComment(%q) = (%q,%v), want (%q,%v)", c.text, vals, ok, c.vals, c.ok)
		}
	}
}

//[<] 🤖🤖🤖
