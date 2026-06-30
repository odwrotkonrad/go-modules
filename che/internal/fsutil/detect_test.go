package fsutil

// [>] 🤖🤖

import "testing"

func TestNormalize(t *testing.T) {
	osCases := map[string]string{"darwin": "macos", "linux": "linux"}
	for in, want := range osCases {
		if got := NormalizeOS(in); got != want {
			t.Errorf("NormalizeOS(%q) = %q, want %q", in, got, want)
		}
	}
}

// [<] 🤖🤖
