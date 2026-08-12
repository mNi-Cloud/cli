package cli

import (
	"context"
	"errors"

	"github.com/mNi-Cloud/cli/internal/output"
	"github.com/urfave/cli/v3"
)

func describeCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:      "describe",
		Usage:     "Show the state of a resource for a person to read",
		ArgsUsage: "<resource> <name>",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "resource"},
			&cli.StringArg{Name: "name"},
		},
		Before: deps.RequireLogin,
		Action: deps.Describe,
	}
}

// Describe reads one resource together with both sides of its dependency
// graph, and writes them the way a person reads them rather than the way the
// server sends them.
func (d *Deps) Describe(ctx context.Context, cmd *cli.Command) error {
	resourceName := cmd.StringArg("resource")
	name := cmd.StringArg("name")
	if resourceName == "" || name == "" {
		return errors.New("mni describe needs a resource and a name (usage: mni describe <resource> <name>)")
	}

	resource, err := d.FindResource(ctx, resourceName)
	if err != nil {
		return err
	}
	resourceClient, err := d.ResourceFor(resource)
	if err != nil {
		return err
	}

	object, err := resourceClient.Get(ctx, name)
	if err != nil {
		return err
	}
	dependencies, err := resourceClient.Dependencies(ctx, name)
	if err != nil {
		return err
	}
	dependents, err := d.Dependents(ctx, resource, name)
	if err != nil {
		return err
	}

	return d.Printer().PrintDescription(output.Description{
		Object:       object,
		Dependencies: dependencies,
		Dependents:   dependents,
	})
}
