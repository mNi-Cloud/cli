package cli

import (
	"os"

	"github.com/urfave/cli/v3"
)

// NewCommand builds the mni command tree over the terminal.
func NewCommand(version string) *cli.Command {
	return NewCommandFor(version, NewDeps(os.Stdin, os.Stdout, os.Stderr))
}

// NewCommandFor builds the mni command tree over given dependencies.
func NewCommandFor(version string, deps *Deps) *cli.Command {
	return &cli.Command{
		Name:                  "mni",
		Version:               version,
		Usage:                 "CLI client for mNi Cloud",
		EnableShellCompletion: true,
		// urfave/cli hides the completion command it adds, and a command the
		// help never names is one nobody installs.
		ConfigureShellCompletionCommand: func(completion *cli.Command) {
			completion.Hidden = false
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "context",
				Usage:       "Context to use instead of the current one",
				Destination: &deps.ContextName,
				Sources:     cli.EnvVars("MNI_CONTEXT"),
			},
			&cli.StringFlag{
				Name:        "tenant",
				Aliases:     []string{"t"},
				Usage:       "Tenant to address instead of the one of the context",
				Destination: &deps.TenantName,
				Sources:     cli.EnvVars("MNI_TENANT"),
			},
		},
		Commands: []*cli.Command{
			loginCommand(deps),
			logoutCommand(deps),
			whoamiCommand(deps),
			configCommand(deps),
			tenantsCommand(deps),
			apiResourcesCommand(deps),
			explainCommand(deps),
			getCommand(deps),
			describeCommand(deps),
			applyCommand(deps),
			editCommand(deps),
			deleteCommand(deps),
			dependenciesCommand(deps),
			dependentsCommand(deps),
			vmCommand(deps),
		},
	}
}
