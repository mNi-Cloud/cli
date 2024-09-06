package app

import (
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands/bs"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands/ctr"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands/login"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands/vm"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands/vpc"
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
			Value:   "https://api.mnicloud.jp/vpc",
			EnvVars: []string{"MNI_VPC_ENDPOINT"},
		},
		&cli.StringFlag{
			Name:    "vm-endpoint",
			Usage:   "The endpoint of mni-vm service",
			Value:   "https://api.mnicloud.jp/vm",
			EnvVars: []string{"MNI_VM_ENDPOINT"},
		},
		&cli.StringFlag{
			Name:    "bs-endpoint",
			Usage:   "The endpoint of mni-bs service",
			Value:   "https://api.mnicloud.jp/bs",
			EnvVars: []string{"MNI_BS_ENDPOINT"},
		},
		&cli.StringFlag{
			Name:    "ctr-endpoint",
			Usage:   "The endpoint of mni-ctr service",
			Value:   "https://api.mnicloud.jp/ctr",
			EnvVars: []string{"MNI_CTR_ENDPOINT"},
		},
		&cli.StringFlag{
			Name:    "auth-endpoint",
			Usage:   "The endpoint of mni-auth service",
			Value:   "https://api.mnicloud.jp/auth",
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
				vpc.VpcCommand,
				vpc.SubnetCommand,
				vpc.EipCommand,
				vpc.EipAssociateCommand,
			},
		},
		{
			Name: "vm",
			Subcommands: []*cli.Command{
				vm.VirtualMachinePoolCommand,
				vm.VirtualMachineCommand,
				vm.ImageCommand,
			},
		},
		{
			Name: "bs",
			Subcommands: []*cli.Command{
				bs.VolumeCommand,
			},
		},
		{
			Name: "ctr",
			Subcommands: []*cli.Command{
				ctr.ContainerPoolCommand,
				ctr.ContainerCommand,
			},
		},
		login.Command,
	}

	return app
}
