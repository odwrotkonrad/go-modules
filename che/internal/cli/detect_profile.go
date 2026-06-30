package cli

// [>] 🤖🤖

import (
	"fmt"

	"github.com/spf13/cobra"
)

var DetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "print the detected profile and exit",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(theHost.Profile)
		return nil
	},
}

// [<] 🤖🤖
