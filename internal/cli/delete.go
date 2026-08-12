package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/urfave/cli/v3"
)

func deleteCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a resource",
		ArgsUsage: "<resource> <name>",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "resource"},
			&cli.StringArg{Name: "name"},
		},
		Flags:  []cli.Flag{yesFlag()},
		Before: deps.RequireLogin,
		Action: deps.Delete,
	}
}

// Delete removes a resource, and with it whatever depends on it.
func (d *Deps) Delete(ctx context.Context, cmd *cli.Command) error {
	resourceName := cmd.StringArg("resource")
	name := cmd.StringArg("name")
	if resourceName == "" || name == "" {
		return errors.New("mni delete needs a resource and a name (usage: mni delete <resource> <name>)")
	}

	resource, err := d.FindResource(ctx, resourceName)
	if err != nil {
		return err
	}
	resourceClient, err := d.ResourceFor(resource)
	if err != nil {
		return err
	}

	if !cmd.Bool(yesFlagName) {
		allowed, err := d.confirmDelete(ctx, resource, name)
		if err != nil {
			return err
		}
		if !allowed {
			fmt.Fprintf(d.Out, "%s/%s was not deleted.\n", resource.Resource, name)
			return nil
		}
	}

	if err := resourceClient.Delete(ctx, name); err != nil {
		return err
	}

	fmt.Fprintf(d.Out, "%s/%s deleted\n", resource.Resource, name)
	return nil
}

// confirmDelete shows what a delete carries with it and asks whether to go on.
func (d *Deps) confirmDelete(ctx context.Context, resource api.APIResource, name string) (bool, error) {
	if !d.Interactive {
		return false, notATerminal(fmt.Sprintf("whether to delete %s/%s", resource.Resource, name))
	}

	dependents, err := d.Dependents(ctx, resource, name)
	if err != nil {
		return false, err
	}

	if len(dependents) > 0 {
		fmt.Fprintf(d.Out, "Deleting %s/%s deletes what depends on it as well:\n", resource.Resource, name)
		if err := d.Printer().PrintDependencies(dependents); err != nil {
			return false, err
		}
		fmt.Fprintln(d.Out)
	}

	return d.Confirm(fmt.Sprintf("Delete %s/%s?", resource.Resource, name))
}

// notATerminal explains that a run without a terminal has to say up front what
// it allows.
func notATerminal(question string) error {
	return fmt.Errorf("cannot ask %s because %w: pass --%s to delete without being asked", question, errNotATerminal, yesFlagName)
}
