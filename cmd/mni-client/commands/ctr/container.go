package ctr

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	mni_ctr "github.com/mNi-Cloud/backend/ctr/pkg/client/v1alpha1"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands"
	"github.com/urfave/cli/v2"
)

var ContainerCommand = &cli.Command{
	Name: "containers",
	Subcommands: []*cli.Command{
		{
			Name:   "list",
			Before: commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				ctrClient, err := commands.NewCtrClient(c)
				if err != nil {
					return err
				}

				res, err := ctrClient.V1Alpha1().GetContainerListWithResponse(c.Context, &mni_ctr.GetContainerListParams{Authorization: "Bearer " + c.String("token")})
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
					containers := res.JSON200
					return displayMultipleContainer(c, *containers)
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

				ctrClient, err := commands.NewCtrClient(c)
				if err != nil {
					return err
				}

				res, err := ctrClient.V1Alpha1().GetContainerWithResponse(c.Context, c.Args().First(), &mni_ctr.GetContainerParams{Authorization: "Bearer " + c.String("token")})
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
					container := res.JSON200
					return displaySingleContainer(c, *container)
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
					Name: "vpc",
				},
				&cli.StringFlag{
					Name:     "subnet",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "image",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "cores",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "memory",
					Required: true,
				},
				&cli.StringSliceFlag{
					Name: "env",
				},
				&cli.StringSliceFlag{
					Name: "mounts",
				},
			},
			Before: commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				ctrClient, err := commands.NewCtrClient(c)
				if err != nil {
					return err
				}

				name := c.String("name")
				rawVpc := c.String("vpc")
				var vpc *string
				if rawVpc != "" {
					vpc = &rawVpc
				}
				subnet := c.String("subnet")
				image := c.String("image")
				rawEnv := c.StringSlice("env")
				rawMounts := c.StringSlice("mounts")
				cores := c.String("cores")
				memory := c.String("memory")

				env := []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				}{}
				if rawEnv != nil {
					for _, entry := range rawEnv {
						if !strings.Contains(entry, "=") {
							return cli.Exit("Invalid Env Format (KEY=value)", 1)
						}
						split := strings.SplitN(entry, "=", 2)
						key, value := split[0], split[1]

						env = append(env, struct {
							Name  string `json:"name"`
							Value string `json:"value"`
						}{Name: key, Value: value})
					}
				}

				mounts := []mni_ctr.ContainerVolumeMount{}
				if rawMounts != nil {
					for _, entry := range rawMounts {
						split := strings.SplitN(entry, ":", 3)
						if len(split) < 3 {
							return cli.Exit("Invalid VolumeMount Format (name:volume:path)", 1)
						}
						name, volume, path := split[0], split[1], split[2]

						mounts = append(mounts, mni_ctr.ContainerVolumeMount{
							Name:      name,
							Volume:    volume,
							MountPath: path,
						})
					}
				}

				res, err := ctrClient.V1Alpha1().CreateContainerWithResponse(c.Context, &mni_ctr.CreateContainerParams{Authorization: "Bearer " + c.String("token")}, mni_ctr.Container{
					Name:   &name,
					Vpc:    vpc,
					Subnet: &subnet,
					Spec: mni_ctr.ContainerSpec{
						Image:        &image,
						Env:          &env,
						VolumeMounts: &mounts,
						Cores:        &cores,
						Memory:       &memory,
					},
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
					container := res.JSON201
					return displaySingleContainer(c, *container)
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

				ctrClient, err := commands.NewCtrClient(c)
				if err != nil {
					return err
				}

				res, err := ctrClient.V1Alpha1().DeleteContainerWithResponse(c.Context, c.Args().First(), &mni_ctr.DeleteContainerParams{Authorization: "Bearer " + c.String("token")})
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
					fmt.Println("Container deleted successfully")
					return nil
				}
			},
		},
	},
}

func displaySingleContainer(c *cli.Context, container mni_ctr.Container) error {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Field", "Value"})

	name := ""
	vpc := ""
	subnet := ""
	pool := ""
	status := ""
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

	if container.Pool != nil {
		pool = *container.Pool
	}

	if container.Status != nil {
		status = *container.Status
	}

	if container.CreatedAt != nil {
		createdAt = container.CreatedAt.String()
	}

	t.AppendRow(table.Row{"Name", name})
	t.AppendRow(table.Row{"Vpc", vpc})
	t.AppendRow(table.Row{"Subnet", subnet})
	t.AppendRow(table.Row{"Pool", pool})
	displayContainerSpecForSingle(t, container.Spec)
	t.AppendRow(table.Row{"Status", status})
	t.AppendRow(table.Row{"CreatedAt", createdAt})

	t.Render()

	return nil
}

func displayMultipleContainer(c *cli.Context, containers []mni_ctr.Container) error {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Name", "Vpc", "Subnet", "Pool", "Image", "Status", "CreatedAt"})

	for _, container := range containers {
		name := ""
		vpc := ""
		subnet := ""
		pool := ""
		image := ""
		status := ""
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

		if container.Pool != nil {
			pool = *container.Pool
		}

		if container.Spec.Image != nil {
			image = *container.Spec.Image
		}

		if container.Status != nil {
			status = *container.Status
		}

		if container.CreatedAt != nil {
			createdAt = container.CreatedAt.String()
		}

		t.AppendRow(table.Row{name, vpc, subnet, pool, image, status, createdAt})
	}

	t.Render()

	return nil
}
