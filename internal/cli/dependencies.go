package cli

import (
	"context"
	"errors"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/output"
	"github.com/mNi-Cloud/cli/internal/unstructured"
	"github.com/urfave/cli/v3"
)

// directFlagName limits what depends on a resource to the resources that name
// it themselves.
const directFlagName = "direct"

func dependenciesCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:      "dependencies",
		Usage:     "List what a resource needs",
		ArgsUsage: "<resource> <name>",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "resource"},
			&cli.StringArg{Name: "name"},
		},
		Flags:  []cli.Flag{outputFlag()},
		Before: deps.RequireLogin,
		Action: deps.ListDependencies,
	}
}

func dependentsCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:      "dependents",
		Usage:     "List what a resource is needed by, and what deleting it would carry with it",
		ArgsUsage: "<resource> <name>",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "resource"},
			&cli.StringArg{Name: "name"},
		},
		Flags: []cli.Flag{
			outputFlag(),
			&cli.BoolFlag{
				Name:  directFlagName,
				Usage: "List only what names the resource itself, without following the chain",
			},
		},
		Before: deps.RequireLogin,
		Action: deps.ListDependents,
	}
}

// ListDependencies prints what a resource needs. The server names what the
// resource depends on directly; nothing follows from it, because mNi Cloud
// carries a delete the other way round.
func (d *Deps) ListDependencies(ctx context.Context, cmd *cli.Command) error {
	resource, name, err := dependencyTarget(cmd, "dependencies")
	if err != nil {
		return err
	}

	found, err := d.FindResource(ctx, resource)
	if err != nil {
		return err
	}
	resourceClient, err := d.ResourceFor(found)
	if err != nil {
		return err
	}

	dependencies, err := resourceClient.Dependencies(ctx, name)
	if err != nil {
		return err
	}
	return d.printDependencies(cmd, dependencies)
}

// ListDependents prints what depends on a resource. It follows the chain by
// default, because that is what a delete of the resource removes, and stops at
// the resources that name it themselves when --direct is given.
func (d *Deps) ListDependents(ctx context.Context, cmd *cli.Command) error {
	resource, name, err := dependencyTarget(cmd, "dependents")
	if err != nil {
		return err
	}

	found, err := d.FindResource(ctx, resource)
	if err != nil {
		return err
	}

	dependents, err := d.dependentsOf(ctx, found, name, cmd.Bool(directFlagName))
	if err != nil {
		return err
	}
	return d.printDependencies(cmd, dependents)
}

func (d *Deps) dependentsOf(ctx context.Context, resource api.APIResource, name string, direct bool) ([]api.Dependency, error) {
	if !direct {
		return d.Dependents(ctx, resource, name)
	}

	resourceClient, err := d.ResourceFor(resource)
	if err != nil {
		return nil, err
	}
	return resourceClient.Dependents(ctx, name)
}

func (d *Deps) printDependencies(cmd *cli.Command, dependencies []api.Dependency) error {
	format := cmd.String(outputFlagName)
	if format == output.FormatTable {
		return d.Printer().PrintDependencies(dependencies)
	}

	list, err := unstructured.ListFrom(dependencies)
	if err != nil {
		return err
	}
	return d.Printer().Print(list, format)
}

func dependencyTarget(cmd *cli.Command, command string) (string, string, error) {
	resource := cmd.StringArg("resource")
	name := cmd.StringArg("name")
	if resource == "" || name == "" {
		return "", "", errors.New("mni " + command + " needs a resource and a name (usage: mni " + command + " <resource> <name>)")
	}
	return resource, name, nil
}
