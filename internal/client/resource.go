package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/unstructured"
)

const (
	dependenciesPath = "/dependencies"
	dependentsPath   = "/dependents"
)

// PatchType is how api-gateway is asked to merge a partial object.
type PatchType int

const (
	MergePatchType PatchType = iota
	ApplyPatchType
)

func (p PatchType) contentType() (string, error) {
	switch p {
	case MergePatchType:
		return "application/merge-patch+json", nil
	case ApplyPatchType:
		return "application/apply-patch+json", nil
	default:
		return "", fmt.Errorf("unknown patch type %d", p)
	}
}

// ResourceClient reads and writes objects of one kind.
type ResourceClient interface {
	List(ctx context.Context) (unstructured.UnstructuredList, error)
	Get(ctx context.Context, name string) (unstructured.Unstructured, error)
	Create(ctx context.Context, object unstructured.Unstructured) (unstructured.Unstructured, error)
	Update(ctx context.Context, name string, object unstructured.Unstructured) (unstructured.Unstructured, error)
	Patch(ctx context.Context, name string, patchType PatchType, object unstructured.Unstructured) (unstructured.Unstructured, error)
	Delete(ctx context.Context, name string) error
	Dependencies(ctx context.Context, name string) ([]api.Dependency, error)
	Dependents(ctx context.Context, name string) ([]api.Dependency, error)
}

type resourceClient struct {
	baseURL    string
	httpClient *http.Client
	resource   api.APIResource
	tenant     string
}

func (r *resourceClient) List(ctx context.Context) (unstructured.UnstructuredList, error) {
	return get[unstructured.UnstructuredList](ctx, r.httpClient, r.collectionURL())
}

func (r *resourceClient) Get(ctx context.Context, name string) (unstructured.Unstructured, error) {
	target, err := r.objectURL(name)
	if err != nil {
		return nil, err
	}
	return get[unstructured.Unstructured](ctx, r.httpClient, target)
}

func (r *resourceClient) Create(ctx context.Context, object unstructured.Unstructured) (unstructured.Unstructured, error) {
	return send[unstructured.Unstructured](ctx, r.httpClient, http.MethodPost, r.collectionURL(), "application/json", object)
}

func (r *resourceClient) Update(ctx context.Context, name string, object unstructured.Unstructured) (unstructured.Unstructured, error) {
	target, err := r.objectURL(name)
	if err != nil {
		return nil, err
	}
	return send[unstructured.Unstructured](ctx, r.httpClient, http.MethodPut, target, "application/json", object)
}

func (r *resourceClient) Patch(ctx context.Context, name string, patchType PatchType, object unstructured.Unstructured) (unstructured.Unstructured, error) {
	contentType, err := patchType.contentType()
	if err != nil {
		return nil, err
	}
	target, err := r.objectURL(name)
	if err != nil {
		return nil, err
	}
	return send[unstructured.Unstructured](ctx, r.httpClient, http.MethodPatch, target, contentType, object)
}

func (r *resourceClient) Delete(ctx context.Context, name string) error {
	target, err := r.objectURL(name)
	if err != nil {
		return err
	}
	_, err = remove[any](ctx, r.httpClient, target)
	return err
}

// Dependencies lists the resources api-gateway reports this one as needing.
func (r *resourceClient) Dependencies(ctx context.Context, name string) ([]api.Dependency, error) {
	target, err := r.objectURL(name)
	if err != nil {
		return nil, err
	}
	return get[[]api.Dependency](ctx, r.httpClient, target+dependenciesPath)
}

// Dependents lists the resources api-gateway reports as depending on this one.
// The server names only the ones that depend on it directly.
func (r *resourceClient) Dependents(ctx context.Context, name string) ([]api.Dependency, error) {
	target, err := r.objectURL(name)
	if err != nil {
		return nil, err
	}
	return get[[]api.Dependency](ctx, r.httpClient, target+dependentsPath)
}

// collectionURL builds the path api-gateway serves this resource on. A tenant
// only appears for a namespaced resource; namespaces stay inside the gateway.
func (r *resourceClient) collectionURL() string {
	path := r.baseURL + "/" + r.resource.Group + "/" + r.resource.Version
	if r.resource.Namespaced() {
		path += "/tenants/" + url.PathEscape(r.tenant)
	}
	return path + "/" + r.resource.Resource
}

func (r *resourceClient) objectURL(name string) (string, error) {
	if name == "" {
		return "", errors.New("no name to address a " + r.resource.Kind + " with")
	}
	return r.collectionURL() + "/" + url.PathEscape(name), nil
}
