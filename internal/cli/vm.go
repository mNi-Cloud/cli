package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/mNi-Cloud/cli/internal/console"
	"github.com/urfave/cli/v3"
)

const (
	// virtualMachines is the resource the vm subcommands address. The catalog of
	// the server names its group and version.
	virtualMachines = "virtualmachines"

	serialSubresource = "serial"
	vncSubresource    = "vnc"

	portFlagName = "port"
	// tunnelHost keeps a console on this machine. A console carries the screen
	// and the keyboard of a machine, so it is never offered to the network.
	tunnelHost = "127.0.0.1"

	highestPort = 65535
)

// powerOperation is one operation the vm-controller serves as a subresource of
// a machine.
type powerOperation struct {
	name  string
	usage string
	done  string
}

var powerOperations = []powerOperation{
	{name: "start", usage: "Start a virtual machine", done: "started"},
	{name: "stop", usage: "Stop a virtual machine", done: "stopped"},
	{name: "restart", usage: "Restart a virtual machine", done: "restarted"},
}

func vmCommand(deps *Deps) *cli.Command {
	commands := make([]*cli.Command, 0, len(powerOperations)+2)
	for _, operation := range powerOperations {
		commands = append(commands, powerCommand(deps, operation))
	}
	commands = append(commands,
		&cli.Command{
			Name:      serialSubresource,
			Usage:     "Open the serial console of a virtual machine",
			ArgsUsage: "<name>",
			Arguments: []cli.Argument{&cli.StringArg{Name: "name"}},
			Action:    deps.SerialConsole,
		},
		&cli.Command{
			Name:      vncSubresource,
			Usage:     "Offer the graphical console of a virtual machine on a local port",
			ArgsUsage: "<name>",
			Arguments: []cli.Argument{&cli.StringArg{Name: "name"}},
			Flags: []cli.Flag{
				&cli.IntFlag{
					Name:  portFlagName,
					Usage: "Local port to offer the console on, or none to take a free one",
				},
			},
			Action: deps.VNCConsole,
		},
	)

	return &cli.Command{
		Name:     "vm",
		Usage:    "Operate a virtual machine",
		Before:   deps.RequireLogin,
		Commands: commands,
	}
}

func powerCommand(deps *Deps, operation powerOperation) *cli.Command {
	return &cli.Command{
		Name:      operation.name,
		Usage:     operation.usage,
		ArgsUsage: "<name>",
		Arguments: []cli.Argument{&cli.StringArg{Name: "name"}},
		Action:    deps.Power(operation),
	}
}

// Power runs one power operation on a machine.
func (d *Deps) Power(operation powerOperation) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		name, err := machineName(cmd, operation.name)
		if err != nil {
			return err
		}

		subresource, err := d.SubresourceFor(ctx, virtualMachines, name, operation.name)
		if err != nil {
			return err
		}
		if err := subresource.Post(ctx); err != nil {
			return err
		}

		fmt.Fprintf(d.Out, "%s/%s %s\n", virtualMachines, name, operation.done)
		return nil
	}
}

// SerialConsole joins the terminal to the serial console of a machine.
func (d *Deps) SerialConsole(ctx context.Context, cmd *cli.Command) error {
	name, err := machineName(cmd, serialSubresource)
	if err != nil {
		return err
	}

	terminal, ok := terminalFile(d.In)
	if !ok {
		return fmt.Errorf("cannot open a serial console because %w", errNotATerminal)
	}

	subresource, err := d.SubresourceFor(ctx, virtualMachines, name, serialSubresource)
	if err != nil {
		return err
	}
	stream, err := subresource.Connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = stream.Close() }()

	fmt.Fprintf(d.ErrOut, "Connected to the serial console of %s/%s. Press %s to leave it.\n",
		virtualMachines, name, console.EscapeName)

	if err := console.Attach(console.File{File: terminal}, d.Out, stream); err != nil {
		return err
	}

	fmt.Fprintf(d.ErrOut, "\nDisconnected from the serial console of %s/%s.\n", virtualMachines, name)
	return nil
}

// VNCConsole offers the graphical console of a machine on a local port, because
// its protocol is meant for a VNC client rather than for a terminal.
func (d *Deps) VNCConsole(ctx context.Context, cmd *cli.Command) error {
	name, err := machineName(cmd, vncSubresource)
	if err != nil {
		return err
	}

	address, err := tunnelAddress(cmd.Int(portFlagName))
	if err != nil {
		return err
	}

	subresource, err := d.SubresourceFor(ctx, virtualMachines, name, vncSubresource)
	if err != nil {
		return err
	}

	tunnel, err := console.Listen(address)
	if err != nil {
		return err
	}
	defer func() { _ = tunnel.Close() }()

	fmt.Fprintln(d.Out, tunnel.Address())
	fmt.Fprintf(d.ErrOut, "The graphical console of %s/%s is offered on %s. Point a VNC client at it, and press Ctrl-C to stop.\n",
		virtualMachines, name, tunnel.Address())

	return tunnel.Serve(ctx, func(ctx context.Context) (io.ReadWriteCloser, error) {
		return subresource.Connect(ctx)
	})
}

func machineName(cmd *cli.Command, command string) (string, error) {
	name := cmd.StringArg("name")
	if name == "" {
		return "", errors.New("mni vm " + command + " needs a name (usage: mni vm " + command + " <name>)")
	}
	return name, nil
}

func tunnelAddress(port int) (string, error) {
	if port < 0 || port > highestPort {
		return "", fmt.Errorf("%d is no port to offer a console on", port)
	}
	return net.JoinHostPort(tunnelHost, strconv.Itoa(port)), nil
}
