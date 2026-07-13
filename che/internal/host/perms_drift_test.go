package host

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func octal(t *testing.T, s string) os.FileMode {
	t.Helper()
	n, err := strconv.ParseUint(s, 8, 32)
	require.NoErrorf(t, err, "mode %q", s)
	return os.FileMode(n)
}

var specialBits = map[string]os.FileMode{"setgid": os.ModeSetgid, "sticky": os.ModeSticky}

// permsDriftWant: settled asserts an empty delta run, deltaLine the reported
// drift, wetPerm/wetSetgid the on-disk state after the wet run fixes it.
type permsDriftWant struct {
	Settled   bool   `yaml:"settled"`
	DeltaLine string `yaml:"deltaLine"`
	WetPerm   string `yaml:"wetPerm"`
	WetSetgid bool   `yaml:"wetSetgid"`
}

// TestPermsDrift: mode drift on an existing extra-dir. [why] regression:
// os.FileMode encodes setgid/sticky outside 0o7000, so specs like 2775 must
// round-trip without perpetual chmod; settled dests must print nothing and a
// delta run must never mutate.
func TestPermsDrift(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/perms_drift.test.spec.yml", func(t *testing.T, c testyml.Case[permsDriftWant]) {
		var mkModeArg, setBits, chmodArg string
		a := c.Input.Args
		for i := range a {
			switch name := a.Name(i); name {
			case "mkMode":
				mkModeArg = a.String(t, i)
			case "setBits":
				setBits = a.String(t, i)
			case "chmod":
				chmodArg = a.String(t, i)
			default:
				t.Fatalf("unknown arg %q", name)
			}
		}
		home := t.TempDir()
		dest := filepath.Join(home, "drift-dir")
		require.NoError(t, os.Mkdir(dest, octal(t, mkModeArg)))
		if setBits != "" {
			bit, ok := specialBits[setBits]
			require.Truef(t, ok, "unknown setBits %q", setBits)
			require.NoError(t, os.Chmod(dest, octal(t, mkModeArg)|bit))
		}
		items := []spec.FileItem{{Dests: []spec.DestSpec{{Path: dest}}, Perms: spec.Perms{Chmod: chmodArg}}}
		fi, err := os.Stat(dest)
		require.NoError(t, err)
		modeBefore := fi.Mode()

		delta := New(home, filepath.Join(home, "root"), home, testutil.CheProfile, options.Options{DryRun: options.DryRun.Delta})
		deltaRun := func() string {
			out, err := testutil.CaptureStdout(t, func() error { return delta.MakeDirs(items) })
			require.NoError(t, err)
			return testutil.StripANSI(out)
		}

		w := c.Expected.Output
		out := deltaRun()
		if w.Settled {
			assert.Empty(t, strings.TrimSpace(out), "settled dest must print nothing")
		} else {
			testyml.MustMatch(t, out, testyml.Expand(w.DeltaLine, map[string]string{"DEST": dest}))
		}
		fi, err = os.Stat(dest)
		require.NoError(t, err)
		assert.Equal(t, modeBefore, fi.Mode(), "delta must not mutate (dry run)")

		if w.WetPerm == "" {
			return
		}
		wet := New(home, filepath.Join(home, "root"), home, testutil.CheProfile, options.Options{})
		_, err = testutil.CaptureStdout(t, func() error { return wet.MakeDirs(items) })
		require.NoError(t, err)
		fi, err = os.Stat(dest)
		require.NoError(t, err)
		assert.Equal(t, octal(t, w.WetPerm), fi.Mode().Perm(), "wet run must fix the mode")
		if w.WetSetgid {
			assert.NotZero(t, fi.Mode()&os.ModeSetgid, "wet run must add setgid")
		}
		assert.Empty(t, strings.TrimSpace(deltaRun()), "settled dest must print nothing after fix")
	})
}

// [<] 🤖🤖
