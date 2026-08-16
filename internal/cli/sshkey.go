package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/sshkey"
	"github.com/mNi-Cloud/cli/internal/unstructured"
	"github.com/urfave/cli/v3"
)

const (
	// sshKeys is the resource the ssh-key subcommands address. The catalog of
	// the server names its group and version.
	sshKeys = "sshkeys"

	publicKeyFileArgName = "public-key-file"
	sshKeyAddArgsUsage   = "<name> <public-key-file>"
)

func sshKeyCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:   "ssh-key",
		Usage:  "Operate an SSH key for a virtual machine",
		Before: deps.RequireLogin,
		Commands: []*cli.Command{
			{
				Name:      "add",
				Usage:     "Register the public key a file on this machine holds",
				ArgsUsage: sshKeyAddArgsUsage,
				Arguments: []cli.Argument{
					&cli.StringArg{Name: "name"},
					&cli.StringArg{Name: publicKeyFileArgName},
				},
				Action: deps.AddSSHKey,
			},
		},
	}
}

// AddSSHKey registers the public key a file holds, so that nobody has to paste
// a key into a manifest by hand.
func (d *Deps) AddSSHKey(ctx context.Context, cmd *cli.Command) error {
	name := cmd.StringArg("name")
	path := cmd.StringArg(publicKeyFileArgName)
	if name == "" || path == "" {
		return errors.New("mni ssh-key add needs a name and a public key file (usage: mni ssh-key add " + sshKeyAddArgsUsage + ")")
	}

	// The file is read first, so that a key this machine can already refuse
	// costs no call to the server.
	publicKey, err := sshkey.ReadPublicKey(path)
	if err != nil {
		return err
	}

	resource, err := d.FindResource(ctx, sshKeys)
	if err != nil {
		return err
	}
	resourceClient, err := d.ResourceFor(resource)
	if err != nil {
		return err
	}

	if _, err := resourceClient.Create(ctx, sshKeyManifest(resource, name, publicKey)); err != nil {
		return err
	}

	fmt.Fprintf(d.Out, "%s/%s created\n", resource.Resource, name)
	return nil
}

func sshKeyManifest(resource api.APIResource, name, publicKey string) unstructured.Unstructured {
	return unstructured.Unstructured{
		"apiVersion": resource.APIVersion(),
		"kind":       resource.Kind,
		"metadata":   map[string]any{"name": name},
		"spec":       map[string]any{"publicKey": publicKey},
	}
}
