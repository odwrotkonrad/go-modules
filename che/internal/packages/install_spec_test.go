package packages

// [>] 🤖🤖

import (
	"cmp"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

type stubPair struct {
	Prefix string `yaml:"prefix"`
	Output string `yaml:"output"`
}

type requestSpec struct {
	Name     string   `yaml:"name"`
	Versions []string `yaml:"versions"`
	Global   string   `yaml:"global"`
	Checksum string   `yaml:"checksum"`
}

type installCfg struct {
	OS                string            `yaml:"os"`
	Arch              string            `yaml:"arch"`
	Distro            *string           `yaml:"distro"`
	Cmds              []string          `yaml:"cmds"`
	Update            bool              `yaml:"update"`
	IfMissing         bool              `yaml:"ifMissing"`
	DryRun            bool              `yaml:"dryRun"`
	MissingMethodWarn bool              `yaml:"missingMethodWarn"`
	OnlyMethods       []string          `yaml:"onlyMethods"`
	BuiltinFilePath   bool              `yaml:"builtinFilePath"`
	TempFilePath      bool              `yaml:"tempFilePath"`
	TempHome          bool              `yaml:"tempHome"`
	FailOn            []string          `yaml:"failOn"`
	Stubs             []stubPair        `yaml:"stubs"`
	OsRelease         string            `yaml:"osRelease"`
	FetchBodies       map[string]string `yaml:"fetchBodies"`
	FetchError        string            `yaml:"fetchError"`
	Method            string            `yaml:"method"`
	Packages          []string          `yaml:"packages"`
	Requests          []requestSpec     `yaml:"requests"`
}

type installWant struct {
	Calls      []string       `yaml:"calls"`
	ExactCalls *[]string      `yaml:"exactCalls"`
	CallCounts map[string]int `yaml:"callCounts"`
	Fetches    []string       `yaml:"fetches"`
	Envs       []string       `yaml:"envs"`
	Missing    []string       `yaml:"missing"`
}

func makeInstallStub(m *execx.Mock, cfg installCfg) {
	var stubs []func(argv []string) ([]byte, error)
	if len(cfg.FailOn) > 0 {
		stubs = append(stubs, failOn(cfg.FailOn...))
	}
	if len(cfg.Stubs) > 0 {
		pairs := make([]string, 0, 2*len(cfg.Stubs))
		for _, s := range cfg.Stubs {
			pairs = append(pairs, s.Prefix, s.Output)
		}
		stubs = append(stubs, stubOutputs(pairs...))
	}
	if len(stubs) > 0 {
		m.Stub = chainStubs(stubs...)
	}
}

func runInstallCase(t *testing.T, c testyml.Case[installWant]) {
	t.Helper()
	var cfg installCfg
	c.Input.Args.To(t, 1, &cfg)
	in, m := newInstaller(t, c.Input.Args.String(t, 0), cmp.Or(cfg.OS, "linux"), cmdMap(cfg.Cmds), Options{
		Update: cfg.Update, IfMissing: cfg.IfMissing, DryRun: cfg.DryRun,
		MissingMethodWarn: cfg.MissingMethodWarn, OnlyMethods: cfg.OnlyMethods,
	})
	if cfg.Arch != "" {
		in.Host.Arch = cfg.Arch
	}
	if cfg.Distro != nil {
		in.Host.Distro = *cfg.Distro
	}
	if cfg.BuiltinFilePath {
		in.FilePath = BuiltinPath
	}
	if cfg.TempFilePath {
		in.FilePath = filepath.Join(t.TempDir(), "packages.yml")
	}
	if cfg.TempHome {
		tempHome(t, in)
	}
	makeInstallStub(m, cfg)
	if cfg.OsRelease != "" {
		release := cfg.OsRelease
		in.Host.ReadFile = func(path string) ([]byte, error) {
			if path == "/etc/os-release" {
				return []byte(release), nil
			}
			return nil, fmt.Errorf("read %s: not stubbed", path)
		}
	}
	for url, body := range cfg.FetchBodies {
		testFetch.Bodies[url] = []byte(body)
	}
	if cfg.FetchError != "" {
		testFetch.Err = errors.New(cfg.FetchError)
	}

	out, err := captureStdout(t, func() error {
		switch cmp.Or(cfg.Method, "install") {
		case "install":
			return in.Install(cfg.Packages)
		case "installRequests":
			reqs := make([]Request, 0, len(cfg.Requests))
			for _, r := range cfg.Requests {
				reqs = append(reqs, Request(r))
			}
			return in.InstallRequests(reqs)
		case "checkPresent":
			missing := in.CheckPresent(cfg.Packages)
			if missing == nil {
				missing = []string{}
			}
			want := c.Expected.Output.Missing
			if want == nil {
				want = []string{}
			}
			assert.Equal(t, want, missing, "CheckPresent missing set")
			return nil
		case "checkUpgradable":
			return in.CheckUpgradable(cfg.Packages)
		default:
			t.Fatalf("unknown method %q", cfg.Method)
			return nil
		}
	})
	c.Expected.Check(t, err)

	joined := strings.Join(m.Calls(), "\n")
	for _, matcher := range c.Expected.Output.Calls {
		testyml.MustMatch(t, joined, matcher)
	}
	for _, matcher := range c.NotExpected.Output.Calls {
		testyml.MustNotMatch(t, joined, matcher)
	}
	if want := c.Expected.Output.ExactCalls; want != nil {
		calls := m.Calls()
		if assert.Lenf(t, calls, len(*want), "exact calls, got:\n%s", joined) {
			for i, matcher := range *want {
				if !testyml.IsMatchFull(calls[i], matcher) {
					t.Errorf("call[%d] %q does not fully match %q", i, calls[i], matcher)
				}
			}
		}
	}
	for fragment, n := range c.Expected.Output.CallCounts {
		testyml.MustCount(t, joined, fragment, n)
	}
	fetches := strings.Join(testFetch.Calls(), "\n")
	for _, matcher := range c.Expected.Output.Fetches {
		testyml.MustMatch(t, fetches, matcher)
	}
	for _, matcher := range c.NotExpected.Output.Fetches {
		testyml.MustNotMatch(t, fetches, matcher)
	}
	if len(c.Expected.Output.Envs) > 0 {
		var envs []string
		for _, e := range m.Envs() {
			envs = append(envs, strings.Join(e, "\n"))
		}
		joinedEnvs := strings.Join(envs, "\n")
		for _, matcher := range c.Expected.Output.Envs {
			testyml.MustMatch(t, joinedEnvs, matcher)
		}
	}
	for _, matcher := range c.Expected.StdOut {
		testyml.MustMatch(t, out, matcher)
	}
	for _, matcher := range c.NotExpected.StdOut {
		testyml.MustNotMatch(t, out, matcher)
	}
}

func runInstallSpec(t *testing.T, path string) {
	t.Helper()
	testyml.Run(t, td, path, func(t *testing.T, c testyml.Case[installWant]) { runInstallCase(t, c) })
}

func TestInstallNix(t *testing.T) {
	runInstallSpec(t, "testdata/spec/funcs/install_nix.test.spec.yml")
}

func TestInstallApt(t *testing.T) {
	runInstallSpec(t, "testdata/spec/funcs/install_apt.test.spec.yml")
}

func TestInstallManagers(t *testing.T) {
	runInstallSpec(t, "testdata/spec/funcs/install_managers.test.spec.yml")
}

func TestInstallVersions(t *testing.T) {
	runInstallSpec(t, "testdata/spec/funcs/install_versions.test.spec.yml")
}

func TestInstallRequests(t *testing.T) {
	runInstallSpec(t, "testdata/spec/funcs/install_requests.test.spec.yml")
}

func TestInstallBasePackages(t *testing.T) {
	runInstallSpec(t, "testdata/spec/funcs/base_packages.test.spec.yml")
}

func TestCheck(t *testing.T) { runInstallSpec(t, "testdata/spec/funcs/check.test.spec.yml") }

// [<] 🤖🤖
