package plugin

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

// slug: deterministic dir name per url form.
func TestSlug(t *testing.T) {
	cases := map[string]string{
		"git@gitlab.com:konradodwrot/workspace.git": "gitlab.com-konradodwrot-workspace",
		"https://gitlab.com/a/b.git":                "gitlab.com-a-b",
		"ssh://git@gitlab.com/a/b.git":              "gitlab.com-a-b",
		"file:///tmp/x":                             "tmp-x",
	}
	for url, want := range cases {
		if got := slug(url); got != want {
			t.Errorf("slug(%q) = %q, want %q", url, got, want)
		}
	}
}

// Ensure: first call shallow-clones into the cache dir (logged as cloned),
// second fast-forward pulls new upstream commits (logged as pulled); a
// no-change pull stays silent (attempt lines are debug-gated).
func TestEnsureCloneThenPull(t *testing.T) {
	up := testutil.Repo(t, map[string]string{"che.yml": "p: {}\n"})
	home := t.TempDir()
	url := "file://" + up

	var dir string
	out, err := testutil.CaptureStdout(t, func() error {
		var e error
		dir, e = Ensure(home, url, "p")
		return e
	})
	if err != nil {
		t.Fatalf("Ensure (clone) errored: %v", err)
	}
	if dir != Dir(home, url) {
		t.Errorf("dir = %q, want %q", dir, Dir(home, url))
	}
	if _, err := os.Stat(filepath.Join(dir, "che.yml")); err != nil {
		t.Fatalf("clone missing che.yml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git/shallow")); err != nil {
		t.Errorf("clone not shallow: %v", err)
	}
	testutil.WantLines(t, out, "plugin(p): cloned "+url+" -> "+dir)
	testutil.NotLine(t, out, "clone "+url)

	out, err = testutil.CaptureStdout(t, func() error {
		_, e := Ensure(home, url, "p")
		return e
	})
	if err != nil {
		t.Fatalf("Ensure (no-change pull) errored: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("no-change pull printed %q, want silence", out)
	}

	testutil.WriteTree(t, up, map[string]string{"extra.txt": "x\n"})
	testutil.GitRepo(t, up)
	out, err = testutil.CaptureStdout(t, func() error {
		_, e := Ensure(home, url, "p")
		return e
	})
	if err != nil {
		t.Fatalf("Ensure (pull) errored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "extra.txt")); err != nil {
		t.Errorf("pull did not fetch extra.txt: %v", err)
	}
	pulledRe := regexp.MustCompile(`plugin\(p\): pulled [0-9a-f]{7}\.\.[0-9a-f]{7} ` + regexp.QuoteMeta(dir))
	if !pulledRe.MatchString(testutil.StripANSI(out)) {
		t.Errorf("pull output %q missing pulled <old7>..<new7> line", out)
	}
	testutil.NotLine(t, out, "pull "+dir)
}

// [<] 🤖🤖
