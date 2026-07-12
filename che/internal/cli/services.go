package cli

// [>] 🤖🤖

import (
	"gitlab.com/konradodwrot/go-modules/che/internal/host"
)

// serviceOp builds a launchd step op: resolve profile services, run action over them.
func serviceOp(action func(host.Host, []host.Service) error) func(*loader, load) error {
	return func(_ *loader, l load) error {
		svcs, err := l.host.ResolveServices(l.selection.Services)
		if err != nil {
			return err
		}
		return action(l.host, svcs)
	}
}

// [<] 🤖🤖
