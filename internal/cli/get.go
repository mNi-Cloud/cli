package cli

import (
	"context"
	"errors"

	"github.com/mNi-Cloud/cli/internal/output"
	"github.com/mNi-Cloud/cli/internal/unstructured"
	"github.com/urfave/cli/v3"
)

func getCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get a resource",
		ArgsUsage: "<resource> [name]",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "resource"},
			&cli.StringArg{Name: "name"},
		},
		Flags:  []cli.Flag{outputFlag()},
		Before: deps.RequireLogin,
		Action: deps.Get,
	}
}

// Get reads one resource, or all of them when no name is given.
func (d *Deps) Get(ctx context.Context, cmd *cli.Command) error {
	resourceName := cmd.StringArg("resource")
	if resourceName == "" {
		return errors.New("mni get needs a resource (usage: mni get <resource> [name])")
	}

	resource, err := d.FindResource(ctx, resourceName)
	if err != nil {
		return err
	}
	resourceClient, err := d.ResourceFor(resource)
	if err != nil {
		return err
	}

	format := cmd.String(outputFlagName)
	printer := d.Printer()

	name := cmd.StringArg("name")
	if name == "" {
		list, err := resourceClient.List(ctx)
		if err != nil {
			return err
		}
		if format == output.FormatTable {
			return printer.PrintTable(resource, list)
		}
		return printer.Print(list, format)
	}

	object, err := resourceClient.Get(ctx, name)
	if err != nil {
		return err
	}
	if format == output.FormatTable {
		return printer.PrintTable(resource, []unstructured.Unstructured{object})
	}
	return printer.Print(object, format)
}
