// Package config reads and writes the CLI profile: the contexts a user can
// address and the tokens that belong to them. Tokens live in their own file so
// that the config can be shared while the secrets stay private.
package config

import (
	"errors"
	"fmt"
)

var (
	// ErrNoCurrentContext reports that nothing tells the CLI which server to talk to.
	ErrNoCurrentContext = errors.New("no current context is set: run `mni login` or `mni config use-context <name>`")
	// ErrNoTenant reports that neither the flag nor the context names a tenant.
	ErrNoTenant = errors.New("no tenant is set: pass --tenant or run `mni config use-tenant <name>`")
)

// ContextNotFoundError reports a context name that the config does not hold.
type ContextNotFoundError struct {
	Name string
}

func (e *ContextNotFoundError) Error() string {
	return fmt.Sprintf("context %q is not configured", e.Name)
}

// Config is the whole profile file.
type Config struct {
	CurrentContext string    `yaml:"currentContext"`
	Contexts       []Context `yaml:"contexts"`
}

// Context is one server the CLI can address.
type Context struct {
	Name                  string `yaml:"name"`
	Server                string `yaml:"server"`
	Tenant                string `yaml:"tenant,omitempty"`
	CACert                string `yaml:"caCert,omitempty"`
	InsecureSkipTLSVerify bool   `yaml:"insecureSkipTLSVerify,omitempty"`
	OAuth                 OAuth  `yaml:"oauth"`
}

// OAuth is the public client registration the CLI logs in with.
type OAuth struct {
	Issuer      string `yaml:"issuer"`
	ClientID    string `yaml:"clientID"`
	RedirectURI string `yaml:"redirectURI"`
}

// Validate reports the fields a context needs before it can be used to log in.
func (c Context) Validate() error {
	missing := []string{}
	if c.Name == "" {
		missing = append(missing, "name")
	}
	if c.Server == "" {
		missing = append(missing, "server")
	}
	if c.OAuth.Issuer == "" {
		missing = append(missing, "oauth.issuer")
	}
	if c.OAuth.ClientID == "" {
		missing = append(missing, "oauth.clientID")
	}
	if c.OAuth.RedirectURI == "" {
		missing = append(missing, "oauth.redirectURI")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("context %q is missing %v", c.Name, missing)
}

// Find returns the context with the given name.
func (c *Config) Find(name string) (Context, bool) {
	for _, ctx := range c.Contexts {
		if ctx.Name == name {
			return ctx, true
		}
	}
	return Context{}, false
}

// Put adds a context or replaces the one that already carries its name.
func (c *Config) Put(ctx Context) {
	for i := range c.Contexts {
		if c.Contexts[i].Name == ctx.Name {
			c.Contexts[i] = ctx
			return
		}
	}
	c.Contexts = append(c.Contexts, ctx)
}

// Remove drops a context and reports whether it was there.
func (c *Config) Remove(name string) bool {
	for i := range c.Contexts {
		if c.Contexts[i].Name != name {
			continue
		}
		c.Contexts = append(c.Contexts[:i], c.Contexts[i+1:]...)
		if c.CurrentContext == name {
			c.CurrentContext = ""
		}
		return true
	}
	return false
}

// SetCurrent points the config at a context it already holds.
func (c *Config) SetCurrent(name string) error {
	if _, ok := c.Find(name); !ok {
		return &ContextNotFoundError{Name: name}
	}
	c.CurrentContext = name
	return nil
}

// Resolve returns the named context, or the current one when name is empty.
func (c *Config) Resolve(name string) (Context, error) {
	if name == "" {
		if c.CurrentContext == "" {
			return Context{}, ErrNoCurrentContext
		}
		name = c.CurrentContext
	}

	ctx, ok := c.Find(name)
	if !ok {
		return Context{}, &ContextNotFoundError{Name: name}
	}
	return ctx, nil
}

// ResolveTenant returns the tenant to address, preferring the command line.
func ResolveTenant(ctx Context, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if ctx.Tenant != "" {
		return ctx.Tenant, nil
	}
	return "", ErrNoTenant
}
