package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/client"
	"github.com/mNi-Cloud/cli/internal/unstructured"
	"github.com/urfave/cli/v3"
)

func applyCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:  "apply",
		Usage: "Apply manifests",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Usage:    "Path to a YAML manifest file (it may hold several documents)",
				Required: true,
			},
		},
		Before: deps.RequireLogin,
		Action: deps.Apply,
	}
}

// Apply creates the resources of a manifest, or patches the ones that are
// already there.
func (d *Deps) Apply(ctx context.Context, cmd *cli.Command) error {
	path := cmd.String("file")
	if path == "" {
		return errors.New("mni apply needs a manifest (usage: mni apply -f <file>)")
	}

	objects, err := readManifest(path)
	if err != nil {
		return err
	}

	for _, object := range objects {
		if err := d.applyOne(ctx, object); err != nil {
			return err
		}
	}
	return nil
}

func (d *Deps) applyOne(ctx context.Context, object unstructured.Unstructured) error {
	group, version, err := api.SplitAPIVersion(object.APIVersion())
	if err != nil {
		return err
	}
	kind := object.Kind()
	name := object.Name()
	if kind == "" {
		return errors.New("a document in the manifest has no kind")
	}
	if name == "" {
		return fmt.Errorf("the %s in the manifest has no metadata.name", kind)
	}

	apiClient, err := d.Client()
	if err != nil {
		return err
	}
	catalog, err := apiClient.APIResources(ctx)
	if err != nil {
		return err
	}
	resource, ok := catalog.FindByKind(group, version, kind)
	if !ok {
		return fmt.Errorf("this server serves no %s in %s/%s", kind, group, version)
	}

	resourceClient, err := d.ResourceFor(resource)
	if err != nil {
		return err
	}

	// Only a 404 means the resource has to be created. Any other failure is
	// reported, because creating on top of a network failure would be a guess.
	_, err = resourceClient.Get(ctx, name)
	switch {
	case err == nil:
		if _, err := resourceClient.Patch(ctx, name, client.ApplyPatchType, object); err != nil {
			return err
		}
		fmt.Fprintf(d.Out, "%s/%s updated\n", resource.Resource, name)
		return nil

	case api.IsNotFound(err):
		if _, err := resourceClient.Create(ctx, object); err != nil {
			return err
		}
		fmt.Fprintf(d.Out, "%s/%s created\n", resource.Resource, name)
		return nil

	default:
		return err
	}
}

func readManifest(path string) ([]unstructured.Unstructured, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read the manifest %s: %w", path, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(content))
	objects := []unstructured.Unstructured{}
	for {
		var raw map[string]any
		if err := decoder.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("cannot read the manifest %s: %w", path, err)
		}
		if raw == nil {
			continue
		}
		objects = append(objects, unstructured.Unstructured(raw))
	}

	if len(objects) == 0 {
		return nil, fmt.Errorf("the manifest %s holds no document", path)
	}
	return objects, nil
}
