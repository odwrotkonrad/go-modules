package che

// [>] 🤖🤖

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestToDest(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/to_dest.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		p := &ProfileReady{home: "/Users/x"}
		return p.toDest(c.Input.Args.String(t, 0)), nil
	})
}

func TestSrc(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/src.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		a := c.Input.Args
		p := newProfile(a.String(t, 0), "/Users/x", options.Options{}).withDir(a.String(t, 0))
		return p.resolveSrc(a.String(t, 1)), nil
	})
}

// TestResolveScripts: args name the fixture scripts to create and the rels to
// resolve; expected.output rels are joined under the fixture dir.
func TestResolveScripts(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/resolve_scripts.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) {
		a := c.Input.Args
		files := map[string]string{}
		for _, rel := range a.Strings(t, 0) {
			files[rel] = "#!/bin/sh\n"
		}
		dir := testutil.Tree(t, files)
		p := newProfile(dir, "/Users/x", options.Options{}).withDir(dir)
		got, err := p.resolveScripts(a.Strings(t, 1))
		if c.Expected.Check(t, err) {
			return
		}
		want := make([]string, len(c.Expected.Output))
		for i, rel := range c.Expected.Output {
			want[i] = filepath.Join(dir, rel)
		}
		assert.Equal(t, want, got)
	})
}

// [<] 🤖🤖
