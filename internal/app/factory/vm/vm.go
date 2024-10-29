package vm

import (
	"bufio"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/mNi-Cloud/backend/common/pkg/mni/apigen/model"
	"github.com/mNi-Cloud/backend/vm/api/v1alpha1/vm"
	"github.com/mNi-Cloud/backend/vm/pkg/client"
	"github.com/mNi-Cloud/cli/internal/pkg/factory"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

type VirtualMachineCommandFactory struct {
	factory.CommandFactory
}

func (c VirtualMachineCommandFactory) Command(name string) *cli.Command {
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

			res, err := vmClient.V1Alpha1().VirtualMachines().StartVmWithResponse(ctx.Context, ctx.Args().First(), &vm.StartVmParams{Authorization: "Bearer " + ctx.String("token")})
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
				fmt.Println("Vm started successfully")
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

			res, err := vmClient.V1Alpha1().VirtualMachines().StopVmWithResponse(ctx.Context, ctx.Args().First(), &vm.StopVmParams{Authorization: "Bearer " + ctx.String("token")})
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
				fmt.Println("Vm stopped successfully")
				return nil
			}
		},
	}, &cli.Command{
		Name:      "reboot",
		Before:    factory.TokenFunc(),
		ArgsUsage: "<name>",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() != 1 {
				cli.ShowSubcommandHelpAndExit(ctx, 1)
				return nil
			}

			vmClient := client.NewClient(ctx.String("vm-endpoint"))

			res, err := vmClient.V1Alpha1().VirtualMachines().SoftRebootVmWithResponse(ctx.Context, ctx.Args().First(), &vm.SoftRebootVmParams{Authorization: "Bearer " + ctx.String("token")})
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
				fmt.Println("Vm rebooted successfully")
				return nil
			}
		},
	}, &cli.Command{
		Name:      "serial",
		Before:    factory.TokenFunc(),
		ArgsUsage: "<name>",
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
	}, &cli.Command{
		Name:      "vnc",
		Before:    factory.TokenFunc(),
		ArgsUsage: "<name>",
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
	})
	return command
}

func NewVirtualMachineCommandFactory(model *model.Model) factory.CommandFactory {
	return &VirtualMachineCommandFactory{
		CommandFactory: factory.NewRestCommandFactory("vm-endpoint", "v1alpha1", "vms", model),
	}
}
