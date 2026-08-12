package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func discoveryDocument(issuer string) map[string]any {
	return map[string]any{
		"issuer":                                         issuer,
		"authorization_endpoint":                         issuer + "/auth",
		"token_endpoint":                                 issuer + "/api/auth/token",
		"scopes_supported":                               []string{"openid", "profile", "email", "offline_access"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"client_secret_basic", "none"},
		"authorization_response_iss_parameter_supported": true,
	}
}

func newDiscoveryServer(t *testing.T, document func(issuer string) any) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, document(server.URL))
	})
	return server
}

func TestDiscover(t *testing.T) {
	server := newDiscoveryServer(t, func(issuer string) any { return discoveryDocument(issuer) })

	metadata, err := Discoverer{HTTPClient: server.Client()}.Discover(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if metadata.TokenEndpoint != server.URL+"/api/auth/token" {
		t.Errorf("TokenEndpoint = %q, want %q", metadata.TokenEndpoint, server.URL+"/api/auth/token")
	}
	if metadata.AuthorizationEndpoint != server.URL+"/auth" {
		t.Errorf("AuthorizationEndpoint = %q, want %q", metadata.AuthorizationEndpoint, server.URL+"/auth")
	}
	if !metadata.SupportsRefreshToken() {
		t.Error("SupportsRefreshToken() = false, want true")
	}
	if !metadata.AuthorizationResponseIssParameterSupported {
		t.Error("AuthorizationResponseIssParameterSupported = false, want true")
	}
}

func TestDiscoverTrimsTrailingSlash(t *testing.T) {
	server := newDiscoveryServer(t, func(issuer string) any { return discoveryDocument(issuer) })

	if _, err := (Discoverer{HTTPClient: server.Client()}).Discover(context.Background(), server.URL+"/"); err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
}

func TestDiscoverRejectsIssuerMismatch(t *testing.T) {
	server := newDiscoveryServer(t, func(string) any { return discoveryDocument("https://elsewhere.test") })

	_, err := Discoverer{HTTPClient: server.Client()}.Discover(context.Background(), server.URL)
	if err == nil {
		t.Fatal("Discover() error = nil, want an issuer mismatch error")
	}
	if !strings.Contains(err.Error(), "issuer") {
		t.Errorf("Discover() error = %q, want it to mention the issuer", err)
	}
}

func TestDiscoverRejectsMissingTokenEndpoint(t *testing.T) {
	server := newDiscoveryServer(t, func(issuer string) any {
		document := discoveryDocument(issuer)
		delete(document, "token_endpoint")
		return document
	})

	_, err := Discoverer{HTTPClient: server.Client()}.Discover(context.Background(), server.URL)
	if err == nil {
		t.Fatal("Discover() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "token_endpoint") {
		t.Errorf("Discover() error = %q, want it to name token_endpoint", err)
	}
}

func TestDiscoverRejectsMissingAuthorizationEndpoint(t *testing.T) {
	server := newDiscoveryServer(t, func(issuer string) any {
		document := discoveryDocument(issuer)
		delete(document, "authorization_endpoint")
		return document
	})

	if _, err := (Discoverer{HTTPClient: server.Client()}).Discover(context.Background(), server.URL); err == nil {
		t.Fatal("Discover() error = nil, want an error")
	}
}

func TestDiscoverRejectsMissingPKCE(t *testing.T) {
	server := newDiscoveryServer(t, func(issuer string) any {
		document := discoveryDocument(issuer)
		document["code_challenge_methods_supported"] = []string{"plain"}
		return document
	})

	_, err := Discoverer{HTTPClient: server.Client()}.Discover(context.Background(), server.URL)
	if err == nil {
		t.Fatal("Discover() error = nil, want an error about S256")
	}
	if !strings.Contains(err.Error(), "S256") {
		t.Errorf("Discover() error = %q, want it to mention S256", err)
	}
}

func TestDiscoverRejectsMissingAuthorizationCodeGrant(t *testing.T) {
	server := newDiscoveryServer(t, func(issuer string) any {
		document := discoveryDocument(issuer)
		document["grant_types_supported"] = []string{"client_credentials"}
		return document
	})

	if _, err := (Discoverer{HTTPClient: server.Client()}).Discover(context.Background(), server.URL); err == nil {
		t.Fatal("Discover() error = nil, want an error")
	}
}

func TestDiscoverReportsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	_, err := Discoverer{HTTPClient: server.Client()}.Discover(context.Background(), server.URL)
	if err == nil {
		t.Fatal("Discover() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Discover() error = %q, want it to carry the status", err)
	}
}

func TestDiscoverRejectsEmptyIssuer(t *testing.T) {
	if _, err := (Discoverer{}).Discover(context.Background(), ""); err == nil {
		t.Fatal("Discover(\"\") error = nil, want an error")
	}
}
