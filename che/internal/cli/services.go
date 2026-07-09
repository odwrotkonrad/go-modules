package cli

// [>] 🤖🤖

import (
	"gitlab.com/konradodwrot/go-modules/che/internal/host"
)

// serviceOp builds a launchd step op: resolve profile services, run action over them.
func serviceOp(action func(host.Host, []host.Service) error) func(*CheApp, unit) error {
	return func(_ *CheApp, u unit) error {
		svcs, err := u.host.ResolveServices(u.res.Services)
		if err != nil {
			return err
		}
		return action(u.host, svcs)
	}
}

// [<] 🤖🤖
