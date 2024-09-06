package vm

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jedib0t/go-pretty/v6/table"
	mni_vm "github.com/mNi-Cloud/backend/vm/pkg/client/v1alpha1"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

var VirtualMachineCommand = &cli.Command{
	Name: "vms",
	Subcommands: []*cli.Command{
		{
			Name:   "list",
			Before: commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				vmClient, err := commands.NewVmClient(c)
				if err != nil {
					return err
				}

				res, err := vmClient.V1Alpha1().GetVmListWithResponse(c.Context, &mni_vm.GetVmListParams{Authorization: "Bearer " + c.String("token")})
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
					vms := res.JSON200
					return displayMultipleVirtualMachine(c, *vms)
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

				res, err := vmClient.V1Alpha1().GetVmWithResponse(c.Context, c.Args().First(), &mni_vm.GetVmParams{Authorization: "Bearer " + c.String("token")})
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
					vm := res.JSON200
					return displaySingleVirtualMachine(c, *vm)
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
					Name:     "cores",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "memory",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "image",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "volume-size",
					Required: true,
				},
				&cli.StringFlag{
					Name:  "userdata-file",
					Usage: "userdata `FILE`",
				},
				&cli.StringSliceFlag{
					Name:  "additional-volume",
					Usage: "disk1:volume-name...",
				},
			},
			Before: commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				vmClient, err := commands.NewVmClient(c)
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
				cores := c.Int("cores")
				memory := c.String("memory")
				image := c.String("image")
				volumeSize := c.String("volume-size")
				additionalVolumes := c.StringSlice("additional-volume")

				additionalDisks := []mni_vm.VirtualMachineDisk{}

				if c.IsSet("userdata-file") {
					bytes, err := os.ReadFile(c.String("userdata-file"))
					if err != nil {
						return err
					}
					encodedUserdata := base64.StdEncoding.EncodeToString(bytes)
					additionalDisks = append(additionalDisks, mni_vm.VirtualMachineDisk{
						Name: "cloudinit",
						CloudInitSource: &struct {
							// UserData base64 encoded
							UserData string `json:"userData"`
						}{
							UserData: encodedUserdata,
						},
					})
				}

				if additionalVolumes != nil {
					for _, volume := range additionalVolumes {
						if !strings.Contains("volume", ":") {
							return cli.Exit("Invalid AdditionaoVolume Format (name:volume)", 1)
						}
						split := strings.SplitN(volume, ":", 2)
						name, volume := split[0], split[1]

						additionalDisks = append(additionalDisks, mni_vm.VirtualMachineDisk{
							Name: name,
							VolumeSource: &struct {
								Name string `json:"name"`
							}{Name: volume},
						})
					}
				}

				res, err := vmClient.V1Alpha1().CreateVmWithResponse(c.Context, &mni_vm.CreateVmParams{Authorization: "Bearer " + c.String("token")}, mni_vm.VirtualMachine{
					Name:   &name,
					Vpc:    vpc,
					Subnet: &subnet,
					Spec: mni_vm.VirtualMachineSpec{
						Cores:           &cores,
						Memory:          &memory,
						Image:           &image,
						VolumeSize:      &volumeSize,
						AdditionalDisks: &additionalDisks,
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
					vm := res.JSON201
					return displaySingleVirtualMachine(c, *vm)
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

				res, err := vmClient.V1Alpha1().DeleteVmWithResponse(c.Context, c.Args().First(), &mni_vm.DeleteVmParams{Authorization: "Bearer " + c.String("token")})
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
					fmt.Println("Vm deleted successfully")
					return nil
				}
			},
		},
		{
			Name:      "start",
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

				res, err := vmClient.V1Alpha1().StartVmWithResponse(c.Context, c.Args().First(), &mni_vm.StartVmParams{Authorization: "Bearer " + c.String("token")})
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
					fmt.Println("Vm started successfully")
					return nil
				}
			},
		},
		{
			Name:      "stop",
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

				res, err := vmClient.V1Alpha1().StopVmWithResponse(c.Context, c.Args().First(), &mni_vm.StopVmParams{Authorization: "Bearer " + c.String("token")})
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
					fmt.Println("Vm stopped successfully")
					return nil
				}
			},
		},
		{
			Name:      "reboot",
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

				res, err := vmClient.V1Alpha1().SoftRebootVmWithResponse(c.Context, c.Args().First(), &mni_vm.SoftRebootVmParams{Authorization: "Bearer " + c.String("token")})
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
					fmt.Println("Vm rebooted successfully")
					return nil
				}
			},
		},
		{
			Name:      "serial",
			ArgsUsage: "<name>",
			Before:    commands.TokenFunc(),
			Action: func(ctx *cli.Context) error {
				if ctx.NArg() != 1 {
					cli.ShowSubcommandHelpAndExit(ctx, 1)
					return nil
				}

				interrupt := make(chan os.Signal, 1)
				signal.Notify(interrupt, os.Interrupt)

				c, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("%s/v1alpha1/vms/%s/serial", strings.Replace(ctx.String("vm-endpoint"), "http", "ws", 1), ctx.Args().First()), http.Header{"Authorization": []string{"Bearer " + ctx.String("token")}})
				if err != nil {
					log.Fatal("dial:", err)
				}

				log.Println("Connected to serial console.")
				log.Println("Press Ctrl+] to exit.")
				defer c.Close()

				done := make(chan struct{})
				go func() {
					defer close(done)
					for {
						_, message, err := c.ReadMessage()
						if err != nil {
							log.Println("read:", err)
							return
						}
						fmt.Print(string(message[:]))
					}
				}()

				fd := int(os.Stdin.Fd())
				oldState, err := term.MakeRaw(fd)
				if err != nil {
					return err
				}
				defer term.Restore(fd, oldState)

				readStop := make(chan error)

				go func() {
					reader := bufio.NewReader(os.Stdin)
					for {
						buf, err := reader.ReadByte()
						if err != nil && err != io.EOF {
							readStop <- err
							return
						}
						if err == io.EOF {
							return
						}

						if buf == 29 {
							interrupt <- os.Interrupt
							return
						}

						err = c.WriteMessage(websocket.TextMessage, []byte{buf})
						if err != nil {
							readStop <- err
							return
						}

					}
				}()

				for {
					select {
					case <-done:
						return nil
					case <-interrupt:

						fmt.Println("\nDisconnecting...")

						select {
						case <-done:
						case <-time.After(time.Second):
						}
						return nil
					case err = <-readStop:
						if err != nil {
							return cli.Exit(err.Error(), 1)
						}
					}
				}
			},
		},
		{
			Name:      "vnc",
			ArgsUsage: "<name>",
			Flags:     []cli.Flag{},
			Before:    commands.TokenFunc(),
			Action: func(ctx *cli.Context) error {
				if ctx.NArg() != 1 {
					cli.ShowSubcommandHelpAndExit(ctx, 1)
					return nil
				}

				c, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("%s/v1alpha1/vms/%s/vnc", strings.Replace(ctx.String("vm-endpoint"), "http", "ws", 1), ctx.Args().First()), http.Header{"Authorization": []string{"Bearer " + ctx.String("token")}})
				if err != nil {
					log.Fatal("dial:", err)
				}
				defer c.Close()

				lnAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
				ln, err := net.ListenTCP("tcp", lnAddr)
				defer ln.Close()

				listenResChan := make(chan error)

				go func() {
					fd, err := ln.Accept()
					if err != nil {
						listenResChan <- err
					}
					defer fd.Close()

					readTcp := make(chan error)
					readWebsocket := make(chan error)

					go func() {
						for {
							_, data, err := c.ReadMessage()
							if err != nil {
								readWebsocket <- err
								return
							}

							_, err = fd.Write(data)
							if err != nil {
								readWebsocket <- err
								return
							}
						}
					}()

					go func() {
						buffer := make([]byte, 65536)
						for {
							bytesRead, err := fd.Read(buffer)

							if err != nil {
								readTcp <- err
								return
							}

							if err := c.WriteMessage(websocket.BinaryMessage, buffer[:bytesRead]); err != nil {
								readTcp <- err
								return
							}
						}
					}()

					select {
					case err := <-readTcp:
						if err != nil {
							listenResChan <- err
							return
						}
					case err := <-readWebsocket:
						if err != nil {
							listenResChan <- err
							return
						}
					}
					listenResChan <- nil
					return
				}()

				port := ln.Addr().(*net.TCPAddr).Port
				fmt.Printf("VNC server started on 127.0.0.1:%d\n", port)

				err = <-listenResChan
				if err != nil {
					return cli.Exit(err.Error(), 1)
				}

				return nil
			},
		},
	},
}

func displaySingleVirtualMachine(c *cli.Context, vm mni_vm.VirtualMachine) error {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Field", "Value"})

	name := ""
	vpc := ""
	subnet := ""
	pool := ""
	status := ""
	createdAt := ""

	if vm.Name != nil {
		name = *vm.Name
	}

	if vm.Vpc != nil {
		vpc = *vm.Vpc
	}

	if vm.Subnet != nil {
		subnet = *vm.Subnet
	}

	if vm.Pool != nil {
		pool = *vm.Pool
	}

	if vm.Status != nil {
		status = *vm.Status
	}

	if vm.CreatedAt != nil {
		createdAt = vm.CreatedAt.String()
	}

	t.AppendRow(table.Row{"Name", name})
	t.AppendRow(table.Row{"Vpc", vpc})
	t.AppendRow(table.Row{"Subnet", subnet})
	t.AppendRow(table.Row{"Pool", pool})
	displayVirtualMachineSpecForSingle(t, vm.Spec)
	t.AppendRow(table.Row{"Status", status})
	t.AppendRow(table.Row{"Created At", createdAt})

	t.Render()

	return nil
}

func displayMultipleVirtualMachine(c *cli.Context, vms []mni_vm.VirtualMachine) error {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Name", "Vpc", "Subnet", "Pool", "Status", "CreatedAt"})

	for _, vm := range vms {
		name := ""
		vpc := ""
		subnet := ""
		pool := ""
		status := ""
		createdAt := ""

		if vm.Name != nil {
			name = *vm.Name
		}

		if vm.Vpc != nil {
			vpc = *vm.Vpc
		}

		if vm.Subnet != nil {
			subnet = *vm.Subnet
		}

		if vm.Pool != nil {
			pool = *vm.Pool
		}

		if vm.Status != nil {
			status = *vm.Status
		}

		if vm.CreatedAt != nil {
			createdAt = vm.CreatedAt.String()
		}

		t.AppendRow(table.Row{name, vpc, subnet, pool, status, createdAt})
	}

	t.Render()

	return nil
}
