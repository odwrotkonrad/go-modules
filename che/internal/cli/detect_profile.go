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
		fmt.Println(theHost.Profile)
		return nil
	},
}

// [<] 🤖🤖
