package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/goccy/go-yaml"
	"github.com/mNi-Cloud/cli/internal/unstructured"
	"github.com/urfave/cli/v3"
)

func editCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:      "edit",
		Usage:     "Edit a resource in an editor",
		ArgsUsage: "<resource> <name>",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "resource"},
			&cli.StringArg{Name: "name"},
		},
		Before: deps.RequireLogin,
		Action: deps.Edit,
	}
}

// Edit opens a resource in an editor and sends back what was saved.
func (d *Deps) Edit(ctx context.Context, cmd *cli.Command) error {
	resourceName := cmd.StringArg("resource")
	name := cmd.StringArg("name")
	if resourceName == "" || name == "" {
		return errors.New("mni edit needs a resource and a name (usage: mni edit <resource> <name>)")
	}

	resource, err := d.FindResource(ctx, resourceName)
	if err != nil {
		return err
	}
	resourceClient, err := d.ResourceFor(resource)
	if err != nil {
		return err
	}

	current, err := resourceClient.Get(ctx, name)
	if err != nil {
		return err
	}
	before, err := current.EncodeYAML()
	if err != nil {
		return fmt.Errorf("cannot encode the resource as YAML: %w", err)
	}

	after, err := editInEditor(before)
	if err != nil {
		return err
	}
	if after == before {
		fmt.Fprintf(d.Out, "%s/%s unchanged\n", resource.Resource, name)
		return nil
	}

	var edited map[string]any
	if err := yaml.Unmarshal([]byte(after), &edited); err != nil {
		return fmt.Errorf("cannot read the edited YAML: %w", err)
	}

	updated, err := resourceClient.Update(ctx, name, unstructured.Unstructured(edited))
	if err != nil {
		return err
	}

	fmt.Fprintf(d.Out, "%s/%s edited\n", resource.Resource, updated.Name())
	return nil
}

// editInEditor writes a document to a temporary file, opens it in the editor
// of the user, and returns what was saved.
func editInEditor(content string) (string, error) {
	file, err := os.CreateTemp("", "mni-edit-*.yaml")
	if err != nil {
		return "", fmt.Errorf("cannot create a temporary file: %w", err)
	}
	path := file.Name()
	defer os.Remove(path)

	if _, err := file.WriteString(content); err != nil {
		file.Close()
		return "", fmt.Errorf("cannot write the temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("cannot close the temporary file: %w", err)
	}

	editor := exec.Command("/bin/sh", "-c", editorCommand()+" "+path)
	editor.Stdin = os.Stdin
	editor.Stdout = os.Stdout
	editor.Stderr = os.Stderr
	if err := editor.Run(); err != nil {
		return "", fmt.Errorf("cannot run the editor: %w", err)
	}

	edited, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read the edited file: %w", err)
	}
	return string(edited), nil
}

func editorCommand() string {
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	return "vi"
}
