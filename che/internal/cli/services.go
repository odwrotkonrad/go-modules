package cli

// [>] 🤖🤖

import (
	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go-modules/che/internal/host"
)

var ServicesCmd = &cobra.Command{
	Use:   "services",
	Short: "load/unload/verify the profile's launchd services",
}

var (
	BootoutCmd = serviceCmd("bootout", "unload each service (bootout if loaded, wait until gone)", host.Host.Bootout)
	BootinCmd  = serviceCmd("bootin", "load each service (bootstrap from plist)", host.Host.Bootin)
	EnsureCmd  = serviceCmd("ensure", "settle then verify each long-running service has a live pid", host.Host.Ensure)
)

func init() {
	ServicesCmd.AddCommand(BootoutCmd, BootinCmd, EnsureCmd)
}

// serviceCmd builds a launchd subcommand: resolve profile services, run action over them.
func serviceCmd(use, short string, action func(host.Host, []host.Service) error) *cobra.Command {
	return unitCmd(use, short, func(u unit) error {
		svcs, err := u.host.ResolveServices(u.res.Services)
		if err != nil {
			return err
		}
		return action(u.host, svcs)
	})
}

// [<] 🤖🤖
