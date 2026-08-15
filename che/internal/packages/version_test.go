package packages

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestPinMatches(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/pin_matches.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		return PinMatches(c.Input.Args.String(t, 0), c.Input.Args.String(t, 1)), nil
	})
}

func TestEntryVersionPinsArchiveVersion(t *testing.T) {
	const y = `packages:
  kind:
    version: "0.32.0"
    installers:
      - binariesRemoteArchive:
          platformEligibility: [{linux-amd64: sha256:goodsha}]
          url: https://example.com/kind-{version}
`
	body := []byte("kind-bin")
	in, _ := newInstaller(t, withSha(y, body), "linux", cmdMap(nil), Options{})
	tempHome(t, in)
	testFetch.Bodies["https://example.com/kind-0.32.0"] = body
	require.NoError(t, in.Install([]string{"kind"}))
	require.Contains(t, testFetch.Calls(), "https://example.com/kind-0.32.0")
}

func TestArchiveWithoutVersionNeedsNoPin(t *testing.T) {
	const y = `packages:
  aws:
    - binariesRemoteArchive:
        platformEligibility: [linux-amd64]
        extractBinaries: [aws/dist/aws]
        url: https://example.com/aws-latest.zip
`
	in, _ := newInstaller(t, y, "linux", cmdMap(nil), Options{})
	tempHome(t, in)
	testFetch.Bodies["https://example.com/aws-latest.zip"] = zipBody(t, "aws/dist/aws")
	require.NoError(t, in.Install([]string{"aws"}))
	require.Contains(t, testFetch.Calls(), "https://example.com/aws-latest.zip")
}

func TestAliasBinaryLinksRenamedBinary(t *testing.T) {
	const y = `packages:
  bat:
    installers:
      - brew
      - apt:
          aliasBinary:
            batcat: bat
`
	cmds := cmdMap([]string{"apt-get"})
	in, m := newInstaller(t, y, "linux", cmds, Options{})
	home := tempHome(t, in)
	m.Stub = func(argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		if strings.HasPrefix(joined, "dpkg -s") {
			return nil, errNotInstalled
		}
		if strings.Contains(joined, "apt-get install") {
			cmds["batcat"] = "/usr/bin/batcat"
		}
		return nil, nil
	}
	require.NoError(t, in.Install([]string{"bat"}))
	requireCalls(t, m, "--no-install-recommends bat")
	resolved, err := os.Readlink(filepath.Join(home, ".local", "bin", "bat"))
	require.NoError(t, err)
	require.Equal(t, "/usr/bin/batcat", resolved)
}

func TestAliasBinarySkippedWhenSourceAbsent(t *testing.T) {
	const y = `packages:
  bat:
    installers:
      - apt:
          aliasBinary:
            batcat: bat
`
	in, _ := newInstaller(t, y, "darwin", cmdMap([]string{"brew", "bat"}), Options{})
	home := tempHome(t, in)
	require.NoError(t, in.Install([]string{"bat"}))
	require.NoFileExists(t, filepath.Join(home, ".local", "bin", "bat"))
}

func TestPostInstallShippedScriptResolves(t *testing.T) {
	const y = `packages:
  kitty:
    installers:
      - script:
          url: https://example.com/installer.sh
    postInstall:
      path: scripts/post-install-kitty.sh
`
	in, m := newInstaller(t, y, "darwin", cmdMap(nil), Options{})
	in.FilePath = BuiltinPath
	m.Stub = stubOutputs("curl ", "echo installing\n")
	require.NoError(t, in.Install([]string{"kitty"}))
	requireCalls(t, m, `ln -fs "$app/MacOS/kitten" "$bin/kitten"`, `ln -fs "$app/MacOS/kitty" "$bin/kitty"`)
}

type scriptPathsGot struct {
	Items       []string `yaml:"items"`
	PostInstall string   `yaml:"postInstall"`
}

func TestResolveScriptPaths(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_script_paths.test.spec.yml", func(t *testing.T, c testyml.Case[map[string]scriptPathsGot]) (map[string]scriptPathsGot, error) {
		var f File
		require.NoError(t, yaml.Unmarshal([]byte(c.Input.Args.String(t, 1)), &f))
		f.ResolveScriptPaths(c.Input.Args.String(t, 0))
		got := map[string]scriptPathsGot{}
		for name, e := range f.Packages {
			g := scriptPathsGot{}
			for _, it := range e.Items {
				path := ""
				if it.Script != nil {
					path = it.Script.Path
				}
				g.Items = append(g.Items, path)
			}
			if e.PostInstall != nil {
				g.PostInstall = e.PostInstall.Path
			}
			got[name] = g
		}
		return got, nil
	})
}

func TestOverrideAnchoredScriptRuns(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scripts", "my-post.sh"), []byte("echo user-post\n"), 0o755))
	yml := "packages:\n  mytool:\n    installers: [{script: {run: echo install}}]\n    postInstall: {path: scripts/my-post.sh}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "override.yml"), []byte(yml), 0o644))
	o, err := Load(filepath.Join(dir, "override.yml"))
	require.NoError(t, err)
	o.ResolveScriptPaths(dir)

	in, m := newInstaller(t, "packages: {}", "darwin", cmdMap(nil), Options{})
	base, err := LoadBuiltin()
	require.NoError(t, err)
	base.Merge(o)
	in.File, in.FilePath = base, BuiltinPath
	require.NoError(t, in.Install([]string{"mytool"}))
	requireCalls(t, m, filepath.Join(dir, "scripts", "my-post.sh"))
}

// [<] 🤖🤖
