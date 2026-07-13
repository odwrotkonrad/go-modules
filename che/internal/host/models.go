package host

// [>] 🤖🤖

import (
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

// Domain model:
//
//	Host     op executor anchored at one repo checkout: source tree, invoking
//	         identity, profile, runtime config, fs seams
//	Service  one resolved launchd job, located via its plist under root/
//	         (plistSource: the candidate template paths)
//	tmplDest one resolved template dest: live path, host vs repo kind,
//	         per-dest render options

// Host is the live system the load ops act on.
type Host struct {
	RepoRoot string // <configs> dir (contains che.yml, ci/, templates/)
	Root     string // <configs>/root, the load ops' source subtree
	Home     string
	Profile  string // "<space>/<os>-<arch>"
	cfg      options.Options
	logSub   string
	fs       fsutil.FileSystemWriter
	reader   fsutil.FileSystemReader
	fetcher  RemoteFetcher
}

// RemoteFetcher fetches a remote template source ref's content
// (<repo>//<path>[?ref=<ref>], marker stripped).
type RemoteFetcher interface {
	Fetch(ref string) (string, error)
}

// Service is one resolved launchd job.
type Service struct {
	Name        string // label == plist basename
	Plist       string // live dest plist path
	Domain      string // "system" or "gui/<uid>"
	Sudo        bool   // system domain only
	LongRunning bool   // KeepAlive, expect a live pid after bootstrap
}

// plistSource is one candidate template path under root/.
type plistSource struct {
	rel    string // repo-relative under root/, with marker
	marker string // ".ontoHost.cp" or ".ontoHost.tpl"
	system bool   // LaunchDaemons -> system, LaunchAgents -> gui
}

// tmplDest is one resolved template dest: live absolute path, host vs repo
// kind (dest path decides: ~/ or absolute -> host, relative -> repo), the
// per-dest options, and the header path Compose stamps.
type tmplDest struct {
	path   string
	host   bool
	opts   render.Options
	header string
}

// gitFetcher is the live RemoteFetcher: shallow in-memory git clones, one
// clone cache shared across the Host's renders.
type gitFetcher struct{ fetch func(string) (string, error) }

type scriptResult struct {
	script string
	status string // "ok" | "fail"
}

// tmplItem pairs a resolved template item with its rendered dests.
type tmplItem struct {
	item  spec.FileItem
	dests []tmplDest
}

// [<] 🤖🤖
