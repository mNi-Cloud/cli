package vm

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	mni_vm "github.com/mNi-Cloud/backend/vm/pkg/client/v1alpha1"
	"github.com/mNi-Cloud/cli/cmd/mni-client/commands"
	"github.com/urfave/cli/v2"
)

var VirtualMachinePoolCommand = &cli.Command{
	Name: "vmpools",
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
					if res.JSONDefault != nil {
						return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
					} else {
						return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), string(res.Body)), 1)
					}

				} else {
					vms := res.JSON200

					return displayMultipleVirtualMachinePool(c, *vms)
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
					if res.JSONDefault != nil {
						return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
					} else {
						return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), string(res.Body)), 1)
					}

				} else {
					vm := res.JSON200

					return displaySingleVirtualMachinePool(c, *vm)
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
				&cli.IntFlag{
					Name:     "replicas",
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
				replicas := c.Int("replicas")
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

				res, err := vmClient.V1Alpha1().CreateVmPoolWithResponse(c.Context, &mni_vm.CreateVmPoolParams{Authorization: "Bearer " + c.String("token")}, mni_vm.VirtualMachinePool{
					Name:     &name,
					Vpc:      vpc,
					Subnet:   &subnet,
					Replicas: &replicas,
					Spec: mni_vm.VirtualMachineSpec{
						Cores:           &cores,
						Memory:          &memory,
						Image:           &image,
						VolumeSize:      &volumeSize,
						AdditionalDisks: &additionalDisks,
					},
				})

				if res.StatusCode() != 201 {
					if res.JSONDefault != nil {
						return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
					} else {
						return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), string(res.Body)), 1)
					}

				} else {
					vm := res.JSON201

					return displaySingleVirtualMachinePool(c, *vm)
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
					if res.JSONDefault != nil {
						return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
					} else {
						return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), string(res.Body)), 1)
					}

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
					if res.JSONDefault != nil {
						return cli.Exit(fmt.Sprintf("Error %d: %s %s", res.StatusCode(), res.JSONDefault.Resource, res.JSONDefault.Message), 1)
					} else {
						return cli.Exit(fmt.Sprintf("Error %d: %s", res.StatusCode(), string(res.Body)), 1)
					}

				} else {
					fmt.Println("VM stopped successfully")
					return nil
				}
			},
		},
		//{
		//		Name:      "serial",
		//		ArgsUsage: "<name>",
		//		Before:    commands.TokenFunc(),
		//		Action: func(ctx *cli.Context) error {
		//			if ctx.NArg() != 1 {
		//				cli.ShowSubcommandHelpAndExit(ctx, 1)
		//				return nil
		//			}

		//			interrupt := make(chan os.Signal, 1)
		//			signal.Notify(interrupt, os.Interrupt)

		//			c, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("%s/v1alpha1/vms/%s/serial", strings.Replace(ctx.String("vm-endpoint"), "http", "ws", 1), ctx.Args().First()), http.Header{"Authorization//": []string{"Bearer " + ctx.String("token")}})
		//			if err != nil {
		//				log.Fatal("dial:", err)
		//			}

		//			log.Println("Connected to serial console.")
		//			log.Println("Press Ctrl+] to exit.")
		//			defer c.Close()

		//			done := make(chan struct{})
		//			go func() {
		//				defer close(done)
		//				for {
		//					_, message, err := c.ReadMessage()
		//					if err != nil {
		//						log.Println("read:", err)
		//						return
		//					}
		//					fmt.Print(string(message[:]))
		//				}
		//			}()

		//			fd := int(os.Stdin.Fd())
		//			oldState, err := term.MakeRaw(fd)
		//			if err != nil {
		//				return err
		//			}
		//			defer term.Restore(fd, oldState)

		//			readStop := make(chan error)

		//			go func() {
		//				reader := bufio.NewReader(os.Stdin)
		//				for {
		//					buf, err := reader.ReadByte()
		//					if err != nil && err != io.EOF {
		//						readStop <- err
		//						return
		//					}
		//					if err == io.EOF {
		//						return
		//					}

		//					if buf == 29 {
		//						interrupt <- os.Interrupt
		//						return
		//					}

		//					err = c.WriteMessage(websocket.TextMessage, []byte{buf})
		//					if err != nil {
		//						readStop <- err
		//						return
		//					}

		//				}
		//			}()

		//			for {
		//				select {
		//				case <-done:
		//					return nil
		//				case <-interrupt:

		//					fmt.Println("\nDisconnecting...")

		//					select {
		//					case <-done:
		//					case <-time.After(time.Second):
		//					}
		//					return nil
		//				case err = <-readStop:
		//					if err != nil {
		//						return cli.Exit(err.Error(), 1)
		//					}
		//				}
		//			}
		//		},
		//},
		//{
		//		Name:      "vnc",
		//		ArgsUsage: "<name>",
		//		Flags:     []cli.Flag{},
		//		Before:    commands.TokenFunc(),
		//		Action: func(ctx *cli.Context) error {
		//			if ctx.NArg() != 1 {
		//				cli.ShowSubcommandHelpAndExit(ctx, 1)
		//				return nil
		//			}

		//			c, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("%s/v1alpha1/vms/%s/vnc", strings.Replace(ctx.String("vm-endpoint"), "http", "ws", 1), ctx.Args().First()), http.Header{"Authorization": //[]string{"Bearer " + ctx.String("token")}})
		//			if err != nil {
		//				log.Fatal("dial:", err)
		//			}
		//			defer c.Close()

		//			lnAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		//			ln, err := net.ListenTCP("tcp", lnAddr)
		//			defer ln.Close()

		//			listenResChan := make(chan error)

		//			go func() {
		//				fd, err := ln.Accept()
		//				if err != nil {
		//					listenResChan <- err
		//				}
		//				defer fd.Close()

		//				readTcp := make(chan error)
		//				readWebsocket := make(chan error)

		//				go func() {
		//					for {
		//						_, data, err := c.ReadMessage()
		//						if err != nil {
		//							readWebsocket <- err
		//							return
		//						}

		//						_, err = fd.Write(data)
		//						if err != nil {
		//							readWebsocket <- err
		//							return
		//						}
		//					}
		//				}()

		//				go func() {
		//					buffer := make([]byte, 65536)
		//					for {
		//						bytesRead, err := fd.Read(buffer)

		//						if err != nil {
		//							readTcp <- err
		//							return
		//						}

		//						if err := c.WriteMessage(websocket.BinaryMessage, buffer[:bytesRead]); err != nil {
		//							readTcp <- err
		//							return
		//						}
		//					}
		//				}()

		//				select {
		//				case err := <-readTcp:
		//					if err != nil {
		//						listenResChan <- err
		//						return
		//					}
		//				case err := <-readWebsocket:
		//					if err != nil {
		//						listenResChan <- err
		//						return
		//					}
		//				}
		//				listenResChan <- nil
		//				return
		//			}()

		//			port := ln.Addr().(*net.TCPAddr).Port
		//			fmt.Printf("VNC server started on 127.0.0.1:%d\n", port)

		//			err = <-listenResChan
		//			if err != nil {
		//				return cli.Exit(err.Error(), 1)
		//			}

		//			return nil
		//		},
		//},
	},
}

func displayVirtualMachineSpecForSingle(t table.Writer, spec mni_vm.VirtualMachineSpec) {
	cores := ""
	memory := ""
	image := ""
	volumeSize := ""

	if spec.Cores != nil {
		cores = strconv.FormatInt(int64(*spec.Cores), 10)
	}

	if spec.Memory != nil {
		memory = *spec.Memory
	}

	if spec.Image != nil {
		image = *spec.Image
	}

	if spec.VolumeSize != nil {
		volumeSize = *spec.VolumeSize
	}

	t.AppendRow(table.Row{"Cores", cores})
	t.AppendRow(table.Row{"Memory", memory})
	t.AppendRow(table.Row{"Image", image})
	t.AppendRow(table.Row{"VolumeSize", volumeSize})

	if spec.AdditionalDisks != nil {
		first := true
		for _, disk := range *spec.AdditionalDisks {
			var diskString string
			if disk.VolumeSource != nil {
				diskString = disk.Name + " : Volume " + disk.VolumeSource.Name
			} else if disk.CloudInitSource != nil {
				diskString = disk.Name + " : CloudInit " + disk.CloudInitSource.UserData
			}

			if first {
				t.AppendRow(table.Row{"AdditionalDisk", diskString})
				first = false
			} else {
				t.AppendRow(table.Row{"", diskString})
			}
		}
	}
}

func displaySingleVirtualMachinePool(c *cli.Context, vm mni_vm.VirtualMachinePool) error {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Field", "Value"})

	name := ""
	vpc := ""
	subnet := ""
	replicas := 0
	running := ""
	instances := ""
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
	if vm.Replicas != nil {
		replicas = *vm.Replicas
	}
	if vm.Running != nil {
		running = strconv.FormatBool(*vm.Running)
	}
	if vm.Instances != nil {
		instances = strings.Join(*vm.Instances, ", ")
	}
	if vm.CreatedAt != nil {
		createdAt = vm.CreatedAt.String()
	}

	t.AppendRow(table.Row{"Name", name})
	t.AppendRow(table.Row{"Vpc", vpc})
	t.AppendRow(table.Row{"Subnet", subnet})
	displayVirtualMachineSpecForSingle(t, vm.Spec)
	t.AppendRow(table.Row{"Replicas", replicas})
	t.AppendRow(table.Row{"Running", running})
	t.AppendRow(table.Row{"Instances", instances})
	t.AppendRow(table.Row{"CreatedAt", createdAt})

	t.Render()

	return nil
}

func displayMultipleVirtualMachinePool(c *cli.Context, vms []mni_vm.VirtualMachinePool) error {
	t := table.NewWriter()
	t.SetOutputMirror(c.App.Writer)

	t.AppendHeader(table.Row{"Name", "Vpc", "Subnet", "Running", "Instances"})

	for _, vm := range vms {

		name := ""
		vpc := ""
		subnet := ""
		running := ""
		instances := ""

		if vm.Name != nil {
			name = *vm.Name
		}
		if vm.Vpc != nil {
			vpc = *vm.Vpc
		}
		if vm.Subnet != nil {
			subnet = *vm.Subnet
		}
		if vm.Running != nil {
			running = strconv.FormatBool(*vm.Running)
		}
		if vm.Instances != nil {
			instances = strings.Join(*vm.Instances, ", ")
		}

		t.AppendRow(table.Row{name, vpc, subnet, running, instances})
	}

	t.Render()

	return nil
}
