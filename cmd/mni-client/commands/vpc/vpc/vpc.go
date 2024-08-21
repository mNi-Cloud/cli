package vpc

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands"
	mni_vpc "github.com/mNi-Cloud/backend/vpc/pkg/client/v1alpha1"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name: "vpcs",
	Subcommands: []*cli.Command{
		{
			Name: "list",
			Action: func(c *cli.Context) error {
				vpcClient, err := commands.NewVpcClient(c)
				if err != nil {
					return err
				}
				res, err := vpcClient.V1Alpha1().GetVpcListWithResponse(c.Context, &mni_vpc.GetVpcListParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 200 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					vpcs := res.JSON200

					displayMultiple(c, vpcs)

					return nil
				}
			},
		},
		{
			Name:      "get",
			ArgsUsage: "<name>",
			Action: func(c *cli.Context) error {
				if c.NArg() != 1 {
					cli.ShowSubcommandHelpAndExit(c, 1)
					return nil
				}

				vpcClient, err := commands.NewVpcClient(c)
				if err != nil {
					return err
				}
				res, err := vpcClient.V1Alpha1().GetVpcWithResponse(c.Context, c.Args().First(), &mni_vpc.GetVpcParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 200 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					vpc := res.JSON200

					displaySingle(c, vpc)

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
			Action: func(c *cli.Context) error {
				vpcClient, err := commands.NewVpcClient(c)
				if err != nil {
					return err
				}

				name := c.String("name")
				res, err := vpcClient.V1Alpha1().CreateVpcWithResponse(c.Context, &mni_vpc.CreateVpcParams{Authorization: "Bearer " + c.String("token")}, mni_vpc.Vpc{Name: &name})
				if err != nil {
					return err
				}

				if res.StatusCode() != 201 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					vpc := res.JSON201

					displaySingle(c, vpc)

					return nil
				}
			},
		},
		{
			Name:      "delete",
			ArgsUsage: "<name>",
			Action: func(c *cli.Context) error {
				if c.NArg() != 1 {
					cli.ShowSubcommandHelpAndExit(c, 1)
					return nil
				}

				vpcClient, err := commands.NewVpcClient(c)
				if err != nil {
					return err
				}
				res, err := vpcClient.V1Alpha1().DeleteVpcWithResponse(c.Context, c.Args().First(), &mni_vpc.DeleteVpcParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 204 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					fmt.Println("VPC deleted successfully")
					return nil
				}
			},
		},
	},
}

func displaySingle(c *cli.Context, vpc *mni_vpc.Vpc) {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Field", "Value"})

	name := ""
	subnets := []string{}
	status := ""
	createdAt := ""

	if vpc.Name != nil {
		name = *vpc.Name
	}
	if vpc.Subnets != nil {
		subnets = *vpc.Subnets
	}
	if vpc.Status != nil {
		status = *vpc.Status
	}
	if vpc.CreatedAt != nil {
		createdAt = vpc.CreatedAt.String()
	}

	t.AppendRow(table.Row{"Name", name})
	t.AppendRow(table.Row{"Subnets", strings.Join(subnets, ", ")})
	t.AppendRow(table.Row{"Status", status})
	t.AppendRow(table.Row{"CreatedAt", createdAt})

	t.Render()
}

func displayMultiple(c *cli.Context, vpcs *[]mni_vpc.Vpc) {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Name", "Subnets", "Status"})

	for _, vpc := range *vpcs {

		name := ""
		subnets := []string{}
		status := ""

		if vpc.Name != nil {
			name = *vpc.Name
		}
		if vpc.Subnets != nil {
			subnets = *vpc.Subnets
		}
		if vpc.Status != nil {
			status = *vpc.Status
		}

		t.AppendRow(table.Row{name, strings.Join(subnets, ", "), status})
	}

	t.Render()
}
