package cli

// [>] 🤖🤖

import (
	"gitlab.com/konradodwrot/go-modules/che/internal/config"
	"gitlab.com/konradodwrot/go-modules/che/internal/host"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

// Domain model:
//
//	loader
//	  config   config.Config: cobra flag destinations, finalized by init
//	           (env fallbacks, mode validation)
//	  local    load: the local repo
//	  plugins  pluginLoader: one deferred load per plugins entry
//	load
//	  host       host.Host: op executor anchored at the repo checkout
//	  selection  spec.Resolved: the checkout's che.yml resolved for the run
//	  pluginRef, pluginEnv: the plugins entry that produced the checkout
//	             (empty for the local repo)
//
// TODO: rethink this data model

// step is one op of the full load: its subcommand shape (name/parent/short)
// plus the load op and a predicate over the local repo's resolved selection
// gating it under `all`.
type step struct {
	name     string
	parent   string
	short    string
	op       func(*loader, load) error
	selected func(spec.Resolved) bool
}

// load applies one repo's resolved selection onto the host.
type load struct {
	host      host.Host
	selection spec.Resolved
	pluginRef string
	pluginEnv map[string]string
}

// pluginLoader builds each plugins-entry load on first use, caching outcomes
// (nil: skipped by execIf).
type pluginLoader struct {
	refs     []spec.PluginRef
	built    map[string]*load
	repoRoot string
	home     string
	cfg      config.Config
	eval     func(string) (bool, error)
	newHost  func(repoRoot, home, profile string, cfg config.Config) host.Host
}

// loader wires the cobra tree. init (PersistentPreRunE) finalizes config and
// builds the local load and the plugin loader, read by each RunE.
type loader struct {
	config  config.Config
	local   load
	plugins pluginLoader
	// newHost builds each load's Host; tests override it to inject a mock fs.
	newHost func(repoRoot, home, profile string, cfg config.Config) host.Host
}

func New() *loader {
	return &loader{newHost: host.New}
}

// [<] 🤖🤖
