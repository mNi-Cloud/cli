package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/mNi-Cloud/cli/internal/client"
	"github.com/mNi-Cloud/cli/internal/console"
	"github.com/urfave/cli/v3"
)

const (
	// containers is the resource the ctr subcommands address. The catalog of the
	// server names its group and version.
	containers = "containers"

	logsSubresource = "logs"
	execSubresource = "exec"

	logsArgsUsage = "<name>"
	execArgsUsage = "<name> -- <command> [args...]"

	followFlagName     = "follow"
	tailFlagName       = "tail"
	timestampsFlagName = "timestamps"
	previousFlagName   = "previous"
	sinceFlagName      = "since"

	stdinFlagName = "stdin"
	ttyFlagName   = "tty"

	// commandArgName holds the words of the command to run, which the user types
	// after a bare `--` so that the flags of the command are its own.
	commandArgName = "command"
)

func ctrCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:   "ctr",
		Usage:  "Operate a container",
		Before: deps.RequireLogin,
		Commands: []*cli.Command{
			{
				Name:      logsSubresource,
				Usage:     "Read the logs of a container",
				ArgsUsage: logsArgsUsage,
				Arguments: []cli.Argument{&cli.StringArg{Name: "name"}},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    followFlagName,
						Aliases: []string{"f"},
						Usage:   "Keep reading as the container writes",
					},
					&cli.IntFlag{
						Name:  tailFlagName,
						Usage: "Read only the last lines, or all of them when this is not given",
					},
					&cli.BoolFlag{
						Name:  timestampsFlagName,
						Usage: "Write the time every line was logged at",
					},
					&cli.BoolFlag{
						Name:  previousFlagName,
						Usage: "Read the logs of the run before this one",
					},
					&cli.StringFlag{
						Name:  sinceFlagName,
						Usage: "Read only what was logged in the last stretch of time, such as 5m",
					},
				},
				Action: deps.ContainerLogs,
			},
			{
				Name:      execSubresource,
				Usage:     "Run a command in a container",
				ArgsUsage: execArgsUsage,
				Arguments: []cli.Argument{
					&cli.StringArg{Name: "name"},
					// A command is a whole line, so every word left over belongs
					// to it.
					&cli.StringArgs{Name: commandArgName, Max: -1},
				},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    stdinFlagName,
						Aliases: []string{"i"},
						Usage:   "Send what the user types to the command",
					},
					&cli.BoolFlag{
						Name:    ttyFlagName,
						Aliases: []string{"t"},
						Usage:   "Give the command a terminal, which needs --stdin",
					},
				},
				Action: deps.ContainerExec,
			},
		},
	}
}

// ContainerLogs writes the logs of a container the way the server streams them.
// The stream is opened whether or not the user follows the logs, and the server
// leaves once it has written the last line.
func (d *Deps) ContainerLogs(ctx context.Context, cmd *cli.Command) error {
	name, err := containerName(cmd, logsSubresource, logsArgsUsage)
	if err != nil {
		return err
	}

	query, err := logsQuery(cmd)
	if err != nil {
		return err
	}

	subresource, err := d.SubresourceFor(ctx, containers, name, logsSubresource, client.WithQuery(query))
	if err != nil {
		return err
	}
	stream, err := subresource.Connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = stream.Close() }()

	return console.Copy(ctx, d.Out, stream)
}

// ContainerExec runs one command in a container and ends the way the command
// ended, so that a script reads the result the way it reads a command run on
// this machine.
func (d *Deps) ContainerExec(ctx context.Context, cmd *cli.Command) error {
	name, err := containerName(cmd, execSubresource, execArgsUsage)
	if err != nil {
		return err
	}

	argv := cmd.StringArgs(commandArgName)
	if len(argv) == 0 {
		return errors.New("mni ctr exec needs a command (usage: mni ctr exec " + execArgsUsage + ")")
	}

	stdin, tty := cmd.Bool(stdinFlagName), cmd.Bool(ttyFlagName)
	options, err := d.execOptions(stdin, tty)
	if err != nil {
		return err
	}

	subresource, err := d.SubresourceFor(ctx, containers, name, execSubresource,
		client.WithQuery(execQuery(argv, stdin, tty)))
	if err != nil {
		return err
	}
	stream, err := subresource.Connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = stream.Close() }()

	status, err := console.Exec(ctx, stream, options)
	if err != nil {
		return err
	}
	return commandEnded(status)
}

// commandEnded turns how a command ended into how this process ends. An exit
// code below zero is none of a command: the server sends one when the session
// broke before the command ended, and passing it on would reach the shell as
// 255, where nothing tells it apart from a command that ended with 255. Such a
// run is a failure of mni itself instead.
func commandEnded(status console.ExitStatus) error {
	switch {
	case status.ExitCode < 0 && status.Message == "":
		return console.ErrNoExitStatus
	case status.ExitCode < 0:
		return errors.New(status.Message)
	case status.ExitCode > 0:
		return cli.Exit(status.Message, status.ExitCode)
	default:
		return nil
	}
}

// execOptions reads what the user asked for into the ends of the command. Only
// a command that is given a terminal needs one here, because the terminal is put
// into raw mode and its size is followed; input on its own is as often a pipe as
// a person typing.
func (d *Deps) execOptions(stdin, tty bool) (console.ExecOptions, error) {
	options := console.ExecOptions{Stdout: d.Out, Stderr: d.ErrOut}
	if tty && !stdin {
		return options, errors.New("mni ctr exec --tty needs --stdin, because a terminal nobody can type on is not what the user meant")
	}
	if !stdin {
		return options, nil
	}

	if !tty {
		options.Stdin = d.In
		return options, nil
	}

	file, ok := terminalFile(d.In)
	if !ok {
		return options, fmt.Errorf("cannot give a command a terminal because %w", errNotATerminal)
	}

	terminal := console.File{File: file}
	options.Stdin = terminal
	options.Terminal = terminal
	return options, nil
}

// logsQuery reads the flags of the command into the parameters the server takes.
// A flag the user left alone is left out, so that the server keeps its own
// default.
func logsQuery(cmd *cli.Command) (url.Values, error) {
	query := url.Values{}
	for _, name := range []string{followFlagName, timestampsFlagName, previousFlagName} {
		if cmd.Bool(name) {
			query.Set(name, "true")
		}
	}

	if cmd.IsSet(tailFlagName) {
		lines := cmd.Int(tailFlagName)
		if lines < 0 {
			return nil, fmt.Errorf("%d is no number of lines to read", lines)
		}
		query.Set(tailFlagName, strconv.Itoa(lines))
	}

	if since := cmd.String(sinceFlagName); since != "" {
		if _, err := time.ParseDuration(since); err != nil {
			return nil, fmt.Errorf("%q is no length of time to read the logs of: %w", since, err)
		}
		query.Set(sinceFlagName, since)
	}

	return query, nil
}

// execQuery names the command to run and the ends it is given. Every word of
// the command is a parameter of its own, so that a word with a space in it
// stays one word.
func execQuery(argv []string, stdin, tty bool) url.Values {
	query := url.Values{}
	for _, word := range argv {
		query.Add(commandArgName, word)
	}
	if stdin {
		query.Set(stdinFlagName, "true")
	}
	if tty {
		query.Set(ttyFlagName, "true")
	}
	return query
}

func containerName(cmd *cli.Command, command, usage string) (string, error) {
	name := cmd.StringArg("name")
	if name == "" {
		return "", errors.New("mni ctr " + command + " needs a name (usage: mni ctr " + command + " " + usage + ")")
	}
	return name, nil
}
