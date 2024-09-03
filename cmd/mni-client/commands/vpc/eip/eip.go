package eip

import (
	"fmt"
	"strconv"

	"github.com/jedib0t/go-pretty/v6/table"
	mni_vpc "github.com/mNi-Cloud/backend/vpc/pkg/client/v1alpha1"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name: "eips",
	Subcommands: []*cli.Command{
		{
			Name:   "list",
			Before: commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				vpcClient, err := commands.NewVpcClient(c)
				if err != nil {
					return err
				}
				res, err := vpcClient.V1Alpha1().GetEipListWithResponse(c.Context, &mni_vpc.GetEipListParams{Authorization: "Bearer " + c.String("token")})
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
					eips := res.JSON200

					displayMultiple(c, eips)

					return nil
				}
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

				vpcClient, err := commands.NewVpcClient(c)
				if err != nil {
					return err
				}
				res, err := vpcClient.V1Alpha1().GetEipWithResponse(c.Context, c.Args().First(), &mni_vpc.GetEipParams{Authorization: "Bearer " + c.String("token")})
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
					eip := res.JSON200

					displaySingle(c, eip)

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
			},
			Before: commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				vpcClient, err := commands.NewVpcClient(c)
				if err != nil {
					return err
				}
				name := c.String("name")
				res, err := vpcClient.V1Alpha1().CreateEipWithResponse(c.Context, &mni_vpc.CreateEipParams{Authorization: "Bearer " + c.String("token")}, mni_vpc.Eip{Name: &name})
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
					eip := res.JSON201

					displaySingle(c, eip)

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

				vpcClient, err := commands.NewVpcClient(c)
				if err != nil {
					return err
				}
				res, err := vpcClient.V1Alpha1().DeleteEipWithResponse(c.Context, c.Args().First(), &mni_vpc.DeleteEipParams{Authorization: "Bearer " + c.String("token")})
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
					return nil
				}
			},
		},
	},
}

func displayMultiple(c *cli.Context, eips *[]mni_vpc.Eip) {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Name", "Address"})

	for _, eip := range *eips {
		name := ""
		address := ""

		if eip.Name != nil {
			name = *eip.Name
		}
		if eip.Address != nil {
			address = *eip.Address
		}

		t.AppendRow(table.Row{name, address})
	}

	t.Render()
}

func displaySingle(c *cli.Context, eip *mni_vpc.Eip) {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Field", "Value"})

	name := ""
	address := ""
	createdAt := ""
	isAssociated := ""

	if eip.Name != nil {
		name = *eip.Name
	}
	if eip.Address != nil {
		address = *eip.Address
	}
	if eip.CreatedAt != nil {
		createdAt = eip.CreatedAt.String()
	}
	if eip.IsAssociated != nil {
		isAssociated = strconv.FormatBool(*eip.IsAssociated)
	}

	t.AppendRow(table.Row{"Name", name})
	t.AppendRow(table.Row{"Address", address})
	t.AppendRow(table.Row{"CreatedAt", createdAt})
	t.AppendRow(table.Row{"IsAssociated", isAssociated})

	t.Render()
}
