package vm

import (
	"fmt"
	"github.com/mNi-Cloud/backend/common/pkg/mni/apigen/model"
	"github.com/mNi-Cloud/backend/vm/api/v1alpha1/vmpool"
	"github.com/mNi-Cloud/backend/vm/pkg/client"
	"github.com/mNi-Cloud/cli/internal/pkg/factory"
	"github.com/urfave/cli/v2"
)

type VirtualMachinePoolCommandFactory struct {
	factory.CommandFactory
}

func (c VirtualMachinePoolCommandFactory) Command(name string) *cli.Command {
	command := c.CommandFactory.Command(name)
	command.Subcommands = append(command.Subcommands, &cli.Command{
		Name:      "start",
		Before:    factory.TokenFunc(),
		ArgsUsage: "<name>",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() != 1 {
				cli.ShowSubcommandHelpAndExit(ctx, 1)
				return nil
			}

			vmClient := client.NewClient(ctx.String("vm-endpoint"))

			res, err := vmClient.V1Alpha1().VirtualMachinePools().StartVmPoolWithResponse(ctx.Context, ctx.Args().First(), &vmpool.StartVmPoolParams{Authorization: "Bearer " + ctx.String("token")})
			if err != nil {
				return err
			}

			if res.StatusCode() != 200 {
				if res.JSONDefault != nil {
					return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), res.JSONDefault.Message), 1)
				} else {
					return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), string(res.Body)), 1)
				}
			} else {
				fmt.Println("VmPool started successfully")
				return nil
			}
		},
	}, &cli.Command{
		Name:      "stop",
		Before:    factory.TokenFunc(),
		ArgsUsage: "<name>",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() != 1 {
				cli.ShowSubcommandHelpAndExit(ctx, 1)
				return nil
			}

			vmClient := client.NewClient(ctx.String("vm-endpoint"))

			res, err := vmClient.V1Alpha1().VirtualMachinePools().StopVmPoolWithResponse(ctx.Context, ctx.Args().First(), &vmpool.StopVmPoolParams{Authorization: "Bearer " + ctx.String("token")})
			if err != nil {
				return err
			}

			if res.StatusCode() != 200 {
				if res.JSONDefault != nil {
					return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), res.JSONDefault.Message), 1)
				} else {
					return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), string(res.Body)), 1)
				}
			} else {
				fmt.Println("VmPool stopped successfully")
				return nil
			}
		},
	})
	return command
}

func NewVirtualMachinePoolCommandFactory(model *model.Model) factory.CommandFactory {
	return &VirtualMachinePoolCommandFactory{
		CommandFactory: factory.NewRestCommandFactory("vm-endpoint", "v1alpha1", "vmpools", model),
	}
}
