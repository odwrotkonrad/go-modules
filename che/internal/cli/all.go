package cli

// [>] 🤖🤖

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

// allCmd runs every step op in load order, each gated on whether the resolved
// selection defines anything for it. A failing step does not stop the rest:
// step errors collect and join, matching forEachRepoUnit semantics.
func (c *CheApp) allCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "all",
		Short: "run every op the profile selects, in order",
		RunE: func(cmd *cobra.Command, args []string) error {
			var fails []error
			for _, s := range steps() {
				name := s.displayName()
				// [why] plugin selections build lazily inside forEachRepoUnit;
				// skipping on the local selection alone would silently drop
				// plugin content.
				if len(c.pluginRefs) == 0 && !s.selected(c.units[0].res) {
					log.Debug("all(skip)", name+" (nothing selected)", log.Off)
					continue
				}
				log.Msg("all(run)", name, log.Off)
				if err := c.forEachRepoUnit(s.name, func(u repoUnit) error { return s.op(c, u) }); err != nil {
					fails = append(fails, fmt.Errorf("%s: %w", name, err))
				}
			}
			return errors.Join(fails...)
		},
	}
}

// [<] 🤖🤖
