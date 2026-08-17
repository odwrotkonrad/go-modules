package packages

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestToolPackages(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/tool_packages.test.spec.yml", func(t *testing.T, c testyml.Case[map[string]map[string]string]) (map[string]map[string]string, error) {
		var f File
		if err := yaml.Unmarshal([]byte(c.Input.Args.String(t, 1)), &f); err != nil {
			return nil, err
		}
		switch c.Input.Args.String(t, 0) {
		case "merge":
			var o File
			require.NoError(t, yaml.Unmarshal([]byte(c.Input.Args.String(t, 2)), &o))
			f.Merge(&o)
		case "delta":
			var b File
			require.NoError(t, yaml.Unmarshal([]byte(c.Input.Args.String(t, 2)), &b))
			f = *f.Delta(&b)
		}
		got := map[string]map[string]string{}
		for tool, set := range f.ToolPackages {
			got[tool] = map[string]string{}
			for name, version := range set {
				if version == "" {
					version = "rolling"
				}
				got[tool][name] = version
			}
		}
		return got, nil
	})
}

const toolPkgsYaml = `toolPackages:
  vscode:
    golang.go:
    redhat.vscode-yaml: 1.24.0
`

func TestInstallToolPackagesWhenMissing(t *testing.T) {
	in, m := newInstaller(t, toolPkgsYaml, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("")
	require.NoError(t, in.InstallToolPackages("vscode", Requests([]string{"golang.go"})))
	requireCalls(t, m, "code --install-extension golang.go")
}

func TestInstallToolPackagesSkipsInstalledAndListsOnce(t *testing.T) {
	in, m := newInstaller(t, toolPkgsYaml, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("Golang.Go\nredhat.vscode-yaml@1.24.0\n")
	require.NoError(t, in.InstallToolPackages("vscode", Requests([]string{"golang.go", "redhat.vscode-yaml"})))
	require.Equal(t, []string{"code --list-extensions --show-versions"}, m.Calls())
}

func TestInstallToolPackagesPinDriftReinstalls(t *testing.T) {
	in, m := newInstaller(t, toolPkgsYaml, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("redhat.vscode-yaml@1.23.0\n")
	require.NoError(t, in.InstallToolPackages("vscode", Requests([]string{"redhat.vscode-yaml"})))
	requireCalls(t, m, "code --install-extension redhat.vscode-yaml@1.24.0")
}

func TestInstallToolPackagesUpdateForcesUnpinned(t *testing.T) {
	in, m := newInstaller(t, toolPkgsYaml, "darwin", cmdMap([]string{"code"}), Options{Update: true})
	m.Stub = codeListStub("golang.go@0.50.0\n")
	require.NoError(t, in.InstallToolPackages("vscode", Requests([]string{"golang.go"})))
	requireCalls(t, m, "code --install-extension golang.go --force")
}

func TestInstallToolPackagesRequestVersionOverridesPin(t *testing.T) {
	in, m := newInstaller(t, toolPkgsYaml, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("")
	require.NoError(t, in.InstallToolPackages("vscode", []Request{{Name: "redhat.vscode-yaml", Versions: []string{"1.20.0"}}}))
	requireCalls(t, m, "code --install-extension redhat.vscode-yaml@1.20.0")
}

func TestInstallToolPackagesDryRunEmitsWithoutInstalling(t *testing.T) {
	in, m := newInstaller(t, toolPkgsYaml, "darwin", cmdMap([]string{"code"}), Options{DryRun: true})
	m.Stub = codeListStub("")
	out, err := captureStdout(t, func() error {
		return in.InstallToolPackages("vscode", Requests([]string{"redhat.vscode-yaml"}))
	})
	require.NoError(t, err)
	refuteCalls(t, m, "--install-extension")
	require.Contains(t, out, "vscode/redhat.vscode-yaml 1.24.0 via vscode")
}

func TestInstallToolPackagesSkipsWhenCommandAbsent(t *testing.T) {
	in, m := newInstaller(t, toolPkgsYaml, "darwin", cmdMap(nil), Options{})
	out, err := captureStdout(t, func() error {
		return in.InstallToolPackages("vscode", Requests([]string{"golang.go"}))
	})
	require.NoError(t, err)
	require.Empty(t, m.Calls())
	require.Contains(t, out, "code command absent")
}

func TestInstallToolPackagesUnknownToolErrors(t *testing.T) {
	in, _ := newInstaller(t, toolPkgsYaml, "darwin", cmdMap([]string{"code"}), Options{})
	err := in.InstallToolPackages("pip", Requests([]string{"requests"}))
	require.ErrorContains(t, err, `unknown tool "pip": want one of gcloud, vscode`)
}

func TestInstallToolPackagesUnknownNameErrors(t *testing.T) {
	in, _ := newInstaller(t, toolPkgsYaml, "darwin", cmdMap([]string{"code"}), Options{})
	err := in.InstallToolPackages("vscode", Requests([]string{"nope.ext"}))
	require.ErrorContains(t, err, "unknown vscode package: nope.ext (required entry in toolPackages.vscode of packages.yml)")
}

const gcloudPkgsYaml = `toolPackages:
  gcloud:
    gke-gcloud-auth-plugin:
    kubectl:
`

func gcloudListStub(listed string) func(argv []string) ([]byte, error) {
	return func(argv []string) ([]byte, error) {
		if argv[0] == "gcloud" && argv[1] == "components" && argv[2] == "list" {
			return []byte(listed), nil
		}
		return nil, nil
	}
}

func TestInstallGcloudComponentsWhenMissing(t *testing.T) {
	in, m := newInstaller(t, gcloudPkgsYaml, "darwin", cmdMap([]string{"gcloud"}), Options{})
	m.Stub = gcloudListStub("kubectl\tNot Installed\n")
	require.NoError(t, in.InstallToolPackages("gcloud", Requests([]string{"kubectl"})))
	requireCalls(t, m, "gcloud components install --quiet kubectl")
}

func TestInstallGcloudComponentsSkipsAnyInstalledState(t *testing.T) {
	in, m := newInstaller(t, gcloudPkgsYaml, "darwin", cmdMap([]string{"gcloud"}), Options{})
	m.Stub = gcloudListStub("gke-gcloud-auth-plugin\tInstalled\nkubectl\tUpdate Available\n")
	require.NoError(t, in.InstallToolPackages("gcloud", Requests([]string{"gke-gcloud-auth-plugin", "kubectl"})))
	require.Equal(t, []string{"gcloud components list --quiet --format=value(id,state.name)"}, m.Calls())
}

func TestInstallGcloudComponentsUpdateMovesTheSdk(t *testing.T) {
	in, m := newInstaller(t, gcloudPkgsYaml, "darwin", cmdMap([]string{"gcloud"}), Options{Update: true})
	m.Stub = gcloudListStub("kubectl\tInstalled\n")
	require.NoError(t, in.InstallToolPackages("gcloud", Requests([]string{"kubectl"})))
	requireCalls(t, m, "gcloud components update --quiet")
}

func TestInstallGcloudComponentsRejectsRequestPin(t *testing.T) {
	in, _ := newInstaller(t, gcloudPkgsYaml, "darwin", cmdMap([]string{"gcloud"}), Options{})
	err := in.InstallToolPackages("gcloud", []Request{{Name: "kubectl", Versions: []string{"580.0.0"}}})
	require.ErrorContains(t, err, "toolPackages.gcloud: kubectl: gcloud packages carry no version of their own, omit the pin")
}

func TestInstallGcloudComponentsSkipsWhenComponentManagerUnavailable(t *testing.T) {
	in, m := newInstaller(t, gcloudPkgsYaml, "darwin", cmdMap(nil), Options{})
	out, err := captureStdout(t, func() error {
		return in.InstallToolPackages("gcloud", Requests([]string{"kubectl"}))
	})
	require.NoError(t, err)
	require.Empty(t, m.Calls())
	require.Contains(t, out, "gcloud command absent")
}

func TestCheckToolPackagesPresent(t *testing.T) {
	in, m := newInstaller(t, toolPkgsYaml, "darwin", cmdMap([]string{"code"}), Options{})
	m.Stub = codeListStub("Golang.Go@0.50.0\n")
	out, err := captureStdout(t, func() error {
		require.Equal(t, []string{"redhat.vscode-yaml"}, in.CheckToolPackagesPresent("vscode", []string{"golang.go", "redhat.vscode-yaml"}))
		return nil
	})
	require.NoError(t, err)
	wantLines(t, out, "missing vscode/redhat.vscode-yaml")
	notLine(t, out, "missing vscode/golang.go")
}

// [<] 🤖🤖
