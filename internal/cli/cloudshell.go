package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mNi-Cloud/cli/internal/client"
	"github.com/mNi-Cloud/cli/internal/console"
	"github.com/urfave/cli/v3"
)

const cloudShellBootstrapChannel byte = 6

func cloudShellCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:    "cloudshell",
		Aliases: []string{"cs"},
		Usage:   "Operate CloudShell sessions",
		Before:  deps.RequireLogin,
		Commands: []*cli.Command{
			{Name: "list", Aliases: []string{"ls"}, Usage: "List CloudShell sessions", Action: deps.ListCloudShellSessions},
			{
				Name:      "create",
				Usage:     "Create a CloudShell session in a subnet",
				ArgsUsage: "<subnet>",
				Arguments: []cli.Argument{&cli.StringArg{Name: "subnet"}},
				Action:    deps.CreateCloudShellSession,
			},
			{
				Name:      "delete",
				Aliases:   []string{"rm"},
				Usage:     "Delete a CloudShell session",
				ArgsUsage: "<session>",
				Arguments: []cli.Argument{&cli.StringArg{Name: "session"}},
				Action:    deps.DeleteCloudShellSession,
			},
			{
				Name:      "shell",
				Usage:     "Open a CloudShell terminal",
				ArgsUsage: "<session>",
				Arguments: []cli.Argument{&cli.StringArg{Name: "session"}},
				Action:    deps.OpenCloudShell,
			},
		},
	}
}

func (d *Deps) ListCloudShellSessions(ctx context.Context, _ *cli.Command) error {
	apiClient, tenant, err := d.clientAndTenant()
	if err != nil {
		return err
	}
	sessions, err := apiClient.CloudShellSessions(ctx, tenant)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(struct {
		Sessions []client.CloudShellSession `json:"sessions"`
	}{Sessions: sessions})
	if err != nil {
		return err
	}
	return jsonIndent(d.Out, raw)
}

func (d *Deps) CreateCloudShellSession(ctx context.Context, cmd *cli.Command) error {
	subnet := cmd.StringArg("subnet")
	if subnet == "" {
		return errors.New("mni cloudshell create needs a subnet name")
	}
	apiClient, tenant, err := d.clientAndTenant()
	if err != nil {
		return err
	}
	session, err := apiClient.CreateCloudShellSession(ctx, tenant, subnet)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return jsonIndent(d.Out, raw)
}

func (d *Deps) DeleteCloudShellSession(ctx context.Context, cmd *cli.Command) error {
	session := cmd.StringArg("session")
	if session == "" {
		return errors.New("mni cloudshell delete needs a session ID")
	}
	apiClient, tenant, err := d.clientAndTenant()
	if err != nil {
		return err
	}
	if err := apiClient.DeleteCloudShellSession(ctx, tenant, session); err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "cloudshell/%s deleted\n", session)
	return nil
}

func (d *Deps) OpenCloudShell(ctx context.Context, cmd *cli.Command) error {
	sessionID := cmd.StringArg("session")
	if sessionID == "" {
		return errors.New("mni cloudshell shell needs a session ID")
	}
	terminalFile, outputFile, ok := terminalFiles(d.In, d.Out)
	if !ok {
		return fmt.Errorf("cannot open CloudShell because %w", errNotATerminal)
	}
	terminal := console.NewFile(terminalFile, outputFile)
	size, err := terminal.Size()
	if err != nil {
		return err
	}
	session, err := d.Session()
	if err != nil {
		return err
	}
	credential, err := session.Credential(ctx)
	if err != nil {
		return err
	}
	apiClient, tenant, err := d.clientAndTenant()
	if err != nil {
		return err
	}
	stream, err := apiClient.CloudShell(ctx, tenant, sessionID)
	if err != nil {
		return err
	}
	defer func() { _ = stream.Close() }()
	bootstrap, err := json.Marshal(struct {
		AccessToken string    `json:"accessToken"`
		ExpiresAt   time.Time `json:"expiresAt"`
		Width       uint16    `json:"width"`
		Height      uint16    `json:"height"`
	}{credential.AccessToken, credential.ExpiresAt, size.Width, size.Height})
	if err != nil {
		return err
	}
	if err := stream.WriteMessage(append([]byte{cloudShellBootstrapChannel}, bootstrap...)); err != nil {
		return err
	}
	fmt.Fprintf(d.ErrOut, "Connected to CloudShell %s.\n", sessionID)
	status, err := console.Exec(ctx, stream, console.ExecOptions{Stdin: terminal, Stdout: d.Out, Stderr: d.ErrOut, Terminal: terminal})
	if err != nil {
		return err
	}
	return commandEnded(status)
}

func (d *Deps) clientAndTenant() (*client.Client, string, error) {
	apiClient, err := d.Client()
	if err != nil {
		return nil, "", err
	}
	tenant, err := d.Tenant()
	if err != nil {
		return nil, "", err
	}
	return apiClient, tenant, nil
}
