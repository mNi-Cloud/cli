package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// fakeProvider is an OIDC provider that answers the three calls a login makes.
type fakeProvider struct {
	server *httptest.Server

	authorizeQuery url.Values
	tokenForm      url.Values

	state  func(sent string) string
	issuer func(actual string) string
	deny   string
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()

	provider := &fakeProvider{
		state:  func(sent string) string { return sent },
		issuer: func(actual string) string { return actual },
	}

	mux := http.NewServeMux()
	provider.server = httptest.NewServer(mux)
	t.Cleanup(provider.server.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, discoveryDocument(provider.server.URL))
	})

	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		provider.authorizeQuery = r.URL.Query()

		redirect, err := url.Parse(r.URL.Query().Get("redirect_uri"))
		if err != nil {
			t.Errorf("redirect_uri is not a URL: %v", err)
			return
		}

		query := url.Values{}
		if provider.deny != "" {
			query.Set("error", provider.deny)
			query.Set("error_description", "the resource owner refused the request")
		} else {
			query.Set("code", "the-code")
		}
		query.Set("state", provider.state(r.URL.Query().Get("state")))
		query.Set("iss", provider.issuer(provider.server.URL))
		redirect.RawQuery = query.Encode()

		http.Redirect(w, r, redirect.String(), http.StatusFound)
	})

	mux.HandleFunc("/api/auth/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm() error = %v", err)
			return
		}
		provider.tokenForm = r.PostForm
		writeJSON(t, w, map[string]any{
			"access_token":  "access",
			"refresh_token": "refresh",
			"id_token":      "id",
			"token_type":    "Bearer",
			"scope":         strings.Join(DefaultScopes, " "),
			"expires_in":    86400,
		})
	})

	return provider
}

func freeLoopbackRedirect(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return "http://" + address + "/callback"
}

func (p *fakeProvider) newFlow(output *bytes.Buffer) *Flow {
	client := p.server.Client()
	return &Flow{
		HTTPClient: client,
		Output:     output,
		Now:        fixedNow,
		OpenBrowser: func(authorizationURL string) error {
			resp, err := client.Get(authorizationURL)
			if err != nil {
				return err
			}
			return resp.Body.Close()
		},
	}
}

func (p *fakeProvider) loginRequest(redirect string) LoginRequest {
	return LoginRequest{
		Issuer:      p.server.URL,
		ClientID:    "client-cli-sample",
		RedirectURI: redirect,
		Scopes:      DefaultScopes,
	}
}

func TestFlowRun(t *testing.T) {
	provider := newFakeProvider(t)
	redirect := freeLoopbackRedirect(t)
	output := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token, err := provider.newFlow(output).Run(ctx, provider.loginRequest(redirect))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if token.AccessToken != "access" || token.RefreshToken != "refresh" {
		t.Errorf("token = %+v, want the tokens the provider issued", token)
	}
	if wantExpiry := testNow.Add(86400 * time.Second); !token.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("ExpiresAt = %v, want %v", token.ExpiresAt, wantExpiry)
	}

	query := provider.authorizeQuery
	if got := query.Get("response_type"); got != "code" {
		t.Errorf("response_type = %q, want %q", got, "code")
	}
	if got := query.Get("client_id"); got != "client-cli-sample" {
		t.Errorf("client_id = %q, want %q", got, "client-cli-sample")
	}
	if got := query.Get("redirect_uri"); got != redirect {
		t.Errorf("redirect_uri = %q, want %q", got, redirect)
	}
	if got := query.Get("scope"); got != strings.Join(DefaultScopes, " ") {
		t.Errorf("scope = %q, want %q", got, strings.Join(DefaultScopes, " "))
	}
	if got := query.Get("code_challenge_method"); got != ChallengeMethodS256 {
		t.Errorf("code_challenge_method = %q, want %q", got, ChallengeMethodS256)
	}
	if query.Get("state") == "" {
		t.Error("state is empty, want a fresh one")
	}

	verifier := provider.tokenForm.Get("code_verifier")
	sum := sha256.Sum256([]byte(verifier))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); query.Get("code_challenge") != want {
		t.Errorf("code_challenge = %q, want the S256 of the verifier that was redeemed", query.Get("code_challenge"))
	}
	if got := provider.tokenForm.Get("redirect_uri"); got != redirect {
		t.Errorf("redeemed redirect_uri = %q, want %q", got, redirect)
	}

	if !strings.Contains(output.String(), provider.server.URL+"/auth") {
		t.Errorf("output = %q, want the authorization URL printed", output)
	}
}

// TestFlowRunWithADynamicPort checks that the port the OS handed out reaches
// both halves of the flow. RFC 6749 §4.1.3 has the token request repeat the
// redirect URI of the authorization request, so the two have to say the same.
func TestFlowRunWithADynamicPort(t *testing.T) {
	provider := newFakeProvider(t)
	output := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token, err := provider.newFlow(output).Run(ctx, provider.loginRequest("http://127.0.0.1:0/callback"))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if token.AccessToken != "access" {
		t.Errorf("AccessToken = %q, want the token the provider issued", token.AccessToken)
	}

	asked := provider.authorizeQuery.Get("redirect_uri")
	redirect, err := url.Parse(asked)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if redirect.Port() == "0" || redirect.Port() == "" {
		t.Errorf("redirect_uri = %q, want the port the login listens on", asked)
	}
	if redeemed := provider.tokenForm.Get("redirect_uri"); redeemed != asked {
		t.Errorf("redeemed redirect_uri = %q, want the one of the authorization request %q", redeemed, asked)
	}
}

func TestFlowRunRejectsAStateItDidNotSend(t *testing.T) {
	provider := newFakeProvider(t)
	provider.state = func(string) string { return "forged" }

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := provider.newFlow(&bytes.Buffer{}).Run(ctx, provider.loginRequest(freeLoopbackRedirect(t)))
	if err == nil {
		t.Fatal("Run() error = nil, want the forged state refused")
	}
	if !strings.Contains(err.Error(), "state") {
		t.Errorf("Run() error = %q, want it to mention the state", err)
	}
}

func TestFlowRunRejectsAnotherIssuer(t *testing.T) {
	provider := newFakeProvider(t)
	provider.issuer = func(string) string { return "https://elsewhere.test" }

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := provider.newFlow(&bytes.Buffer{}).Run(ctx, provider.loginRequest(freeLoopbackRedirect(t)))
	if err == nil {
		t.Fatal("Run() error = nil, want the wrong issuer refused")
	}
	if !strings.Contains(err.Error(), "issuer") {
		t.Errorf("Run() error = %q, want it to mention the issuer", err)
	}
}

func TestFlowRunReportsARefusedLogin(t *testing.T) {
	provider := newFakeProvider(t)
	provider.deny = "access_denied"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := provider.newFlow(&bytes.Buffer{}).Run(ctx, provider.loginRequest(freeLoopbackRedirect(t)))
	if err == nil {
		t.Fatal("Run() error = nil, want the refusal reported")
	}
	if !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("Run() error = %q, want it to carry the error code", err)
	}
}

func TestFlowRunKeepsGoingWhenNoBrowserOpens(t *testing.T) {
	provider := newFakeProvider(t)
	redirect := freeLoopbackRedirect(t)
	output := &bytes.Buffer{}

	client := provider.server.Client()
	flow := provider.newFlow(output)
	flow.OpenBrowser = func(authorizationURL string) error {
		go func() {
			if resp, err := client.Get(authorizationURL); err == nil {
				_ = resp.Body.Close()
			}
		}()
		return errors.New("no display")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := flow.Run(ctx, provider.loginRequest(redirect)); err != nil {
		t.Fatalf("Run() error = %v, want the login to finish without a browser", err)
	}
	if !strings.Contains(output.String(), provider.server.URL+"/auth") {
		t.Errorf("output = %q, want the URL printed so it can be opened by hand", output)
	}
}

func TestFlowRunStopsWhenTheContextEnds(t *testing.T) {
	provider := newFakeProvider(t)
	flow := provider.newFlow(&bytes.Buffer{})
	flow.OpenBrowser = func(string) error { return nil }

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := flow.Run(ctx, provider.loginRequest(freeLoopbackRedirect(t))); err == nil {
		t.Fatal("Run() error = nil, want the wait to be cut short")
	}
}

func TestFlowRunRejectsAnIncompleteRequest(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*LoginRequest)
	}{
		{name: "no issuer", mutate: func(r *LoginRequest) { r.Issuer = "" }},
		{name: "no client id", mutate: func(r *LoginRequest) { r.ClientID = "" }},
		{name: "no redirect uri", mutate: func(r *LoginRequest) { r.RedirectURI = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := LoginRequest{
				Issuer:      "https://issuer.test",
				ClientID:    "client",
				RedirectURI: "http://localhost:9876/callback",
				Scopes:      DefaultScopes,
			}
			tt.mutate(&request)

			flow := &Flow{Output: &bytes.Buffer{}, OpenBrowser: func(string) error { return nil }}
			if _, err := flow.Run(context.Background(), request); err == nil {
				t.Fatalf("Run(%s) error = nil, want an error", tt.name)
			}
		})
	}
}

func TestAuthorizationURL(t *testing.T) {
	got, err := authorizationURL("https://issuer.test/auth?prompt=login", authorizationParams{
		ClientID:      "client",
		RedirectURI:   "http://localhost:9876/callback",
		Scopes:        []string{"openid", "offline_access"},
		State:         "the-state",
		CodeChallenge: "the-challenge",
	})
	if err != nil {
		t.Fatalf("authorizationURL() error = %v", err)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	query := parsed.Query()

	if query.Get("prompt") != "login" {
		t.Error("authorizationURL() dropped a query the endpoint already carried")
	}
	if query.Get("scope") != "openid offline_access" {
		t.Errorf("scope = %q, want %q", query.Get("scope"), "openid offline_access")
	}
	if query.Get("code_challenge_method") != ChallengeMethodS256 {
		t.Errorf("code_challenge_method = %q, want %q", query.Get("code_challenge_method"), ChallengeMethodS256)
	}
}

func TestAuthorizationURLRejectsABrokenEndpoint(t *testing.T) {
	if _, err := authorizationURL("://nope", authorizationParams{}); err == nil {
		t.Fatal("authorizationURL() error = nil, want an error")
	}
}
