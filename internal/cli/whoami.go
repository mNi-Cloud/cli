package cli

import (
	"context"
	"fmt"

	"github.com/mNi-Cloud/cli/internal/output"
	"github.com/urfave/cli/v3"
)

func whoamiCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:   "whoami",
		Usage:  "Show who the current context is signed in as",
		Before: deps.RequireLogin,
		Action: deps.Whoami,
	}
}

// Whoami asks the server who the stored token belongs to.
func (d *Deps) Whoami(ctx context.Context, _ *cli.Command) error {
	target, err := d.Context()
	if err != nil {
		return err
	}
	apiClient, err := d.Client()
	if err != nil {
		return err
	}

	identity, err := apiClient.Me(ctx)
	if err != nil {
		return err
	}

	tenant := "(not set)"
	if resolved, err := d.Tenant(); err == nil {
		tenant = resolved
	}

	table := output.NewWriter(d.Out)
	table.WriteHeader("Context", "Server", "User", "User ID", "Tenant")
	table.WriteRow(target.Name, target.Server, identity.Username, identity.UserID, tenant)
	if err := table.Flush(); err != nil {
		return err
	}

	if len(identity.Scopes) > 0 {
		fmt.Fprintf(d.Out, "\nScopes: %v\n", identity.Scopes)
	}
	return nil
}
