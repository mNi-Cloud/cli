package ctr

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	mni_ctr "github.com/mNi-Cloud/backend/ctr/pkg/client/v1alpha1"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands"
	"github.com/urfave/cli/v2"
)

var ContainerPoolCommand = &cli.Command{
	Name: "containerpools",
	Subcommands: []*cli.Command{
		{
			Name:   "list",
			Before: commands.TokenFunc(),
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
					if res.JSONDefault != nil {
						return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
					} else {
						return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), string(res.Body)), 1)
					}

				} else {
					containers := res.JSON200
					return displayMultipleContainerPool(c, *containers)
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

				res, err := ctrClient.V1Alpha1().GetContainerPoolByNameWithResponse(c.Context, c.Args().First(), &mni_ctr.GetContainerPoolByNameParams{Authorization: "Bearer " + c.String("token")})
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
					return displaySingleContainerPool(c, *container)
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
				&cli.IntFlag{
					Name:     "replicas",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "image",
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
				replicas := c.Int("replicas")
				image := c.String("image")
				rawEnv := c.StringSlice("env")
				rawMounts := c.StringSlice("mounts")

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

				res, err := ctrClient.V1Alpha1().CreateContainerPoolWithResponse(c.Context, &mni_ctr.CreateContainerPoolParams{Authorization: "Bearer " + c.String("token")}, mni_ctr.ContainerPool{
					Name:     &name,
					Vpc:      vpc,
					Subnet:   &subnet,
					Replicas: &replicas,
					Spec: mni_ctr.ContainerSpec{
						Image:        &image,
						Env:          &env,
						VolumeMounts: &mounts,
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
					return displaySingleContainerPool(c, *container)
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

				res, err := ctrClient.V1Alpha1().DeleteContainerPoolByNameWithResponse(c.Context, c.Args().First(), &mni_ctr.DeleteContainerPoolByNameParams{Authorization: "Bearer " + c.String("token")})
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
					fmt.Println("VM deleted successfully")
					return nil
				}
			},
		},
	},
}

func displayContainerSpecForSingle(t table.Writer, spec mni_ctr.ContainerSpec) {
	image := ""

	if spec.Image != nil {
		image = *spec.Image
	}

	t.AppendRow(table.Row{"Image", image})

	if spec.Env == nil {
		t.AppendRow(table.Row{"Env", ""})
	} else {
		first := true
		for _, entry := range *spec.Env {
			entryString := entry.Name + "=" + entry.Value
			if first {
				t.AppendRow(table.Row{"Env", entryString})
				first = false
			} else {
				t.AppendRow(table.Row{"", entryString})
			}
		}
	}

	if spec.VolumeMounts == nil {
		t.AppendRow(table.Row{"VolumeMount", ""})
	} else {
		first := true
		for _, volumeMount := range *spec.VolumeMounts {
			entryString := "(" + volumeMount.Name + ")" + volumeMount.Volume + ":" + volumeMount.MountPath
			if first {
				t.AppendRow(table.Row{"VolumeMount", entryString})
				first = false
			} else {
				t.AppendRow(table.Row{"", entryString})
			}
		}
	}
}

func displaySingleContainerPool(c *cli.Context, container mni_ctr.ContainerPool) error {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Field", "Value"})

	name := ""
	vpc := ""
	subnet := ""
	replicas := ""
	instances := ""
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

	if container.Replicas != nil {
		replicas = strconv.Itoa(*container.Replicas)
	}

	if container.Instances != nil {
		instances = strings.Join(*container.Instances, ", ")
	}

	if container.CreatedAt != nil {
		createdAt = container.CreatedAt.String()
	}

	t.AppendRow(table.Row{"Name", name})
	t.AppendRow(table.Row{"Vpc", vpc})
	t.AppendRow(table.Row{"Subnet", subnet})
	displayContainerSpecForSingle(t, container.Spec)
	t.AppendRow(table.Row{"Replicas", replicas})
	t.AppendRow(table.Row{"Instances", instances})
	t.AppendRow(table.Row{"CreatedAt", createdAt})

	t.Render()

	return nil
}

func displayMultipleContainerPool(c *cli.Context, containers []mni_ctr.ContainerPool) error {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Name", "Vpc", "Subnet", "Image", "Instances"})

	for _, container := range containers {
		name := ""
		vpc := ""
		subnet := ""
		image := ""
		instances := ""

		if container.Name != nil {
			name = *container.Name
		}

		if container.Vpc != nil {
			vpc = *container.Vpc
		}

		if container.Subnet != nil {
			subnet = *container.Subnet
		}

		if container.Spec.Image != nil {
			image = *container.Spec.Image
		}

		if container.Instances != nil {
			instances = strings.Join(*container.Instances, ", ")
		}

		t.AppendRow(table.Row{name, vpc, subnet, image, instances})
	}

	t.Render()
	return nil
}
