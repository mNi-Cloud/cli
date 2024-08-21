package app

import (
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands/bs/volume"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands/ctr/container"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands/vm/image"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands/vm/vm"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands/vpc/eip"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands/vpc/eipassociate"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands/vpc/subnet"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands/vpc/vpc"
	"github.com/mNi-Cloud/cli/internal/pkg/oidc"
	"github.com/urfave/cli/v2"
)

func New() *cli.App {
	app := cli.NewApp()
	app.Name = "mni"
	app.Version = "0.0.1"
	app.Description = "CLI client for mNi-Cloud services"
	app.EnableBashCompletion = true

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "idp-endpoint",
			Usage:   "The endpoint of keycloak",
			Value:   "https://auth.mnicloud.jp/",
			EnvVars: []string{"MNI_IDP_ENDPOINT"},
		},
		&cli.StringFlag{
			Name:    "vpc-endpoint",
			Usage:   "The endpoint of mni-vpc service",
			Value:   "https://mnicloud.jp/api/vpc",
			EnvVars: []string{"MNI_VPC_ENDPOINT"},
		},
		&cli.StringFlag{
			Name:    "vm-endpoint",
			Usage:   "The endpoint of mni-vm service",
			Value:   "https://mnicloud.jp/api/vm",
			EnvVars: []string{"MNI_VM_ENDPOINT"},
		},
		&cli.StringFlag{
			Name:    "bs-endpoint",
			Usage:   "The endpoint of mni-bs service",
			Value:   "https://mnicloud.jp/api/bs",
			EnvVars: []string{"MNI_BS_ENDPOINT"},
		},
		&cli.StringFlag{
			Name:    "ctr-endpoint",
			Usage:   "The endpoint of mni-ctr service",
			Value:   "https://mnicloud.jp/api/ctr",
			EnvVars: []string{"MNI_CTR_ENDPOINT"},
		},
		&cli.StringFlag{
			Name:    "auth-endpoint",
			Usage:   "The endpoint of mni-auth service",
			Value:   "https://mnicloud.jp/api/auth",
			EnvVars: []string{"MNI_AUTH_ENDPOINT"},
		},
		&cli.StringFlag{
			Name:   "token",
			Hidden: true,
		},
	}

	app.Commands = []*cli.Command{
		{
			Name: "vpc",
			Subcommands: []*cli.Command{
				vpc.Command,
				subnet.Command,
				eip.Command,
				eipassociate.Command,
			},
		},
		{
			Name: "vm",
			Subcommands: []*cli.Command{
				vm.Command,
				image.Command,
			},
		},
		{
			Name: "bs",
			Subcommands: []*cli.Command{
				volume.Command,
			},
		},
		{
			Name: "ctr",
			Subcommands: []*cli.Command{
				container.Command,
			},
		},
	}

	app.Before = func(c *cli.Context) error {
		token, err := oidc.GetIdToken(c.Context, c.String("idp-endpoint"), "cloud", "mni-cli")
		if err != nil {
			return err
		}
		err = c.Set("token", *token)
		if err != nil {
			return err
		}

		return nil
	}
	return app
}
