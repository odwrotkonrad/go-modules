package packages

// [>] 🤖🤖

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func stubInstallNoise(fail bool) func(argv []string) ([]byte, error) {
	return func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		if strings.HasPrefix(joined, "brew list") {
			return nil, fmt.Errorf("not installed")
		}
		if strings.HasPrefix(joined, "brew install") {
			if fail {
				return []byte("brew noise\n"), fmt.Errorf("exit 1")
			}
			return []byte("brew noise\n"), nil
		}
		return nil, nil
	}
}

func newSilenceInstaller(t *testing.T, opts Options, fail bool) (*Installer, *bytes.Buffer, *[]string) {
	t.Helper()
	in, m := newInstaller(t, "packages:\n  bat: [brew]", "darwin", cmdMap([]string{"brew"}), opts)
	buf := &bytes.Buffer{}
	in.Out = buf
	emitted := &[]string{}
	in.Emit = func(_ log.Level, action, msg string) { *emitted = append(*emitted, action+" "+msg) }
	m.Stub = stubInstallNoise(fail)
	return in, buf, emitted
}

func TestSilenceInstallStdoutHidesSuccessOutput(t *testing.T) {
	in, buf, emitted := newSilenceInstaller(t, Options{SilenceInstallStdout: true}, false)
	require.NoError(t, in.Install([]string{"bat"}))
	require.Empty(t, buf.String())
	require.Contains(t, *emitted, "installed bat via brew")
}

func TestSilenceInstallStdoutPrintsFailureOutput(t *testing.T) {
	in, buf, _ := newSilenceInstaller(t, Options{SilenceInstallStdout: true}, true)
	require.Error(t, in.Install([]string{"bat"}))
	require.Contains(t, buf.String(), "brew noise")
}

func TestSilenceInstallStdoutOffStreams(t *testing.T) {
	in, buf, _ := newSilenceInstaller(t, Options{}, false)
	require.NoError(t, in.Install([]string{"bat"}))
	require.Contains(t, buf.String(), "brew noise")
}

// [<] 🤖🤖
