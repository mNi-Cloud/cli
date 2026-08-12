package cli

import (
	"context"
	"io"
	"testing"

	"github.com/mNi-Cloud/cli/internal/config"
	"github.com/urfave/cli/v3"
)

// parseLoginFlags runs `mni login` far enough to read the flags it was handed,
// without letting the login itself start.
func parseLoginFlags(t *testing.T, args ...string) *cli.Command {
	t.Helper()

	var parsed *cli.Command
	root := NewCommandFor("test", NewDeps(nil, io.Discard, io.Discard))
	login := root.Command("login")
	login.Action = func(_ context.Context, cmd *cli.Command) error {
		parsed = cmd
		return nil
	}

	if err := root.Run(context.Background(), append([]string{"mni", "login"}, args...)); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return parsed
}

// storedProfile is a profile that already holds a context of another server.
func storedProfile() *config.Config {
	profile := &config.Config{CurrentContext: "e2e"}
	profile.Put(config.Context{
		Name:   "e2e",
		Server: "https://console.test:8443/api",
		Tenant: "e2etest",
		CACert: "/tmp/ca.crt",
		OAuth: config.OAuth{
			Issuer:      "https://console.test:5514",
			ClientID:    "client-cli-sample",
			RedirectURI: "http://localhost:9876/callback",
		},
	})
	return profile
}

func TestLoginFlagsDefaultToTheProductionEndpoints(t *testing.T) {
	cmd := parseLoginFlags(t)

	tests := []struct {
		flag string
		want string
	}{
		{flag: "server", want: config.DefaultServer},
		{flag: "issuer", want: config.DefaultIssuer},
		{flag: "client-id", want: config.DefaultClientID},
		{flag: "redirect-uri", want: config.DefaultRedirectURI},
	}

	for _, tt := range tests {
		if got := cmd.String(tt.flag); got != tt.want {
			t.Errorf("--%s = %q, want %q", tt.flag, got, tt.want)
		}
	}
}

func TestFirstLoginWritesTheDefaultContext(t *testing.T) {
	target := loginTarget(&config.Config{}, config.DefaultContextName, parseLoginFlags(t))

	want := config.Context{
		Name:   config.DefaultContextName,
		Server: config.DefaultServer,
		OAuth: config.OAuth{
			Issuer:      config.DefaultIssuer,
			ClientID:    config.DefaultClientID,
			RedirectURI: config.DefaultRedirectURI,
		},
	}
	if target != want {
		t.Errorf("loginTarget() = %+v, want %+v", target, want)
	}
}

func TestFirstLoginTakesTheFlagsOverTheDefaults(t *testing.T) {
	cmd := parseLoginFlags(t,
		"--server", "https://console.test:8443/api",
		"--issuer", "https://console.test:5514",
		"--client-id", "client-cli-sample",
		"--redirect-uri", "http://localhost:1234/callback",
		"--ca-cert", "/tmp/ca.crt",
		"--insecure-skip-tls-verify",
	)

	target := loginTarget(&config.Config{}, "e2e", cmd)

	want := config.Context{
		Name:                  "e2e",
		Server:                "https://console.test:8443/api",
		CACert:                "/tmp/ca.crt",
		InsecureSkipTLSVerify: true,
		OAuth: config.OAuth{
			Issuer:      "https://console.test:5514",
			ClientID:    "client-cli-sample",
			RedirectURI: "http://localhost:1234/callback",
		},
	}
	if target != want {
		t.Errorf("loginTarget() = %+v, want %+v", target, want)
	}
}

func TestLoginLeavesAStoredContextAlone(t *testing.T) {
	profile := storedProfile()
	stored, _ := profile.Find("e2e")

	if target := loginTarget(profile, "e2e", parseLoginFlags(t)); target != stored {
		t.Errorf("loginTarget() = %+v, want the stored context %+v", target, stored)
	}
}

func TestLoginChangesOnlyWhatTheFlagsName(t *testing.T) {
	profile := storedProfile()
	want, _ := profile.Find("e2e")
	want.Server = "https://moved.test:8443/api"

	target := loginTarget(profile, "e2e", parseLoginFlags(t, "--server", want.Server))

	if target != want {
		t.Errorf("loginTarget() = %+v, want %+v", target, want)
	}
}

func TestLoginContextName(t *testing.T) {
	tests := []struct {
		name     string
		override string
		profile  *config.Config
		want     string
	}{
		{
			name:     "the flag names the context",
			override: "picked",
			profile:  storedProfile(),
			want:     "picked",
		},
		{
			name:    "a second login stays on the current context",
			profile: storedProfile(),
			want:    "e2e",
		},
		{
			name:    "a first login lands on the default context",
			profile: &config.Config{},
			want:    config.DefaultContextName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := loginContextName(tt.override, tt.profile); got != tt.want {
				t.Errorf("loginContextName() = %q, want %q", got, tt.want)
			}
		})
	}
}
