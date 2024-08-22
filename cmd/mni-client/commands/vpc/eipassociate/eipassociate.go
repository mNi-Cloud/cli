package eipassociate

import (
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	mni_vpc "github.com/mNi-Cloud/backend/vpc/pkg/client/v1alpha1"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name: "eipassociates",
	Subcommands: []*cli.Command{
		{
			Name:   "list",
			Before: commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				vpcClient, err := commands.NewVpcClient(c)
				if err != nil {
					return err
				}
				res, err := vpcClient.V1Alpha1().GetEipAssociateListWithResponse(c.Context, &mni_vpc.GetEipAssociateListParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 200 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					eipAssociates := res.JSON200

					displayMultiple(c, eipAssociates)

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
				res, err := vpcClient.V1Alpha1().GetEipAssociateWithResponse(c.Context, c.Args().First(), &mni_vpc.GetEipAssociateParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 200 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					eipAssociate := res.JSON200

					displaySingle(c, eipAssociate)

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
					Name:     "eip",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "target",
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
				eip := c.String("eip")
				target := c.String("target")
				res, err := vpcClient.V1Alpha1().CreateEipAssociateWithResponse(c.Context, &mni_vpc.CreateEipAssociateParams{Authorization: "Bearer " + c.String("token")}, mni_vpc.EipAssociate{Name: &name, Eip: &eip, Target: &target})
				if err != nil {
					return err
				}

				if res.StatusCode() != 201 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					eipAssociate := res.JSON201

					displaySingle(c, eipAssociate)

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
				res, err := vpcClient.V1Alpha1().DeleteEipAssociateWithResponse(c.Context, c.Args().First(), &mni_vpc.DeleteEipAssociateParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 204 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					return nil
				}
			},
		},
	},
}

func displayMultiple(c *cli.Context, eipAssociates *[]mni_vpc.EipAssociate) {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Name", "Eip", "Target"})

	for _, eipAssociate := range *eipAssociates {
		name := ""
		eip := ""
		target := ""

		if eipAssociate.Name != nil {
			name = *eipAssociate.Name
		}
		if eipAssociate.Eip != nil {
			eip = *eipAssociate.Eip
		}
		if eipAssociate.Target != nil {
			target = *eipAssociate.Target
		}

		t.AppendRow(table.Row{name, eip, target})
	}

	t.Render()
}

func displaySingle(c *cli.Context, eipAssociate *mni_vpc.EipAssociate) {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Field", "Value"})

	name := ""
	eip := ""
	target := ""
	createdAt := ""

	if eipAssociate.Name != nil {
		name = *eipAssociate.Name
	}
	if eipAssociate.Eip != nil {
		eip = *eipAssociate.Eip
	}
	if eipAssociate.Target != nil {
		target = *eipAssociate.Target
	}
	if eipAssociate.CreatedAt != nil {
		createdAt = eipAssociate.CreatedAt.String()
	}

	t.AppendRow(table.Row{"Name", name})
	t.AppendRow(table.Row{"Eip", eip})
	t.AppendRow(table.Row{"Target", target})
	t.AppendRow(table.Row{"CreatedAt", createdAt})

	t.Render()
}
