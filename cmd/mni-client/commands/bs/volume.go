package bs

import (
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mNi-Cloud/backend/bs/pkg/client/v1alpha1"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands"
	"github.com/urfave/cli/v2"
)

var VolumeCommand = &cli.Command{
	Name: "volumes",
	Subcommands: []*cli.Command{
		{
			Name:   "list",
			Before: commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				bsClient, err := commands.NewBsClient(c)
				if err != nil {
					return err
				}

				res, err := bsClient.V1Alpha1().GetVolumeListWithResponse(c.Context, &v1alpha1.GetVolumeListParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				var subnets *[]v1alpha1.Volume

				if res.StatusCode() != 200 {
					if res.JSONDefault != nil {
						return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
					} else {
						return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), string(res.Body)), 1)
					}

				} else {
					subnets = res.JSON200
				}

				displayMultipleVolume(c, subnets)

				return nil
			},
		},
		{
			Name:      "get",
			ArgsUsage: "<name>",
			Before:    commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				if c.NArg() != 1 {
					cli.ShowSubcommandHelpAndExit(c, 1)
					return nil
				}

				bsClient, err := commands.NewBsClient(c)
				if err != nil {
					return err
				}
				res, err := bsClient.V1Alpha1().GetVolumeWithResponse(c.Context, c.Args().First(), &v1alpha1.GetVolumeParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 200 {
					if res.JSONDefault != nil {
						return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
					} else {
						return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), string(res.Body)), 1)
					}

				} else {
					displaySingleVolume(c, res.JSON200)

					return nil
				}
			},
		},
		{
			Name: "create",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "name",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "size",
					Required: true,
				},
			},
			Before: commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				bsClient, err := commands.NewBsClient(c)
				if err != nil {
					return err
				}

				name := c.String("name")
				size := c.String("size")

				res, err := bsClient.V1Alpha1().CreateVolumeWithResponse(c.Context, &v1alpha1.CreateVolumeParams{
					Authorization: "Bearer " + c.String("token"),
				}, v1alpha1.Volume{
					Name: &name,
					Size: &size,
				})
				if err != nil {
					return err
				}

				if res.StatusCode() != 201 {
					if res.JSONDefault != nil {
						return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
					} else {
						return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), string(res.Body)), 1)
					}

				} else {
					displaySingleVolume(c, res.JSON201)

					return nil
				}
			},
		},
		{
			Name:      "delete",
			ArgsUsage: "<name>",
			Before:    commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				if c.NArg() != 1 {
					cli.ShowSubcommandHelpAndExit(c, 1)
					return nil
				}

				bsClient, err := commands.NewBsClient(c)
				if err != nil {
					return err
				}

				res, err := bsClient.V1Alpha1().DeleteVolumeWithResponse(c.Context, c.Args().First(), &v1alpha1.DeleteVolumeParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 204 {
					if res.JSONDefault != nil {
						return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
					} else {
						return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), string(res.Body)), 1)
					}

				} else {
					fmt.Println("Volume deleted successfully")
					return nil
				}
			},
		},
	},
}

func displayMultipleVolume(c *cli.Context, subnets *[]v1alpha1.Volume) {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Name", "Size", "Status"})
	for _, subnet := range *subnets {
		name := ""
		size := ""
		status := ""

		if subnet.Name != nil {
			name = *subnet.Name
		}
		if subnet.Size != nil {
			size = *subnet.Size
		}
		if subnet.Status != nil {
			status = *subnet.Status
		}

		t.AppendRow(table.Row{name, size, status})
	}

	t.Render()
}

func displaySingleVolume(c *cli.Context, subnet *v1alpha1.Volume) {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Field", "Value"})

	name := ""
	size := ""
	status := ""
	createdAt := ""

	if subnet.Name != nil {
		name = *subnet.Name
	}

	if subnet.Size != nil {
		size = *subnet.Size
	}

	if subnet.Status != nil {
		status = *subnet.Status
	}

	if subnet.CreatedAt != nil {
		createdAt = subnet.CreatedAt.String()
	}

	t.AppendRow(table.Row{"Name", name})
	t.AppendRow(table.Row{"Size", size})
	t.AppendRow(table.Row{"Status", status})
	t.AppendRow(table.Row{"Created At", createdAt})

	t.Render()
}
