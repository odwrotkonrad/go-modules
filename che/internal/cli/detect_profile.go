package cli

// [>] 🤖🤖

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (c *CheApp) detectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "detect",
		Short: "print the eligible profiles (comma-joined) and exit",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(c.units[0].host.Profile)
			for _, p := range c.pluginRefs {
				u, ok, err := c.ensurePlugin(p)
				if err != nil {
					return err
				}
				if ok {
					fmt.Println(u.ref)
				}
			}
			return nil
		},
	}
}

// [<] 🤖🤖
