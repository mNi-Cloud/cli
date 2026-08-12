package client

import (
	"context"
	"fmt"

	"github.com/mNi-Cloud/cli/internal/api"
)

// Dependents lists everything that deleting one resource carries with it.
//
// api-gateway answers with the resources that depend on a resource directly.
// mNi Cloud deletes those along with their own dependents, so the walk follows
// the chain to its end and reports the whole set.
func (c *Client) Dependents(ctx context.Context, resource api.APIResource, tenant, name string) ([]api.Dependency, error) {
	catalog, err := c.APIResources(ctx)
	if err != nil {
		return nil, err
	}

	walk := &dependentWalk{
		client:  c,
		catalog: catalog,
		tenant:  tenant,
		seen:    map[api.Dependency]bool{},
	}
	if err := walk.from(ctx, resource, name); err != nil {
		return nil, err
	}
	return walk.found, nil
}

type dependentWalk struct {
	client  *Client
	catalog api.APIResourceList
	tenant  string
	seen    map[api.Dependency]bool
	found   []api.Dependency
}

func (w *dependentWalk) from(ctx context.Context, resource api.APIResource, name string) error {
	resourceClient, err := w.client.Resource(resource, w.tenant)
	if err != nil {
		return err
	}

	direct, err := resourceClient.Dependents(ctx, name)
	if err != nil {
		return err
	}

	for _, dependent := range direct {
		if w.seen[dependent] {
			continue
		}
		w.seen[dependent] = true
		w.found = append(w.found, dependent)

		next, err := w.resourceOf(dependent)
		if err != nil {
			return err
		}
		if err := w.from(ctx, next, dependent.Name); err != nil {
			return err
		}
	}
	return nil
}

// resourceOf finds the catalog entry a dependent belongs to. A dependent the
// catalog does not name cannot be followed, and stopping there would under
// report what a delete removes, so it is an error.
func (w *dependentWalk) resourceOf(dependent api.Dependency) (api.APIResource, error) {
	group, version, err := dependent.GroupVersion()
	if err != nil {
		return api.APIResource{}, err
	}

	resource, ok := w.catalog.FindByKind(group, version, dependent.Kind)
	if !ok {
		return api.APIResource{}, fmt.Errorf("cannot list what depends on %s: this server serves no %s in %s/%s", dependent, dependent.Kind, group, version)
	}
	return resource, nil
}
