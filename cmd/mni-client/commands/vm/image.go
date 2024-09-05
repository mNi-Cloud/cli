package vm

import (
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mNi-Cloud/backend/vm/pkg/client/v1alpha1"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands"
	"github.com/urfave/cli/v2"
)

var ImageCommand = &cli.Command{
	Name: "images",
	Subcommands: []*cli.Command{
		{
			Name: "list",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "include-public",
					Value: false,
				},
			},
			Before: commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				vmClient, err := commands.NewVmClient(c)
				if err != nil {
					return err
				}

				includePublic := c.Bool("include-public")

				res, err := vmClient.V1Alpha1().GetImageListWithResponse(c.Context, &v1alpha1.GetImageListParams{Authorization: "Bearer " + c.String("token"), IncludePublic: &includePublic})
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
					images := res.JSON200

					displayMultipleImage(c, *images)

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

				vmClient, err := commands.NewVmClient(c)
				if err != nil {
					return err
				}
				res, err := vmClient.V1Alpha1().GetImageWithResponse(c.Context, c.Args().First(), &v1alpha1.GetImageParams{Authorization: "Bearer " + c.String("token")})
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
					image := res.JSON200

					displaySingleImage(c, *image)

					return nil
				}
			},
		},
		{
			Name: "create",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "volume",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "os",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "version",
					Required: true,
				},
				&cli.BoolFlag{
					Name:  "public",
					Value: false,
				},
			},
			Before: commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				vmClient, err := commands.NewVmClient(c)
				if err != nil {
					return err
				}

				volume := c.String("volume")
				os := c.String("os")
				version := c.String("version")
				public := c.Bool("public")

				res, err := vmClient.V1Alpha1().CreateImageWithResponse(c.Context, &v1alpha1.CreateImageParams{Authorization: "Bearer " + c.String("token")}, v1alpha1.Image{
					Volume:  &volume,
					Os:      &os,
					Version: &version,
					Public:  &public,
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
					image := res.JSON201

					displaySingleImage(c, *image)

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

				vmClient, err := commands.NewVmClient(c)
				if err != nil {
					return err
				}

				res, err := vmClient.V1Alpha1().DeleteImageWithResponse(c.Context, c.Args().First(), &v1alpha1.DeleteImageParams{Authorization: "Bearer " + c.String("token")})

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
					fmt.Println("Image deleted successfully")
					return nil
				}
			},
		},
	},
}

func displayMultipleImage(c *cli.Context, images []v1alpha1.Image) {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Name", "Os", "Version", "Public"})

	for _, image := range images {

		name := ""
		os := ""
		version := ""
		public := false

		if image.Name != nil {
			name = *image.Name
		}
		if image.Os != nil {
			os = *image.Os
		}
		if image.Version != nil {
			version = *image.Version
		}
		if image.Public != nil {
			public = *image.Public
		}

		t.AppendRow(table.Row{name, os, version, public})
	}

	t.Render()

}

func displaySingleImage(c *cli.Context, image v1alpha1.Image) {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Field", "Value"})

	name := ""
	os := ""
	version := ""
	public := false

	if image.Name != nil {
		name = *image.Name
	}
	if image.Os != nil {
		os = *image.Os
	}
	if image.Version != nil {
		version = *image.Version
	}
	if image.Public != nil {
		public = *image.Public
	}

	t.AppendRow(table.Row{"Name", name})
	t.AppendRow(table.Row{"Os", os})
	t.AppendRow(table.Row{"Version", version})
	t.AppendRow(table.Row{"Public", public})

	t.Render()
}
