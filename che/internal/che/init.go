package che

// [>] 🤖🤖

import (
	"fmt"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

func InitSources(ctx Context, opts options.Options) error {
	repoRoot, err := findRepoRoot(ctx)
	if err != nil {
		return err
	}
	log.EmitInfo("discover-profiles", "using-spec", specPathOf(repoRoot))
	home, err := resolveInvokingHome(ctx)
	if err != nil {
		return err
	}
	w := &initWalker{ctx: ctx, home: home, opts: opts, repoFiles: newRepoFilesCache(ctx), seen: map[string]bool{}}
	return w.walkSpec(spec.SpecSourceRecipe{}, repoRoot, "spec", overlay{}, true)
}

type initWalker struct {
	ctx       Context
	home      string
	opts      options.Options
	repoFiles *repoFilesCache
	seen      map[string]bool
}

func (w *initWalker) walkSpec(src spec.SpecSourceRecipe, anchor, name string, over overlay, root bool) error {
	if w.opts.SkipRemoteRefs && src.GetSourceType() == spec.SourceTypes.Remote {
		return nil
	}
	ready, err := src.PrepareSource(anchor, w.home, root)
	if err != nil {
		return fmt.Errorf("init-remote-sources %s: %w", name, err)
	}
	if w.seen[ready.DefinitionURI] {
		return nil
	}
	w.seen[ready.DefinitionURI] = true
	paths := specPaths(ready.SourceReady, root)
	in, err := w.repoFiles.interp(paths, ready.DirectoryPath, root, over, w.opts.EnvUnset)
	if err != nil {
		return fmt.Errorf("init-remote-sources %s: %w", name, err)
	}
	doc, err := spec.LoadMerged(paths, in)
	if err != nil {
		return fmt.Errorf("init-remote-sources %s: %w", name, err)
	}
	for _, inc := range doc.SpecsInclude {
		if inc.Optional && inc.IsAbsentLocalDir(ready.DirectoryPath, w.home) {
			log.EmitSkip(log.Levels.Warn, "init-remote-sources", "prepare", inc.URI, "optional source dir absent")
			continue
		}
		if err := w.walkSpec(inc, ready.DirectoryPath, "spec", overlay{inherited: doc.Lookup, env: inc.Env, passed: inc.Variables, inheritedVars: doc.Vars.ForEmbedded()}, false); err != nil {
			return err
		}
	}
	for _, rec := range doc.ProfileRecipes {
		if unset := doc.ProfileEnvUnset(rec.Source.GetProfileName()); len(unset) > 0 {
			log.EmitSkip(log.Levels.Trace, "init-remote-sources", "prepare", rec.Source.GetProfileName(), "unset env refs")
			continue
		}
		for _, ref := range rec.SourcedRefs() {
			log.EmitTrace("init-remote-sources", "detected-remote-ref", "profile "+ref.ProfileName+": "+ref.String())
			err := w.walkSpec(ref.AsSpecSource(), ready.DirectoryPath, ref.ProfileName, overlay{inherited: doc.Lookup, env: ref.Env, passed: ref.Variables, inheritedVars: doc.Vars.ForEmbeddedProfiles()}, false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// [why] the invoked spec is che.yml and .che/che.yml read as one; an embedded spec is the one
// candidate file that resolved
func specPaths(ready spec.SourceReady, root bool) []string {
	if root {
		return ready.Candidates
	}
	return []string{ready.DefinitionURI}
}

// [<] 🤖🤖
