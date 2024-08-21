package container

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands"
	mni_ctr "github.com/mNi-Cloud/backend/ctr/pkg/client/v1alpha1"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name: "containers",
	Subcommands: []*cli.Command{
		{
			Name: "list",
			Action: func(c *cli.Context) error {
				ctrClient, err := commands.NewCtrClient(c)
				if err != nil {
					return err
				}

				res, err := ctrClient.V1Alpha1().GetContainerPoolListWithResponse(c.Context, &mni_ctr.GetContainerPoolListParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 200 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					containers := res.JSON200
					return displayMultiple(c, *containers)
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

				ctrClient, err := commands.NewCtrClient(c)
				if err != nil {
					return err
				}

				res, err := ctrClient.V1Alpha1().GetContainerPoolByNameWithResponse(c.Context, c.Args().First(), &mni_ctr.GetContainerPoolByNameParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 200 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					container := res.JSON200
					return displaySingle(c, *container)
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
					Name:     "subnet",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "image",
					Required: true,
				},
			},
			Action: func(c *cli.Context) error {
				ctrClient, err := commands.NewCtrClient(c)
				if err != nil {
					return err
				}

				name := c.String("name")
				vpc := c.String("vpc")
				subnet := c.String("subnet")
				image := c.String("image")

				res, err := ctrClient.V1Alpha1().CreateContainerPoolWithResponse(c.Context, &mni_ctr.CreateContainerPoolParams{Authorization: "Bearer " + c.String("token")}, mni_ctr.ContainerPool{
					Name:   &name,
					Vpc:    &vpc,
					Subnet: &subnet,
					Image:  &image,
				})
				if err != nil {
					return err
				}

				if res.StatusCode() != 201 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					container := res.JSON201
					return displaySingle(c, *container)
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

				ctrClient, err := commands.NewCtrClient(c)
				if err != nil {
					return err
				}

				res, err := ctrClient.V1Alpha1().DeleteContainerPoolByNameWithResponse(c.Context, c.Args().First(), &mni_ctr.DeleteContainerPoolByNameParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 204 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					fmt.Println("VM deleted successfully")
					return nil
				}
			},
		},
	},
}

func displaySingle(c *cli.Context, container mni_ctr.ContainerPool) error {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Field", "Value"})

	name := ""
	vpc := ""
	subnet := ""
	image := ""
	replicas := ""
	createdAt := ""

	if container.Name != nil {
		name = *container.Name
	}

	if container.Vpc != nil {
		vpc = *container.Vpc
	}

	if container.Subnet != nil {
		subnet = *container.Subnet
	}

	if container.Image != nil {
		image = *container.Image
	}

	if container.Replicas != nil {
		replicas = strconv.Itoa(*container.Replicas)
	}

	if container.CreatedAt != nil {
		createdAt = container.CreatedAt.String()
	}

	t.AppendRow(table.Row{"Name", name})
	t.AppendRow(table.Row{"Vpc", vpc})
	t.AppendRow(table.Row{"Subnet", subnet})
	t.AppendRow(table.Row{"Image", image})
	t.AppendRow(table.Row{"Replicas", replicas})
	t.AppendRow(table.Row{"CreatedAt", createdAt})

	if container.Instances != nil && len(*container.Instances) > 0 {
		ctrClient, err := commands.NewCtrClient(c)
		if err != nil {
			return err
		}
		for _, instanceName := range *container.Instances {
			t.AppendRow(table.Row{"", ""})
			res, err := ctrClient.V1Alpha1().GetContainerWithResponse(c.Context, instanceName, &mni_ctr.GetContainerParams{Authorization: "Bearer " + c.String("token")})
			if err != nil {
				return err
			}

			if res.StatusCode() != 200 {
				t.AppendRow(table.Row{instanceName, fmt.Sprintf("Error %d: %s, %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message)})
			} else {
				instance := res.JSON200

				status := ""
				createdAt := ""

				if instance.Status != nil {
					status = *instance.Status
				}
				if instance.CreatedAt != nil {
					createdAt = instance.CreatedAt.String()
				}

				t.AppendRow(table.Row{"", "--" + *instance.Name})
				t.AppendRow(table.Row{"Status", status})
				t.AppendRow(table.Row{"CreatedAt", createdAt})
			}
		}
	}

	t.Render()

	return nil
}

func displayMultiple(c *cli.Context, containers []mni_ctr.ContainerPool) error {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Name", "Vpc", "Subnet", "Image", "Status"})

	for _, container := range containers {
		name := ""
		vpc := ""
		subnet := ""
		image := ""
		status := "Not Running"

		if container.Name != nil {
			name = *container.Name
		}

		if container.Vpc != nil {
			vpc = *container.Vpc
		}

		if container.Subnet != nil {
			subnet = *container.Subnet
		}

		if container.Image != nil {
			image = *container.Image
		}

		if container.Instances != nil && len(*container.Instances) > 0 {
			statusList := []string{}
			for _, instanceName := range *container.Instances {
				ctrClient, err := commands.NewCtrClient(c)
				if err != nil {
					return err
				}

				res, err := ctrClient.V1Alpha1().GetContainerWithResponse(c.Context, instanceName, &mni_ctr.GetContainerParams{Authorization: c.String("Authorization")})
				if err != nil {
					return err
				}

				if res.StatusCode() == 404 {
					statusList = append(statusList, "Unknown")
				} else if res.StatusCode() != 200 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					statusList = append(statusList, *res.JSON200.Status)
				}
			}
			status = strings.Join(statusList, ", ")
		}
		t.AppendRow(table.Row{name, vpc, subnet, image, status})
	}

	t.Render()
	return nil
}
