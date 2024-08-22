package subnet

import (
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	mni_vpc "github.com/mNi-Cloud/backend/vpc/pkg/client/v1alpha1"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name: "subnets",
	Subcommands: []*cli.Command{
		{
			Name:   "list",
			Before: commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				vpcClient, err := commands.NewVpcClient(c)
				if err != nil {
					return err
				}

				var subnets *[]mni_vpc.Subnet

				res, err := vpcClient.V1Alpha1().GetAllSubnetListWithResponse(c.Context, &mni_vpc.GetAllSubnetListParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}
				if res.StatusCode() != 200 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					subnets = res.JSON200
					displayMultiple(c, subnets)
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
				res, err := vpcClient.V1Alpha1().GetSubnetWithResponse(c.Context, c.Args().First(), &mni_vpc.GetSubnetParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 200 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					subnet := res.JSON200

					displaySingle(c, subnet)

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
					Name:     "vpc",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "protocol",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "cidr",
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
				vpc := c.String("vpc")
				protocol := c.String("protocol")
				cidr := c.String("cidr")
				res, err := vpcClient.V1Alpha1().CreateSubnetWithResponse(c.Context, &mni_vpc.CreateSubnetParams{Authorization: "Bearer " + c.String("token")}, mni_vpc.Subnet{Name: &name, Vpc: &vpc, Protocol: (*mni_vpc.SubnetProtocol)(&protocol), CidrBlock: &cidr})
				if err != nil {
					return err
				}

				if res.StatusCode() != 201 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					subnet := res.JSON201

					displaySingle(c, subnet)

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
				res, err := vpcClient.V1Alpha1().DeleteSubnetWithResponse(c.Context, c.Args().First(), &mni_vpc.DeleteSubnetParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 204 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					fmt.Println("Subnet deleted successfully")
					return nil
				}
			},
		},
	},
}

func displaySingle(c *cli.Context, subnet *mni_vpc.Subnet) {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Field", "Value"})

	name := ""
	protocol := ""
	cidrBlock := ""
	vpc := ""
	createdAt := ""

	if subnet.Name != nil {
		name = *subnet.Name
	}
	if subnet.Protocol != nil {
		protocol = *(*string)(subnet.Protocol)
	}
	if subnet.CidrBlock != nil {
		cidrBlock = *subnet.CidrBlock
	}
	if subnet.Vpc != nil {
		vpc = *subnet.Vpc
	}
	if subnet.CreatedAt != nil {
		createdAt = subnet.CreatedAt.String()
	}

	t.AppendRow(table.Row{"Name", name})
	t.AppendRow(table.Row{"Protocol", protocol})
	t.AppendRow(table.Row{"CidrBlock", cidrBlock})
	t.AppendRow(table.Row{"Vpc", vpc})
	t.AppendRow(table.Row{"CreatedAt", createdAt})

	t.Render()
}

func displayMultiple(c *cli.Context, subnets *[]mni_vpc.Subnet) {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Name", "Protocol", "CidrBlock", "Vpc"})

	for _, subnet := range *subnets {
		name := ""
		protocol := ""
		cidrBlock := ""
		vpc := ""

		if subnet.Name != nil {
			name = *subnet.Name
		}
		if subnet.Protocol != nil {
			protocol = *(*string)(subnet.Protocol)
		}
		if subnet.CidrBlock != nil {
			cidrBlock = *subnet.CidrBlock
		}
		if subnet.Vpc != nil {
			vpc = *subnet.Vpc
		}

		t.AppendRow(table.Row{name, protocol, cidrBlock, vpc})
	}

	t.Render()
}
