package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mNi-Cloud/cli/internal/config"
)

type fakeCredentialStore struct {
	credential config.Credential
	found      bool
	loadErr    error
	saveErr    error
	saved      []config.Credential
	loads      int
}

func (s *fakeCredentialStore) Credential(string) (config.Credential, bool, error) {
	s.loads++
	if s.loadErr != nil {
		return config.Credential{}, false, s.loadErr
	}
	return s.credential, s.found, nil
}

func (s *fakeCredentialStore) SaveCredential(cred config.Credential) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saved = append(s.saved, cred)
	s.credential, s.found = cred, true
	return nil
}

type fakeMetadataSource struct {
	metadata Metadata
	err      error
	calls    int
}

func (s *fakeMetadataSource) Discover(context.Context, string) (Metadata, error) {
	s.calls++
	return s.metadata, s.err
}

func newTestSession(t *testing.T, store CredentialStore, tokenEndpoint string) *Session {
	t.Helper()
	return &Session{
		ContextName: "e2e",
		OAuth:       config.OAuth{Issuer: "https://issuer.test", ClientID: "client-cli-sample"},
		Store:       store,
		Tokens:      &TokenClient{HTTPClient: http.DefaultClient, Now: fixedNow},
		Metadata:    &fakeMetadataSource{metadata: Metadata{TokenEndpoint: tokenEndpoint}},
		Now:         fixedNow,
		Leeway:      30 * time.Second,
	}
}

func TestSessionReturnsAValidToken(t *testing.T) {
	store := &fakeCredentialStore{
		found:      true,
		credential: config.Credential{Context: "e2e", AccessToken: "access", ExpiresAt: testNow.Add(time.Hour)},
	}
	session := newTestSession(t, store, "https://issuer.test/token")

	token, err := session.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token != "access" {
		t.Errorf("Token() = %q, want %q", token, "access")
	}
	if len(store.saved) != 0 {
		t.Errorf("Token() wrote %d credentials, want none", len(store.saved))
	}
}

func TestSessionCredentialReturnsTokenAndExpiryTogether(t *testing.T) {
	expiresAt := testNow.Add(time.Hour)
	store := &fakeCredentialStore{
		found:      true,
		credential: config.Credential{Context: "e2e", AccessToken: "access", ExpiresAt: expiresAt},
	}
	session := newTestSession(t, store, "https://issuer.test/token")

	credential, err := session.Credential(context.Background())
	if err != nil {
		t.Fatalf("Credential() error = %v", err)
	}
	if credential.AccessToken != "access" || !credential.ExpiresAt.Equal(expiresAt) {
		t.Errorf("Credential() = %+v", credential)
	}
}

func TestSessionCachesTheCredential(t *testing.T) {
	store := &fakeCredentialStore{
		found:      true,
		credential: config.Credential{Context: "e2e", AccessToken: "access", ExpiresAt: testNow.Add(time.Hour)},
	}
	session := newTestSession(t, store, "https://issuer.test/token")

	for range 3 {
		if _, err := session.Token(context.Background()); err != nil {
			t.Fatalf("Token() error = %v", err)
		}
	}
	if store.loads != 1 {
		t.Errorf("credential was read %d times, want 1", store.loads)
	}
}

func TestSessionRefreshesAnExpiredToken(t *testing.T) {
	server, recorded := newTokenServer(t, func(form url.Values, w http.ResponseWriter) {
		writeJSON(t, w, map[string]any{
			"access_token":  "fresh",
			"refresh_token": "next-refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	})

	store := &fakeCredentialStore{
		found: true,
		credential: config.Credential{
			Context:      "e2e",
			AccessToken:  "stale",
			RefreshToken: "refresh",
			ExpiresAt:    testNow.Add(-time.Minute),
		},
	}
	session := newTestSession(t, store, server.URL)
	session.Tokens.HTTPClient = server.Client()

	token, err := session.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token != "fresh" {
		t.Errorf("Token() = %q, want %q", token, "fresh")
	}
	if got := recorded.form.Get("refresh_token"); got != "refresh" {
		t.Errorf("refresh_token = %q, want %q", got, "refresh")
	}

	if len(store.saved) != 1 {
		t.Fatalf("saved %d credentials, want 1", len(store.saved))
	}
	saved := store.saved[0]
	if saved.Context != "e2e" || saved.AccessToken != "fresh" || saved.RefreshToken != "next-refresh" {
		t.Errorf("saved credential = %+v, want the refreshed pair", saved)
	}
	if want := testNow.Add(time.Hour); !saved.ExpiresAt.Equal(want) {
		t.Errorf("saved ExpiresAt = %v, want %v", saved.ExpiresAt, want)
	}
}

func TestSessionRefreshesOnlyOnce(t *testing.T) {
	refreshes := 0
	server, _ := newTokenServer(t, func(form url.Values, w http.ResponseWriter) {
		refreshes++
		writeJSON(t, w, map[string]any{
			"access_token": "fresh",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	store := &fakeCredentialStore{
		found: true,
		credential: config.Credential{
			Context:      "e2e",
			RefreshToken: "refresh",
			ExpiresAt:    testNow.Add(-time.Minute),
		},
	}
	session := newTestSession(t, store, server.URL)
	session.Tokens.HTTPClient = server.Client()

	for range 3 {
		if _, err := session.Token(context.Background()); err != nil {
			t.Fatalf("Token() error = %v", err)
		}
	}
	if refreshes != 1 {
		t.Errorf("refreshed %d times, want 1", refreshes)
	}
}

func TestSessionRefreshesWhenTheTokenDiesInsideTheLeeway(t *testing.T) {
	refreshes := 0
	server, _ := newTokenServer(t, func(form url.Values, w http.ResponseWriter) {
		refreshes++
		writeJSON(t, w, map[string]any{"access_token": "fresh", "token_type": "Bearer", "expires_in": 3600})
	})

	store := &fakeCredentialStore{
		found: true,
		credential: config.Credential{
			Context:      "e2e",
			AccessToken:  "stale",
			RefreshToken: "refresh",
			ExpiresAt:    testNow.Add(10 * time.Second),
		},
	}
	session := newTestSession(t, store, server.URL)
	session.Tokens.HTTPClient = server.Client()

	if _, err := session.Token(context.Background()); err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if refreshes != 1 {
		t.Errorf("refreshed %d times, want 1", refreshes)
	}
}

func TestSessionWithoutAnyCredential(t *testing.T) {
	session := newTestSession(t, &fakeCredentialStore{}, "https://issuer.test/token")

	_, err := session.Token(context.Background())
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("Token() error = %v, want ErrLoginRequired", err)
	}
	if !strings.Contains(err.Error(), "mni login") {
		t.Errorf("Token() error = %q, want it to point at `mni login`", err)
	}

	var loginErr *LoginRequiredError
	if !errors.As(err, &loginErr) {
		t.Fatalf("Token() error = %v, want a LoginRequiredError", err)
	}
	if loginErr.Context != "e2e" {
		t.Errorf("Context = %q, want %q", loginErr.Context, "e2e")
	}
}

func TestSessionWithAnExpiredTokenAndNoRefreshToken(t *testing.T) {
	store := &fakeCredentialStore{
		found:      true,
		credential: config.Credential{Context: "e2e", AccessToken: "stale", ExpiresAt: testNow.Add(-time.Hour)},
	}
	session := newTestSession(t, store, "https://issuer.test/token")

	if _, err := session.Token(context.Background()); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("Token() error = %v, want ErrLoginRequired", err)
	}
}

func TestSessionWhenTheRefreshTokenIsRejected(t *testing.T) {
	server, _ := newTokenServer(t, func(form url.Values, w http.ResponseWriter) {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(t, w, map[string]any{"error": "invalid_grant"})
	})

	store := &fakeCredentialStore{
		found: true,
		credential: config.Credential{
			Context:      "e2e",
			AccessToken:  "stale",
			RefreshToken: "revoked",
			ExpiresAt:    testNow.Add(-time.Hour),
		},
	}
	session := newTestSession(t, store, server.URL)
	session.Tokens.HTTPClient = server.Client()

	_, err := session.Token(context.Background())
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("Token() error = %v, want ErrLoginRequired", err)
	}

	var tokenErr *TokenError
	if !errors.As(err, &tokenErr) {
		t.Fatalf("Token() error = %v, want the OAuth error kept as the reason", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("a failed refresh wrote %d credentials, want none", len(store.saved))
	}
}

func TestSessionReportsADiscoveryFailure(t *testing.T) {
	store := &fakeCredentialStore{
		found: true,
		credential: config.Credential{
			Context:      "e2e",
			RefreshToken: "refresh",
			ExpiresAt:    testNow.Add(-time.Hour),
		},
	}
	session := newTestSession(t, store, "")
	session.Metadata = &fakeMetadataSource{err: errors.New("issuer unreachable")}

	_, err := session.Token(context.Background())
	if err == nil {
		t.Fatal("Token() error = nil, want the discovery failure")
	}
	if errors.Is(err, ErrLoginRequired) {
		t.Errorf("Token() error = %v, want a plain failure rather than asking for a new login", err)
	}
	if !strings.Contains(err.Error(), "issuer unreachable") {
		t.Errorf("Token() error = %q, want it to carry the reason", err)
	}
}

func TestSessionReportsAStoreFailure(t *testing.T) {
	session := newTestSession(t, &fakeCredentialStore{loadErr: errors.New("disk on fire")}, "https://issuer.test/token")

	_, err := session.Token(context.Background())
	if err == nil {
		t.Fatal("Token() error = nil, want the store failure")
	}
	if errors.Is(err, ErrLoginRequired) {
		t.Errorf("Token() error = %v, want a plain failure rather than asking for a new login", err)
	}
}
