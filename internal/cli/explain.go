package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/output"
	"github.com/urfave/cli/v3"
)

const (
	// statusFlagName reads the schema of what the server reports instead of the
	// one of what a manifest writes.
	statusFlagName = "status"
	// recursiveFlagName writes every field under the one asked about.
	recursiveFlagName = "recursive"
)

func explainCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:      "explain",
		Usage:     "Show what a manifest may hold for a resource",
		ArgsUsage: "<resource>[.<field>...]",
		Arguments: []cli.Argument{&cli.StringArg{Name: "target"}},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  statusFlagName,
				Usage: "Explain what the server reports back instead of what a manifest writes",
			},
			&cli.BoolFlag{
				Name:    recursiveFlagName,
				Aliases: []string{"r"},
				Usage:   "Write every field under the one asked about",
			},
		},
		Action: deps.Explain,
	}
}

// Explain writes the schema api-gateway publishes for one resource, or for one
// field of it.
func (d *Deps) Explain(ctx context.Context, cmd *cli.Command) error {
	target := cmd.StringArg("target")
	if target == "" {
		return errors.New("mni explain needs a resource (usage: mni explain <resource>[.<field>...])")
	}

	resourceName, fields := splitExplainTarget(target)
	resource, err := d.FindResource(ctx, resourceName)
	if err != nil {
		return err
	}

	section := api.SpecSchemaSection
	if cmd.Bool(statusFlagName) {
		section = api.StatusSchemaSection
	}

	root, err := resource.Schema(section)
	if err != nil {
		return err
	}
	schema, err := root.Resolve(fields)
	if err != nil {
		return fmt.Errorf("cannot explain the %s of %s: %w", section, resource.Resource, err)
	}

	return d.Printer().PrintExplanation(output.Explanation{
		Kind:       resource.Kind,
		APIVersion: resource.APIVersion(),
		Path:       strings.Join(append([]string{string(section)}, fields...), "."),
		Schema:     schema,
		Recursive:  cmd.Bool(recursiveFlagName),
	})
}

// splitExplainTarget takes the resource off the front of what the user typed.
// A resource is never named with a dot in it, so everything behind the first
// one names a field.
func splitExplainTarget(target string) (string, []string) {
	parts := strings.Split(target, ".")
	return parts[0], parts[1:]
}
