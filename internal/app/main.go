package app

import (
	"github.com/labstack/gommon/log"
	bsModel "github.com/mNi-Cloud/backend/bs/api/gen/v1alpha1"
	ctrModel "github.com/mNi-Cloud/backend/ctr/api/gen/v1alpha1"
	vmModel "github.com/mNi-Cloud/backend/vm/api/gen/v1alpha1"
	vpcModel "github.com/mNi-Cloud/backend/vpc/api/gen/v1alpha1"
	"github.com/mNi-Cloud/cli/internal/app/factory/login"
	"github.com/mNi-Cloud/cli/internal/app/factory/vm"
	"github.com/mNi-Cloud/cli/internal/pkg/factory"
	"github.com/urfave/cli/v2"
)

var version string

func New() *cli.App {
	app := cli.NewApp()
	app.Name = "mni"
	app.Version = version
	app.Description = "CLI client for mNi-Cloud services"
	app.EnableBashCompletion = true
	app.DisableSliceFlagSeparator = true

	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name: "debug",
		},
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

	app.Before = func(c *cli.Context) error {
		if c.Bool("debug") {
			log.SetLevel(log.DEBUG)
		}
		return nil
	}

	app.Commands = []*cli.Command{
		login.NewCommandFactory().Command("login"),
		{
			Name: "vpc",
			Subcommands: []*cli.Command{
				factory.NewRestCommandFactory("vpc-endpoint", "v1alpha1", "vpcs", vpcModel.Vpc()).Command("vpcs"),
				factory.NewRestCommandFactory("vpc-endpoint", "v1alpha1", "subnets", vpcModel.Subnet()).Command("subnets"),
				factory.NewRestCommandFactory("vpc-endpoint", "v1alpha1", "eips", vpcModel.Eip()).Command("eips"),
				factory.NewRestCommandFactory("vpc-endpoint", "v1alpha1", "eipassociates", vpcModel.EipAssociate()).Command("eipassociates"),
			},
		},
		{
			Name: "vm",
			Subcommands: []*cli.Command{
				vm.NewVirtualMachineCommandFactory(vmModel.VirtualMachine()).Command("vms"),
				vm.NewVirtualMachinePoolCommandFactory(vmModel.VirtualMachinePool()).Command("vmpools"),
				factory.NewRestCommandFactory("vm-endpoint", "v1alpha1", "images", vmModel.Image()).Command("images"),
			},
		},
		{
			Name: "bs",
			Subcommands: []*cli.Command{
				factory.NewRestCommandFactory("bs-endpoint", "v1alpha1", "volumes", bsModel.Volume()).Command("volumes"),
			},
		},
		{
			Name: "ctr",
			Subcommands: []*cli.Command{
				factory.NewRestCommandFactory("ctr-endpoint", "v1alpha1", "containers", ctrModel.Container()).Command("containers"),
				factory.NewRestCommandFactory("ctr-endpoint", "v1alpha1", "containerpools", ctrModel.ContainerPool()).Command("containerpools"),
			},
		},
	}

	return app
}
