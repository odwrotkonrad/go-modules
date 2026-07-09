package cli

// [>] 🤖🤖

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

// allStep is one op of the full load: the subcommand to invoke and a
// predicate over the local repo's resolved selection gating it.
type allStep struct {
	name     string
	cmd      *cobra.Command
	selected func(spec.Resolved) bool
}

// allSteps lists every op in load order.
func allSteps() []allStep {
	svc := func(r spec.Resolved) bool { return len(r.Services) > 0 }
	return []allStep{
		{"prune-links", PruneCmd, func(r spec.Resolved) bool { return len(r.Dirs) > 0 }},
		{"mk-dirs", DirsCmd, func(r spec.Resolved) bool { return len(r.Dirs)+len(r.ExtraDirs) > 0 }},
		{"link", LinkCmd, func(r spec.Resolved) bool { return len(r.Links) > 0 }},
		{"copy", CopyCmd, func(r spec.Resolved) bool { return len(r.Copies) > 0 }},
		{"render-templates", RenderCmd, func(r spec.Resolved) bool { return len(r.Templates) > 0 }},
		{"run-scripts", RunScriptsCmd, func(r spec.Resolved) bool { return len(r.Scripts) > 0 }},
		{"services bootout", BootoutCmd, svc},
		{"services bootin", BootinCmd, svc},
		{"services ensure", EnsureCmd, svc},
	}
}

// AllCmd runs every op subcommand in load order, each gated on whether the
// resolved selection defines anything for it. A failing step does not stop
// the rest: step errors collect and join, matching forEachUnit semantics.
var AllCmd = &cobra.Command{
	Use:   "all",
	Short: "run every op the profile selects, in order",
	RunE: func(cmd *cobra.Command, args []string) error {
		var fails []error
		for _, s := range allSteps() {
			// [why] plugin selections build lazily inside forEachUnit;
			// skipping on the local selection alone would silently drop
			// plugin content.
			if len(pluginRefs) == 0 && !s.selected(units[0].res) {
				log.Debug("all(skip)", s.name+" (nothing selected)", log.Off)
				continue
			}
			log.Msg("all(run)", s.name, log.Off)
			if err := s.cmd.RunE(s.cmd, nil); err != nil {
				fails = append(fails, fmt.Errorf("%s: %w", s.name, err))
			}
		}
		return errors.Join(fails...)
	},
}

// [<] 🤖🤖
