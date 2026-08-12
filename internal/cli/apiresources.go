package cli

import (
	"context"

	"github.com/mNi-Cloud/cli/internal/output"
	"github.com/urfave/cli/v3"
)

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
	table.WriteHeader("Group", "Version", "Resource")
	for _, resource := range catalog {
		table.WriteRow(resource.Group, resource.Version, resource.Resource)
	}
	return table.Flush()
}
