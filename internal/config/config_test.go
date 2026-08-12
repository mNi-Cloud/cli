package config

import (
	"errors"
	"strings"
	"testing"
)

func sampleContext(name string) Context {
	return Context{
		Name:   name,
		Server: "https://api.example.com",
		Tenant: "e2etest",
		OAuth: OAuth{
			Issuer:      "https://auth.example.com",
			ClientID:    "client-cli-sample",
			RedirectURI: "http://localhost:9876/callback",
		},
	}
}

func TestConfigPutAddsAndReplaces(t *testing.T) {
	cfg := &Config{}

	cfg.Put(sampleContext("e2e"))
	if len(cfg.Contexts) != 1 {
		t.Fatalf("Contexts length = %d, want 1", len(cfg.Contexts))
	}

	updated := sampleContext("e2e")
	updated.Tenant = "other"
	cfg.Put(updated)

	if len(cfg.Contexts) != 1 {
		t.Fatalf("Contexts length = %d after replacing, want 1", len(cfg.Contexts))
	}
	if cfg.Contexts[0].Tenant != "other" {
		t.Errorf("Tenant = %q, want %q", cfg.Contexts[0].Tenant, "other")
	}
}

func TestConfigPutKeepsOrder(t *testing.T) {
	cfg := &Config{}
	cfg.Put(sampleContext("a"))
	cfg.Put(sampleContext("b"))
	cfg.Put(sampleContext("a"))

	if cfg.Contexts[0].Name != "a" || cfg.Contexts[1].Name != "b" {
		t.Errorf("context order = %q, want [a b]", []string{cfg.Contexts[0].Name, cfg.Contexts[1].Name})
	}
}

func TestConfigRemove(t *testing.T) {
	cfg := &Config{CurrentContext: "a"}
	cfg.Put(sampleContext("a"))
	cfg.Put(sampleContext("b"))

	if !cfg.Remove("a") {
		t.Fatal("Remove(\"a\") = false, want true")
	}
	if len(cfg.Contexts) != 1 || cfg.Contexts[0].Name != "b" {
		t.Errorf("Contexts = %+v, want only b", cfg.Contexts)
	}
	if cfg.CurrentContext != "" {
		t.Errorf("CurrentContext = %q, want it cleared", cfg.CurrentContext)
	}
	if cfg.Remove("a") {
		t.Error("Remove(\"a\") = true on a context that is gone")
	}
}

func TestConfigResolveUsesCurrentContext(t *testing.T) {
	cfg := &Config{CurrentContext: "b"}
	cfg.Put(sampleContext("a"))
	cfg.Put(sampleContext("b"))

	resolved, err := cfg.Resolve("")
	if err != nil {
		t.Fatalf("Resolve(\"\") error = %v", err)
	}
	if resolved.Name != "b" {
		t.Errorf("Resolve(\"\") = %q, want %q", resolved.Name, "b")
	}
}

func TestConfigResolveOverride(t *testing.T) {
	cfg := &Config{CurrentContext: "b"}
	cfg.Put(sampleContext("a"))
	cfg.Put(sampleContext("b"))

	resolved, err := cfg.Resolve("a")
	if err != nil {
		t.Fatalf("Resolve(\"a\") error = %v", err)
	}
	if resolved.Name != "a" {
		t.Errorf("Resolve(\"a\") = %q, want %q", resolved.Name, "a")
	}
}

func TestConfigResolveWithoutCurrentContext(t *testing.T) {
	cfg := &Config{}
	cfg.Put(sampleContext("a"))

	if _, err := cfg.Resolve(""); !errors.Is(err, ErrNoCurrentContext) {
		t.Errorf("Resolve(\"\") error = %v, want ErrNoCurrentContext", err)
	}
}

func TestConfigResolveUnknownContext(t *testing.T) {
	cfg := &Config{CurrentContext: "a"}
	cfg.Put(sampleContext("a"))

	_, err := cfg.Resolve("nope")
	if err == nil {
		t.Fatal("Resolve(\"nope\") error = nil, want an error")
	}
	var notFound *ContextNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("Resolve(\"nope\") error = %v, want a ContextNotFoundError", err)
	}
	if notFound.Name != "nope" {
		t.Errorf("Name = %q, want %q", notFound.Name, "nope")
	}
}

func TestConfigResolveCurrentContextThatIsGone(t *testing.T) {
	cfg := &Config{CurrentContext: "gone"}
	cfg.Put(sampleContext("a"))

	if _, err := cfg.Resolve(""); err == nil {
		t.Fatal("Resolve(\"\") error = nil, want an error")
	}
}

func TestResolveTenantPrefersOverride(t *testing.T) {
	got, err := ResolveTenant(sampleContext("e2e"), "flagged")
	if err != nil {
		t.Fatalf("ResolveTenant() error = %v", err)
	}
	if got != "flagged" {
		t.Errorf("ResolveTenant() = %q, want %q", got, "flagged")
	}
}

func TestResolveTenantFallsBackToContext(t *testing.T) {
	got, err := ResolveTenant(sampleContext("e2e"), "")
	if err != nil {
		t.Fatalf("ResolveTenant() error = %v", err)
	}
	if got != "e2etest" {
		t.Errorf("ResolveTenant() = %q, want %q", got, "e2etest")
	}
}

func TestResolveTenantWithoutAnySource(t *testing.T) {
	ctx := sampleContext("e2e")
	ctx.Tenant = ""

	_, err := ResolveTenant(ctx, "")
	if !errors.Is(err, ErrNoTenant) {
		t.Fatalf("ResolveTenant() error = %v, want ErrNoTenant", err)
	}
}

func TestContextValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Context)
		wantErr string
	}{
		{name: "valid", mutate: func(*Context) {}},
		{name: "no name", mutate: func(c *Context) { c.Name = "" }, wantErr: "name"},
		{name: "no server", mutate: func(c *Context) { c.Server = "" }, wantErr: "server"},
		{name: "no issuer", mutate: func(c *Context) { c.OAuth.Issuer = "" }, wantErr: "issuer"},
		{name: "no client id", mutate: func(c *Context) { c.OAuth.ClientID = "" }, wantErr: "clientID"},
		{name: "no redirect uri", mutate: func(c *Context) { c.OAuth.RedirectURI = "" }, wantErr: "redirectURI"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := sampleContext("e2e")
			tt.mutate(&ctx)

			err := ctx.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want it to mention %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}
