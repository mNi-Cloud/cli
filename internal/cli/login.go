package cli

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/auth"
	"github.com/mNi-Cloud/cli/internal/client"
	"github.com/mNi-Cloud/cli/internal/config"
	"github.com/mNi-Cloud/cli/internal/output"
	"github.com/urfave/cli/v3"
)

func loginCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:  "login",
		Usage: "Sign in to a mNi Cloud server and remember the session",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Base URL of the api-gateway", Value: config.DefaultServer},
			&cli.StringFlag{Name: "issuer", Usage: "Base URL of the identity provider", Value: config.DefaultIssuer},
			&cli.StringFlag{Name: "client-id", Usage: "OAuth client ID registered for this CLI", Value: config.DefaultClientID},
			&cli.StringFlag{Name: "redirect-uri", Usage: "Loopback URL the browser is sent back to, where port 0 takes any free port", Value: config.DefaultRedirectURI},
			&cli.StringFlag{Name: "ca-cert", Usage: "PEM file of the CA that signed the server certificate"},
			&cli.BoolFlag{Name: "insecure-skip-tls-verify", Usage: "Do not check the certificate of the server"},
		},
		Action: deps.Login,
	}
}

// Login runs the browser flow and writes the context and its tokens down.
func (d *Deps) Login(ctx context.Context, cmd *cli.Command) error {
	store, err := d.Store()
	if err != nil {
		return err
	}
	cfg, err := d.Config()
	if err != nil {
		return err
	}

	name := loginContextName(d.ContextName, cfg)
	target := loginTarget(cfg, name, cmd)
	if err := target.Validate(); err != nil {
		return err
	}

	httpClient, err := newHTTPClient(target)
	if err != nil {
		return err
	}

	flow := &auth.Flow{HTTPClient: httpClient, Output: d.Out}
	token, err := flow.Run(ctx, auth.LoginRequest{
		Issuer:      target.OAuth.Issuer,
		ClientID:    target.OAuth.ClientID,
		RedirectURI: target.OAuth.RedirectURI,
		Scopes:      auth.DefaultScopes,
	})
	if err != nil {
		return err
	}

	if err := store.SaveCredential(auth.NewCredential(name, token)); err != nil {
		return err
	}

	cfg.Put(target)
	cfg.CurrentContext = name
	if err := store.SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "Signed in to context %q.\n", name)

	apiClient, err := newAPIClient(target, httpClient, newSession(target, store, httpClient))
	if err != nil {
		return err
	}
	return d.settleTenant(ctx, apiClient, cfg, target)
}

// loginContextName picks the context the login writes to: the one the command
// line names, the one that is current, or the context of mNi Cloud itself.
func loginContextName(override string, cfg *config.Config) string {
	if override != "" {
		return override
	}
	if cfg.CurrentContext != "" {
		return cfg.CurrentContext
	}
	return config.DefaultContextName
}

// loginTarget returns the context the login writes to. A context the profile
// already holds keeps what it holds, and a context that is new is built out of
// the command line.
func loginTarget(cfg *config.Config, name string, cmd *cli.Command) config.Context {
	target, found := cfg.Find(name)
	if !found {
		return newLoginContext(name, cmd)
	}

	applyLoginFlags(&target, cmd)
	return target
}

// newLoginContext builds a context out of the whole command line. The flags
// carry the endpoints of mNi Cloud, so a login that names nothing signs in
// there.
func newLoginContext(name string, cmd *cli.Command) config.Context {
	return config.Context{
		Name:                  name,
		Server:                cmd.String("server"),
		CACert:                cmd.String("ca-cert"),
		InsecureSkipTLSVerify: cmd.Bool("insecure-skip-tls-verify"),
		OAuth: config.OAuth{
			Issuer:      cmd.String("issuer"),
			ClientID:    cmd.String("client-id"),
			RedirectURI: cmd.String("redirect-uri"),
		},
	}
}

// applyLoginFlags overwrites only the fields the command line names, so that a
// second login can change one setting without clearing the rest.
func applyLoginFlags(target *config.Context, cmd *cli.Command) {
	if cmd.IsSet("server") {
		target.Server = cmd.String("server")
	}
	if cmd.IsSet("issuer") {
		target.OAuth.Issuer = cmd.String("issuer")
	}
	if cmd.IsSet("client-id") {
		target.OAuth.ClientID = cmd.String("client-id")
	}
	if cmd.IsSet("redirect-uri") {
		target.OAuth.RedirectURI = cmd.String("redirect-uri")
	}
	if cmd.IsSet("ca-cert") {
		target.CACert = cmd.String("ca-cert")
	}
	if cmd.IsSet("insecure-skip-tls-verify") {
		target.InsecureSkipTLSVerify = cmd.Bool("insecure-skip-tls-verify")
	}
}

// settleTenant points the context at a tenant when there is only one to pick.
// With several, the user chooses: picking one for them would silently decide
// where their next resource lands.
func (d *Deps) settleTenant(ctx context.Context, apiClient *client.Client, cfg *config.Config, target config.Context) error {
	tenants, err := apiClient.Tenants(ctx)
	if err != nil {
		return fmt.Errorf("signed in, but the tenants could not be listed: %w", err)
	}

	if len(tenants) == 0 {
		fmt.Fprintln(d.Out, "You are not a member of any tenant yet. Ask a tenant owner to add you.")
		return nil
	}

	if held := slices.IndexFunc(tenants, func(t api.Tenant) bool { return t.Name == target.Tenant }); held >= 0 {
		fmt.Fprintf(d.Out, "Using tenant %q.\n", target.Tenant)
		return nil
	}

	if len(tenants) > 1 {
		fmt.Fprintln(d.Out, "You take part in several tenants:")
		if err := printTenants(d.Out, tenants); err != nil {
			return err
		}
		fmt.Fprintln(d.Out, "\nPick one with `mni config use-tenant <name>`.")
		return nil
	}

	store, err := d.Store()
	if err != nil {
		return err
	}
	target.Tenant = tenants[0].Name
	cfg.Put(target)
	if err := store.SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "Tenant set to %q.\n", target.Tenant)
	return nil
}

func printTenants(out io.Writer, tenants []api.Tenant) error {
	table := output.NewWriter(out)
	table.WriteHeader("Name", "Role", "Phase")
	for _, tenant := range tenants {
		table.WriteRow(tenant.Name, tenant.Role, tenant.Phase)
	}
	return table.Flush()
}
