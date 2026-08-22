package che

// [>] 🤖🤖

import (
	"fmt"
	"path/filepath"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

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
	w := &initWalker{ctx: ctx, home: home, opts: opts, seen: map[string]bool{}}
	return w.walkSpec(spec.SpecSourceRecipe{}, repoRoot, "spec", nil)
}

type initWalker struct {
	ctx  Context
	home string
	opts options.Options
	seen map[string]bool
}

func (w *initWalker) walkSpec(src spec.SpecSourceRecipe, anchor, name string, refEnv map[string]string) error {
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
	doc, err := spec.Load(ready.DefinitionURI, w.ctx.interp(refEnv, w.opts.EnvUnset))
	if err != nil {
		return fmt.Errorf("init-remote-sources %s: %w", name, err)
	}
	for _, inc := range doc.Include {
		if err := w.walkSpec(inc, ready.DirectoryPath, "spec", nil); err != nil {
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
			err := w.walkSpec(
				spec.SpecSourceRecipe{SourceRecipe: spec.SourceRecipe{URI: ref.URI, SpecFile: ref.SpecFile, Ref: ref.Ref}},
				ready.DirectoryPath, ref.ProfileName, ref.Env)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// [<] 🤖🤖
