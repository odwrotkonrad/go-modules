package cli

// [>] 🤖🤖

import (
	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go/che/internal/host"
)

var ServicesCmd = &cobra.Command{
	Use:   "services",
	Short: "load/unload/verify the profile's launchd services",
}

func init() {
	ServicesCmd.AddCommand(
		serviceCmd("bootout", "unload each service (bootout if loaded, wait until gone)", host.Host.Bootout),
		serviceCmd("bootin", "load each service (bootstrap from plist)", host.Host.Bootin),
		serviceCmd("ensure", "settle then verify each long-running service has a live pid", host.Host.Ensure),
	)
}

// serviceCmd builds a launchd subcommand: resolve profile services, run action over them.
func serviceCmd(use, short string, action func(host.Host, []host.Service) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, u := range units {
				err := u.withEnv(func() error {
					svcs, err := u.host.ResolveServices(u.res.Services)
					if err != nil {
						return err
					}
					return action(u.host, svcs)
				})
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// [<] 🤖🤖
