package cli

// [>] 🤖🤖

import (
	"fmt"

	"github.com/spf13/cobra"
)

var DetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "print the eligible profiles (comma-joined) and exit",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(units[0].host.Profile)
		for _, p := range pluginRefs {
			u, ok, err := ensurePlugin(p)
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

// [<] 🤖🤖
