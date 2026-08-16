// Package client talks to the mNi Cloud api-gateway. Every call but the
// resource catalog carries an access token, and a call whose token cannot be
// produced fails instead of going out anonymously.
package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/mNi-Cloud/cli/internal/api"
)

const (
	apiResourcesPath = "/api-resources"
	mePath           = "/me"
	tenantsPath      = "/tenants"
	membersPath      = "/members"
)

// Options is what a client needs to address one server.
type Options struct {
	Server     string
	HTTPClient *http.Client
	WebSocket  *websocket.Dialer
	Tokens     TokenProvider
}

// Client is the api-gateway as this CLI sees it.
type Client struct {
	baseURL string

	// anonymous serves the catalog, which api-gateway publishes without a token.
	anonymous     *http.Client
	authenticated *http.Client

	// websocket and tokens open a streaming subresource. Its handshake carries
	// the token in a header it writes itself, so it cannot go through the round
	// tripper that authenticates the other calls.
	websocket *websocket.Dialer
	tokens    TokenProvider

	catalogOnce sync.Once
	catalog     api.APIResourceList
	catalogErr  error
}

// New builds a client over an HTTP client that already carries its TLS setup.
func New(options Options) (*Client, error) {
	if options.Server == "" {
		return nil, errors.New("no server to talk to")
	}
	if options.HTTPClient == nil {
		return nil, errors.New("no HTTP client to talk with")
	}
	if options.WebSocket == nil {
		return nil, errors.New("no WebSocket dialer to open a stream with")
	}
	if options.Tokens == nil {
		return nil, errors.New("no token provider to authenticate with")
	}

	return &Client{
		baseURL:       strings.TrimSuffix(options.Server, "/"),
		anonymous:     options.HTTPClient,
		authenticated: withBearerToken(options.HTTPClient, options.Tokens),
		websocket:     options.WebSocket,
		tokens:        options.Tokens,
	}, nil
}

// APIResources reads the resource catalog once and remembers it.
func (c *Client) APIResources(ctx context.Context) (api.APIResourceList, error) {
	c.catalogOnce.Do(func() {
		c.catalog, c.catalogErr = get[api.APIResourceList](ctx, c.anonymous, c.baseURL+apiResourcesPath)
	})
	return c.catalog, c.catalogErr
}

// Me reports the user behind the access token.
func (c *Client) Me(ctx context.Context) (api.Identity, error) {
	return get[api.Identity](ctx, c.authenticated, c.baseURL+mePath)
}

// Tenants lists the tenants the caller takes part in.
func (c *Client) Tenants(ctx context.Context) ([]api.Tenant, error) {
	return get[[]api.Tenant](ctx, c.authenticated, c.baseURL+tenantsPath)
}

// CreateTenant opens a tenant owned by the caller.
func (c *Client) CreateTenant(ctx context.Context, request api.NewTenant) (api.Tenant, error) {
	return send[api.Tenant](ctx, c.authenticated, http.MethodPost, c.baseURL+tenantsPath, "application/json", request)
}

// DeleteTenant removes a tenant and everything inside it.
func (c *Client) DeleteTenant(ctx context.Context, name string) error {
	target, err := c.tenantURL(name)
	if err != nil {
		return err
	}
	_, err = remove[any](ctx, c.authenticated, target)
	return err
}

// Members lists the members of a tenant, beside its owner.
func (c *Client) Members(ctx context.Context, tenant string) ([]api.Member, error) {
	target, err := c.tenantURL(tenant)
	if err != nil {
		return nil, err
	}
	return get[[]api.Member](ctx, c.authenticated, target+membersPath)
}

// AddMember lets another user into a tenant and returns the members it then has.
func (c *Client) AddMember(ctx context.Context, tenant string, request api.NewMember) ([]api.Member, error) {
	target, err := c.tenantURL(tenant)
	if err != nil {
		return nil, err
	}
	return send[[]api.Member](ctx, c.authenticated, http.MethodPost, target+membersPath, "application/json", request)
}

// RemoveMember takes a user out of a tenant and returns the members it then has.
func (c *Client) RemoveMember(ctx context.Context, tenant, user string) ([]api.Member, error) {
	target, err := c.tenantURL(tenant)
	if err != nil {
		return nil, err
	}
	if user == "" {
		return nil, errors.New("no user to remove from " + tenant)
	}
	return remove[[]api.Member](ctx, c.authenticated, target+membersPath+"/"+url.PathEscape(user))
}

func (c *Client) tenantURL(name string) (string, error) {
	if name == "" {
		return "", errors.New("no tenant to address")
	}
	return c.baseURL + tenantsPath + "/" + url.PathEscape(name), nil
}

// Resource addresses one kind of resource inside a tenant. A namespaced
// resource without a tenant is refused rather than guessed.
func (c *Client) Resource(resource api.APIResource, tenant string) (ResourceClient, error) {
	addressed, err := c.resourceClient(resource, tenant)
	if err != nil {
		return nil, err
	}
	return addressed, nil
}

// Subresource addresses a subresource of one object. api-gateway relays a
// subresource to the controller that serves the resource, so what it does is up
// to that controller.
func (c *Client) Subresource(resource api.APIResource, tenant, name, subresource string, options ...SubresourceOption) (SubresourceClient, error) {
	if subresource == "" {
		return nil, errors.New("no subresource to call on " + resource.Kind)
	}

	owner, err := c.resourceClient(resource, tenant)
	if err != nil {
		return nil, err
	}
	target, err := owner.objectURL(name)
	if err != nil {
		return nil, err
	}

	addressed := &subresourceClient{
		url:        target + "/" + url.PathEscape(subresource),
		httpClient: c.authenticated,
		dialer:     c.websocket,
		tokens:     c.tokens,
	}
	for _, option := range options {
		option(addressed)
	}
	return addressed, nil
}

func (c *Client) resourceClient(resource api.APIResource, tenant string) (*resourceClient, error) {
	if resource.Namespaced() && tenant == "" {
		return nil, errors.New("no tenant to address " + resource.Resource + " in")
	}

	return &resourceClient{
		baseURL:    c.baseURL,
		httpClient: c.authenticated,
		resource:   resource,
		tenant:     tenant,
	}, nil
}
