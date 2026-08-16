// Package cli defines the commands of the mni binary and the actions behind
// them.
package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/auth"
	"github.com/mNi-Cloud/cli/internal/client"
	"github.com/mNi-Cloud/cli/internal/config"
	"github.com/mNi-Cloud/cli/internal/output"
	"github.com/urfave/cli/v3"
)

// requestTimeout bounds a single call to the gateway or the identity provider.
const requestTimeout = 60 * time.Second

// Deps builds what a command needs, and only when the command asks for it, so
// that `mni login` runs without a token and `mni config` runs without a server.
type Deps struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer

	// Interactive tells whether a question put to In can be answered. A command
	// that would otherwise wait forever asks for a flag instead.
	Interactive bool

	ContextName string
	TenantName  string

	store      *config.Store
	config     *config.Config
	context    *config.Context
	httpClient *http.Client
	session    *auth.Session
	apiClient  *client.Client
}

// NewDeps builds the dependencies of one run.
func NewDeps(in io.Reader, out, errOut io.Writer) *Deps {
	return &Deps{In: in, Out: out, ErrOut: errOut, Interactive: isTerminal(in)}
}

// Store returns the profile files of this machine.
func (d *Deps) Store() (*config.Store, error) {
	if d.store != nil {
		return d.store, nil
	}

	store, err := config.NewStore()
	if err != nil {
		return nil, err
	}
	d.store = store
	return store, nil
}

// Config returns the contexts on this machine.
func (d *Deps) Config() (*config.Config, error) {
	if d.config != nil {
		return d.config, nil
	}

	store, err := d.Store()
	if err != nil {
		return nil, err
	}
	cfg, err := store.LoadConfig()
	if err != nil {
		return nil, err
	}
	d.config = cfg
	return cfg, nil
}

// Context returns the context this run addresses.
func (d *Deps) Context() (config.Context, error) {
	if d.context != nil {
		return *d.context, nil
	}

	cfg, err := d.Config()
	if err != nil {
		return config.Context{}, err
	}
	resolved, err := cfg.Resolve(d.ContextName)
	if err != nil {
		return config.Context{}, err
	}
	d.context = &resolved
	return resolved, nil
}

// HTTPClient returns the HTTP client that carries the TLS setup of the context.
func (d *Deps) HTTPClient() (*http.Client, error) {
	if d.httpClient != nil {
		return d.httpClient, nil
	}

	ctx, err := d.Context()
	if err != nil {
		return nil, err
	}
	httpClient, err := newHTTPClient(ctx)
	if err != nil {
		return nil, err
	}
	d.httpClient = httpClient
	return httpClient, nil
}

// Session returns the token holder of the context. Every authenticated call of
// a run goes through the same one, so a token renewed once is renewed for all.
func (d *Deps) Session() (*auth.Session, error) {
	if d.session != nil {
		return d.session, nil
	}

	ctx, err := d.Context()
	if err != nil {
		return nil, err
	}
	store, err := d.Store()
	if err != nil {
		return nil, err
	}
	httpClient, err := d.HTTPClient()
	if err != nil {
		return nil, err
	}

	d.session = newSession(ctx, store, httpClient)
	return d.session, nil
}

// RequireLogin stops a command that cannot work without a token before it does
// anything else. It runs ahead of the action, so that a user who never logged
// in is sent to `mni login` instead of being asked for a tenant they cannot
// pick yet.
func (d *Deps) RequireLogin(ctx context.Context, _ *cli.Command) (context.Context, error) {
	session, err := d.Session()
	if err != nil {
		return ctx, err
	}
	if _, err := session.Token(ctx); err != nil {
		return ctx, err
	}
	return ctx, nil
}

// Client returns the api-gateway client of the context, already authenticated.
func (d *Deps) Client() (*client.Client, error) {
	if d.apiClient != nil {
		return d.apiClient, nil
	}

	ctx, err := d.Context()
	if err != nil {
		return nil, err
	}
	httpClient, err := d.HTTPClient()
	if err != nil {
		return nil, err
	}
	session, err := d.Session()
	if err != nil {
		return nil, err
	}

	apiClient, err := newAPIClient(ctx, httpClient, session)
	if err != nil {
		return nil, err
	}
	d.apiClient = apiClient
	return apiClient, nil
}

// Tenant returns the tenant this run addresses.
func (d *Deps) Tenant() (string, error) {
	ctx, err := d.Context()
	if err != nil {
		return "", err
	}
	return config.ResolveTenant(ctx, d.TenantName)
}

// Printer writes resources to the output stream of this run.
func (d *Deps) Printer() output.Printer {
	return output.NewPrinter(d.Out)
}

// FindResource looks a name typed on the command line up in the catalog.
func (d *Deps) FindResource(ctx context.Context, name string) (api.APIResource, error) {
	apiClient, err := d.Client()
	if err != nil {
		return api.APIResource{}, err
	}

	catalog, err := apiClient.APIResources(ctx)
	if err != nil {
		return api.APIResource{}, err
	}

	resource, ok := catalog.FindByName(name)
	if !ok {
		return api.APIResource{}, fmt.Errorf("this server serves no resource named %q", name)
	}
	return resource, nil
}

// ResourceFor addresses one kind of resource.
func (d *Deps) ResourceFor(resource api.APIResource) (client.ResourceClient, error) {
	apiClient, err := d.Client()
	if err != nil {
		return nil, err
	}
	tenant, err := d.tenantFor(resource)
	if err != nil {
		return nil, err
	}
	return apiClient.Resource(resource, tenant)
}

// SubresourceFor addresses a subresource of one object, by the name of its
// resource as the user types it.
func (d *Deps) SubresourceFor(ctx context.Context, resourceName, name, subresource string, options ...client.SubresourceOption) (client.SubresourceClient, error) {
	resource, err := d.FindResource(ctx, resourceName)
	if err != nil {
		return nil, err
	}
	apiClient, err := d.Client()
	if err != nil {
		return nil, err
	}
	tenant, err := d.tenantFor(resource)
	if err != nil {
		return nil, err
	}
	return apiClient.Subresource(resource, tenant, name, subresource, options...)
}

// Dependents lists what deleting one resource carries with it. mNi Cloud
// deletes a dependent along with the dependents of the dependent, so the whole
// chain is reported, not only what depends on the resource directly.
func (d *Deps) Dependents(ctx context.Context, resource api.APIResource, name string) ([]api.Dependency, error) {
	apiClient, err := d.Client()
	if err != nil {
		return nil, err
	}
	tenant, err := d.tenantFor(resource)
	if err != nil {
		return nil, err
	}
	return apiClient.Dependents(ctx, resource, tenant, name)
}

// tenantFor returns the tenant a resource is addressed in. Only a namespaced
// resource needs one, so a cluster scoped one works without any tenant set.
func (d *Deps) tenantFor(resource api.APIResource) (string, error) {
	if !resource.Namespaced() {
		return "", nil
	}
	return d.Tenant()
}

func newHTTPClient(ctx config.Context) (*http.Client, error) {
	return client.NewHTTPClient(client.TLSOptions{
		CACert:                ctx.CACert,
		InsecureSkipTLSVerify: ctx.InsecureSkipTLSVerify,
	}, requestTimeout)
}

func newSession(ctx config.Context, store *config.Store, httpClient *http.Client) *auth.Session {
	return auth.NewSession(ctx.Name, ctx.OAuth, store, httpClient)
}

func newAPIClient(ctx config.Context, httpClient *http.Client, tokens client.TokenProvider) (*client.Client, error) {
	dialer, err := client.NewWebSocketDialer(client.TLSOptions{
		CACert:                ctx.CACert,
		InsecureSkipTLSVerify: ctx.InsecureSkipTLSVerify,
	}, requestTimeout)
	if err != nil {
		return nil, err
	}

	return client.New(client.Options{
		Server:     ctx.Server,
		HTTPClient: httpClient,
		WebSocket:  dialer,
		Tokens:     tokens,
	})
}
