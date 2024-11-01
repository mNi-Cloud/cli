package vm

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/mNi-Cloud/backend/common/pkg/mni"
	"github.com/mNi-Cloud/backend/common/pkg/mni/apigen/model"
	"github.com/mNi-Cloud/backend/vm/api/v1alpha1/vm"
	"github.com/mNi-Cloud/backend/vm/api/v1alpha1/vmpool"
	"github.com/mNi-Cloud/backend/vm/pkg/client"
	"github.com/mNi-Cloud/cli/internal/pkg/factory"
	"github.com/urfave/cli/v2"
	"os"
)

type VirtualMachinePoolCommandFactory struct {
	factory.CommandFactory
}

func (c VirtualMachinePoolCommandFactory) Command(name string) *cli.Command {
	command := c.CommandFactory.Command(name)

	var createCommand *cli.Command
	for _, subcommand := range command.Subcommands {
		if subcommand.Name == "create" {
			createCommand = subcommand
			break
		}
	}
	if createCommand != nil {
		createCommand.Flags = append(createCommand.Flags, &cli.StringFlag{
			Name:  "userdata-file",
			Usage: "userdata `FILE`",
		})

		createCommand.Before = func(c *cli.Context) error {
			err := factory.TokenFunc()(c)
			if err != nil {
				return err
			}

			if c.IsSet("userdata-file") {
				bytes, err := os.ReadFile(c.String("userdata-file"))
				if err != nil {
					return err
				}
				userDataBase64 := base64.StdEncoding.EncodeToString(bytes)

				diskModel := &vm.VirtualMachineDiskModel{
					Name: mni.String("cloudinit"),
					CloudInitSource: &vm.VirtualMachineDiskCloudInitSourceModel{
						UserDataBase64: &userDataBase64,
					},
				}

				bytes, err = json.Marshal(diskModel)
				if err != nil {
					return err
				}
				err = c.Set("spec.additionalDisks", string(bytes))
				if err != nil {
					return err
				}
			}
			return nil
		}
	}

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
