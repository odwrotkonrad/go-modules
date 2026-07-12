package cli

// [>] 🤖🤖

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (ld *loader) detectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "detect",
		Short: "print the eligible profiles (comma-joined) and exit",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(ld.local.host.Profile)
			for _, p := range ld.plugins.refs {
				l, ok, err := ld.plugins.ensure(p)
				if err != nil {
					return err
				}
				if ok {
					fmt.Println(l.pluginRef)
				}
			}
			return nil
		},
	}
}

// [<] 🤖🤖
