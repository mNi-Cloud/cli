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

var Command = &cli.Command{
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
				res, err := vmClient.V1Alpha1().GetVmPoolListWithResponse(c.Context, &mni_vm.GetVmPoolListParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 200 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					vms := res.JSON200

					return displayMultiple(c, *vms)
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

				res, err := vmClient.V1Alpha1().GetVmPoolWithResponse(c.Context, c.Args().First(), &mni_vm.GetVmPoolParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 200 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					vm := res.JSON200

					return displaySingle(c, *vm)
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
				&cli.UintFlag{
					Name:     "cores",
					Required: true,
				},
				&cli.UintFlag{
					Name:     "memory",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "image",
					Required: true,
				},
				&cli.UintFlag{
					Name:     "volume-size",
					Required: true,
				},
				&cli.StringFlag{
					Name:  "userdata-file",
					Usage: "userdata `FILE`",
				},
			},
			Before: commands.TokenFunc(),
			Action: func(c *cli.Context) error {
				vmClient, err := commands.NewVmClient(c)
				if err != nil {
					return err
				}

				name := c.String("name")
				vpc := c.String("vpc")
				subnet := c.String("subnet")
				cores := int(c.Uint("cores"))
				memory := fmt.Sprintf("%dM", c.Uint("memory"))
				image := c.String("image")
				volumeSize := fmt.Sprintf("%dGi", c.Uint("volume-size"))

				var userdata *string
				if c.IsSet("userdata-file") {
					bytes, err := os.ReadFile(c.String("userdata-file"))
					if err != nil {
						return err
					}
					encodedUserdata := base64.StdEncoding.EncodeToString(bytes)
					userdata = &encodedUserdata
				}

				res, err := vmClient.V1Alpha1().CreateVmPoolWithResponse(c.Context, &mni_vm.CreateVmPoolParams{Authorization: "Bearer " + c.String("token")}, mni_vm.VirtualMachinePool{
					Name:       &name,
					Vpc:        &vpc,
					Subnet:     &subnet,
					Cores:      &cores,
					Memory:     &memory,
					Image:      &image,
					VolumeSize: &volumeSize,
					UserData:   userdata,
				})

				if res.StatusCode() != 201 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					vm := res.JSON201

					return displaySingle(c, *vm)
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

				res, err := vmClient.V1Alpha1().DeleteVmPoolWithResponse(c.Context, c.Args().First(), &mni_vm.DeleteVmPoolParams{Authorization: "Bearer " + c.String("token")})
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

				res, err := vmClient.V1Alpha1().StartVmPoolWithResponse(c.Context, c.Args().First(), &mni_vm.StartVmPoolParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 204 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					fmt.Println("VM started successfully")
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

				res, err := vmClient.V1Alpha1().StopVmPoolWithResponse(c.Context, c.Args().First(), &mni_vm.StopVmPoolParams{Authorization: "Bearer " + c.String("token")})
				if err != nil {
					return err
				}

				if res.StatusCode() != 204 {
					return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
				} else {
					fmt.Println("VM stopped successfully")
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

				c, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("%s/api/v1alpha1/vms/%s/serial", strings.Replace(ctx.String("vm-endpoint"), "http", "ws", 1), ctx.Args().First()), http.Header{"X-namespace": []string{ctx.String("namespace")}})
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

				c, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("%s/api/v1alpha1/vms/%s/vnc", strings.Replace(ctx.String("vm-endpoint"), "http", "ws", 1), ctx.Args().First()), http.Header{"X-namespace": []string{ctx.String("namespace")}})
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

func displaySingle(c *cli.Context, vm mni_vm.VirtualMachinePool) error {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Field", "Value"})

	name := ""
	vpc := ""
	subnet := ""
	cores := 0
	memory := ""
	image := ""
	volumeSize := ""
	userdata := ""
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
	if vm.Cores != nil {
		cores = *vm.Cores
	}
	if vm.Memory != nil {
		memory = *vm.Memory
	}
	if vm.Image != nil {
		image = *vm.Image
	}
	if vm.VolumeSize != nil {
		volumeSize = *vm.VolumeSize
	}
	if vm.UserData != nil {
		userdata = *vm.UserData
	}
	if vm.CreatedAt != nil {
		createdAt = vm.CreatedAt.String()
	}

	t.AppendRow(table.Row{"Name", name})
	t.AppendRow(table.Row{"Vpc", vpc})
	t.AppendRow(table.Row{"Subnet", subnet})
	t.AppendRow(table.Row{"Cores", cores})
	t.AppendRow(table.Row{"Memory", memory})
	t.AppendRow(table.Row{"Image", image})
	t.AppendRow(table.Row{"VolumeSize", volumeSize})
	t.AppendRow(table.Row{"UserData", userdata})
	t.AppendRow(table.Row{"CreatedAt", createdAt})

	if vm.Instances != nil && len(*vm.Instances) > 0 {
		vmClient, err := commands.NewVmClient(c)
		if err != nil {
			return err
		}
		for _, instanceName := range *vm.Instances {
			t.AppendRow(table.Row{"", ""})
			res, err := vmClient.V1Alpha1().GetVmWithResponse(c.Context, instanceName, &mni_vm.GetVmParams{Authorization: "Bearer " + c.String("token")})
			if err != nil {
				return err
			}

			if res.StatusCode() != 200 {
				t.AppendRow(table.Row{instanceName, fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message)})
			} else {
				instance := res.JSON200

				ip := ""
				status := ""
				createdAt := ""

				if instance.IpAddr != nil {
					ip = *instance.IpAddr
				}
				if instance.Status != nil {
					status = *instance.Status
				}
				if instance.CreatedAt != nil {
					createdAt = instance.CreatedAt.String()
				}

				t.AppendRow(table.Row{"", "-- " + *instance.Name})
				if instance.Volumes != nil && len(*instance.Volumes) > 0 {
					volumes := []string{}
					for _, volume := range *instance.Volumes {
						if volume.VolumeSource != nil {
							volumes = append(volumes, "Volume: "+volume.Name+":"+volume.VolumeSource.Name)
						} else if volume.CloudInitSource != nil {
							volumes = append(volumes, "CloudInit: "+volume.Name)
						} else {
							volumes = append(volumes, "Unknown: "+volume.Name)
						}
					}
					t.AppendRow(table.Row{"Volumes", strings.Join(volumes, ", ")})
				}
				t.AppendRow(table.Row{"IP", ip})
				t.AppendRow(table.Row{"Status", status})
				t.AppendRow(table.Row{"CreatedAt", createdAt})
			}
		}
	}

	t.Render()

	return nil
}

func displayMultiple(c *cli.Context, vms []mni_vm.VirtualMachinePool) error {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Name", "Vpc", "Subnet", "Status"})

	for _, vm := range vms {

		name := ""
		vpc := ""
		subnet := ""
		status := ""

		if vm.Name != nil {
			name = *vm.Name
		}
		if vm.Vpc != nil {
			vpc = *vm.Vpc
		}
		if vm.Subnet != nil {
			subnet = *vm.Subnet
		}

		if vm.Instances != nil && len(*vm.Instances) > 0 {
			instanceName := (*vm.Instances)[0]

			vmClient, err := commands.NewVmClient(c)
			if err != nil {
				return err
			}

			res, err := vmClient.V1Alpha1().GetVmWithResponse(c.Context, instanceName, &mni_vm.GetVmParams{Authorization: "Bearer " + c.String("token")})
			if err != nil {
				return err
			}

			if res.StatusCode() != 200 {
				return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
			} else {
				if res.JSON200.Status != nil {
					status = *res.JSON200.Status
				} else {
					status = "Unknown"
				}
			}
		} else {
			status = "Not Running"
		}

		t.AppendRow(table.Row{name, vpc, subnet, status})
	}

	t.Render()

	return nil
}
