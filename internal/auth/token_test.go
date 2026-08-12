package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
}

var testNow = time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)

func fixedNow() time.Time { return testNow }

type recordedRequest struct {
	form        url.Values
	contentType string
	accept      string
	method      string
}

func newTokenServer(t *testing.T, handle func(form url.Values, w http.ResponseWriter)) (*httptest.Server, *recordedRequest) {
	t.Helper()

	recorded := &recordedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm() error = %v", err)
		}
		recorded.form = r.PostForm
		recorded.contentType = r.Header.Get("Content-Type")
		recorded.accept = r.Header.Get("Accept")
		recorded.method = r.Method
		handle(r.PostForm, w)
	}))
	t.Cleanup(server.Close)
	return server, recorded
}

func TestExchangePostsTheAuthorizationCode(t *testing.T) {
	server, recorded := newTokenServer(t, func(form url.Values, w http.ResponseWriter) {
		writeJSON(t, w, map[string]any{
			"access_token":  "access",
			"refresh_token": "refresh",
			"id_token":      "id",
			"token_type":    "Bearer",
			"scope":         "openid profile email offline_access",
			"expires_in":    86400,
		})
	})

	client := &TokenClient{HTTPClient: server.Client(), Now: fixedNow}
	token, err := client.Exchange(context.Background(), AuthorizationCodeRequest{
		TokenEndpoint: server.URL,
		Code:          "the-code",
		RedirectURI:   "http://localhost:9876/callback",
		ClientID:      "client-cli-sample",
		CodeVerifier:  "the-verifier",
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}

	if recorded.method != http.MethodPost {
		t.Errorf("method = %q, want POST", recorded.method)
	}
	if !strings.HasPrefix(recorded.contentType, "application/x-www-form-urlencoded") {
		t.Errorf("Content-Type = %q, want a form post", recorded.contentType)
	}
	if recorded.accept != "application/json" {
		t.Errorf("Accept = %q, want application/json", recorded.accept)
	}

	want := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"the-code"},
		"redirect_uri":  {"http://localhost:9876/callback"},
		"client_id":     {"client-cli-sample"},
		"code_verifier": {"the-verifier"},
	}
	for key, values := range want {
		if got := recorded.form.Get(key); got != values[0] {
			t.Errorf("form[%q] = %q, want %q", key, got, values[0])
		}
	}

	if token.AccessToken != "access" || token.RefreshToken != "refresh" || token.IDToken != "id" {
		t.Errorf("token = %+v, want the tokens the server sent", token)
	}
	if wantExpiry := testNow.Add(86400 * time.Second); !token.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("ExpiresAt = %v, want %v", token.ExpiresAt, wantExpiry)
	}
}

func TestExchangeReportsOAuthError(t *testing.T) {
	server, _ := newTokenServer(t, func(form url.Values, w http.ResponseWriter) {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(t, w, map[string]any{
			"error":             "invalid_grant",
			"error_description": "the code has already been used",
		})
	})

	client := &TokenClient{HTTPClient: server.Client(), Now: fixedNow}
	_, err := client.Exchange(context.Background(), AuthorizationCodeRequest{TokenEndpoint: server.URL})

	var tokenErr *TokenError
	if !errors.As(err, &tokenErr) {
		t.Fatalf("Exchange() error = %v, want a TokenError", err)
	}
	if tokenErr.Code != "invalid_grant" {
		t.Errorf("Code = %q, want %q", tokenErr.Code, "invalid_grant")
	}
	if tokenErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", tokenErr.StatusCode, http.StatusBadRequest)
	}
	if !strings.Contains(tokenErr.Error(), "already been used") {
		t.Errorf("Error() = %q, want it to carry the description", tokenErr)
	}
}

func TestExchangeReportsUnreadableError(t *testing.T) {
	server, _ := newTokenServer(t, func(form url.Values, w http.ResponseWriter) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>gateway down</html>"))
	})

	client := &TokenClient{HTTPClient: server.Client(), Now: fixedNow}
	_, err := client.Exchange(context.Background(), AuthorizationCodeRequest{TokenEndpoint: server.URL})
	if err == nil {
		t.Fatal("Exchange() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("Exchange() error = %q, want it to carry the status", err)
	}
}

func TestExchangeRejectsTokenWithoutAccessToken(t *testing.T) {
	server, _ := newTokenServer(t, func(form url.Values, w http.ResponseWriter) {
		writeJSON(t, w, map[string]any{"token_type": "Bearer", "expires_in": 3600})
	})

	client := &TokenClient{HTTPClient: server.Client(), Now: fixedNow}
	if _, err := client.Exchange(context.Background(), AuthorizationCodeRequest{TokenEndpoint: server.URL}); err == nil {
		t.Fatal("Exchange() error = nil, want an error about the missing access token")
	}
}

func TestExchangeRejectsTokenWithoutLifetime(t *testing.T) {
	server, _ := newTokenServer(t, func(form url.Values, w http.ResponseWriter) {
		writeJSON(t, w, map[string]any{"access_token": "access", "token_type": "Bearer"})
	})

	client := &TokenClient{HTTPClient: server.Client(), Now: fixedNow}
	_, err := client.Exchange(context.Background(), AuthorizationCodeRequest{TokenEndpoint: server.URL})
	if err == nil {
		t.Fatal("Exchange() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "expires_in") {
		t.Errorf("Exchange() error = %q, want it to name expires_in", err)
	}
}

func TestExchangeRejectsUnknownTokenType(t *testing.T) {
	server, _ := newTokenServer(t, func(form url.Values, w http.ResponseWriter) {
		writeJSON(t, w, map[string]any{"access_token": "access", "token_type": "mac", "expires_in": 60})
	})

	client := &TokenClient{HTTPClient: server.Client(), Now: fixedNow}
	if _, err := client.Exchange(context.Background(), AuthorizationCodeRequest{TokenEndpoint: server.URL}); err == nil {
		t.Fatal("Exchange() error = nil, want an error about the token type")
	}
}

func TestRefreshPostsTheRefreshToken(t *testing.T) {
	server, recorded := newTokenServer(t, func(form url.Values, w http.ResponseWriter) {
		writeJSON(t, w, map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	})

	client := &TokenClient{HTTPClient: server.Client(), Now: fixedNow}
	token, err := client.Refresh(context.Background(), RefreshRequest{
		TokenEndpoint: server.URL,
		RefreshToken:  "old-refresh",
		ClientID:      "client-cli-sample",
	})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if got := recorded.form.Get("grant_type"); got != "refresh_token" {
		t.Errorf("grant_type = %q, want %q", got, "refresh_token")
	}
	if got := recorded.form.Get("refresh_token"); got != "old-refresh" {
		t.Errorf("refresh_token = %q, want %q", got, "old-refresh")
	}
	if got := recorded.form.Get("client_id"); got != "client-cli-sample" {
		t.Errorf("client_id = %q, want %q", got, "client-cli-sample")
	}
	if token.AccessToken != "new-access" || token.RefreshToken != "new-refresh" {
		t.Errorf("token = %+v, want the refreshed pair", token)
	}
}

func TestRefreshKeepsTheOldRefreshTokenWhenTheServerOmitsOne(t *testing.T) {
	server, _ := newTokenServer(t, func(form url.Values, w http.ResponseWriter) {
		writeJSON(t, w, map[string]any{"access_token": "new-access", "token_type": "Bearer", "expires_in": 3600})
	})

	client := &TokenClient{HTTPClient: server.Client(), Now: fixedNow}
	token, err := client.Refresh(context.Background(), RefreshRequest{
		TokenEndpoint: server.URL,
		RefreshToken:  "old-refresh",
	})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if token.RefreshToken != "old-refresh" {
		t.Errorf("RefreshToken = %q, want the old one to be kept", token.RefreshToken)
	}
}

func TestRefreshReportsOAuthError(t *testing.T) {
	server, _ := newTokenServer(t, func(form url.Values, w http.ResponseWriter) {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(t, w, map[string]any{"error": "invalid_grant"})
	})

	client := &TokenClient{HTTPClient: server.Client(), Now: fixedNow}
	_, err := client.Refresh(context.Background(), RefreshRequest{TokenEndpoint: server.URL, RefreshToken: "old"})

	var tokenErr *TokenError
	if !errors.As(err, &tokenErr) {
		t.Fatalf("Refresh() error = %v, want a TokenError", err)
	}
	if tokenErr.Code != "invalid_grant" {
		t.Errorf("Code = %q, want %q", tokenErr.Code, "invalid_grant")
	}
}

func TestRefreshRejectsEmptyRefreshToken(t *testing.T) {
	client := &TokenClient{Now: fixedNow}
	if _, err := client.Refresh(context.Background(), RefreshRequest{TokenEndpoint: "https://example.test"}); err == nil {
		t.Fatal("Refresh() error = nil, want an error")
	}
}
