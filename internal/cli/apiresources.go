package cli

import (
	"context"
	"strings"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/output"
	"github.com/urfave/cli/v3"
)

// noAlternateNames fills the cell of a resource that only answers to its plural
// name.
const noAlternateNames = "-"

func apiResourcesCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:   "api-resources",
		Usage:  "List the resources this server serves",
		Action: deps.APIResources,
	}
}

// APIResources prints the resource catalog of the server.
func (d *Deps) APIResources(ctx context.Context, _ *cli.Command) error {
	apiClient, err := d.Client()
	if err != nil {
		return err
	}

	catalog, err := apiClient.APIResources(ctx)
	if err != nil {
		return err
	}

	table := output.NewWriter(d.Out)
	table.WriteHeader("Group", "Version", "Resource", "Aliases")
	for _, resource := range catalog {
		table.WriteRow(resource.Group, resource.Version, resource.Resource, aliasCell(resource))
	}
	return table.Flush()
}

func aliasCell(resource api.APIResource) string {
	names := resource.AlternateNames()
	if len(names) == 0 {
		return noAlternateNames
	}
	return strings.Join(names, ",")
}
