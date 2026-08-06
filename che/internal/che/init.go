package che

// [>] 🤖🤖

import (
	"fmt"
	"path/filepath"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

// InitSources is the init stage: prefetch every spec source reachable from the
// root spec — top-level include.sources plus every profile's sourced
// include.profiles refs — recursive, cycle-guarded, runIf NOT evaluated
// (init fetches everything declared; discovery decides what runs). A source
// that cannot be fetched AND has no cached checkout is fatal; a failing update
// with a cache warns and continues (spec/che/init.md).
// SkipRemoteRefs skips remote refs.
func InitSources(ctx Context, opts options.Options) error {
	repoRoot, err := findRepoRoot(ctx)
	if err != nil {
		return err
	}
	log.EmitInfo("discover-profiles", "using-spec", filepath.Join(repoRoot, "che.yml"))
	home, err := resolveInvokingHome(ctx)
	if err != nil {
		return err
	}
	w := &initWalker{home: home, opts: opts, seen: map[string]bool{}}
	return w.walkSpec(spec.SpecSourceRecipe{}, repoRoot, "spec")
}

// initWalker threads the prefetch recursion state.
type initWalker struct {
	home string
	opts options.Options
	seen map[string]bool // resolved spec dirs (cycle/dup guard)
}

// walkSpec resolves one spec source (remote: clone/pull into the run cache),
// then walks its che.yml: include.sources + each profile's sourced refs. Any
// unresolvable source is fatal (no cached checkout to fall back on).
func (w *initWalker) walkSpec(src spec.SpecSourceRecipe, anchor, name string) error {
	if w.opts.SkipRemoteRefs && src.GetSourceType() == spec.SourceTypes.Remote {
		return nil
	}
	ready, err := src.PrepareSource(anchor, w.home)
	if err != nil {
		return fmt.Errorf("init-remote-sources %s: %w", name, err)
	}
	if w.seen[ready.DirectoryPath] {
		return nil
	}
	w.seen[ready.DirectoryPath] = true
	doc, err := spec.Load(ready.DefinitionURI)
	if err != nil {
		return fmt.Errorf("init-remote-sources %s: %w", name, err)
	}
	for _, inc := range doc.Include {
		if err := w.walkSpec(inc, ready.DirectoryPath, "spec"); err != nil {
			return err
		}
	}
	for _, rec := range doc.ProfileRecipes {
		for _, ref := range rec.SourcedRefs() {
			log.EmitTrace("init-remote-sources", "detected-remote-ref", "profile "+ref.ProfileName+": "+ref.String())
			err := w.walkSpec(
				spec.SpecSourceRecipe{SourceRecipe: spec.SourceRecipe{URI: ref.URI, SpecFile: ref.SpecFile}},
				ready.DirectoryPath, ref.ProfileName)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// [<] 🤖🤖
