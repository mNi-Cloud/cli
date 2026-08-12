package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// bearerTokenType is the only token type this CLI can present to api-gateway.
const bearerTokenType = "bearer"

// DefaultScopes are the scopes a CLI login asks for. offline_access is what
// buys the refresh token that keeps later commands from opening a browser.
var DefaultScopes = []string{"openid", "profile", "email", "offline_access"}

// Token is a token set as the CLI holds it, with the lifetime already turned
// into the moment the access token dies.
type Token struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	TokenType    string
	Scope        string
	ExpiresAt    time.Time
}

// TokenError is an RFC 6749 §5.2 error object from the token endpoint.
type TokenError struct {
	StatusCode  int
	Code        string
	Description string
}

func (e *TokenError) Error() string {
	message := "the identity provider refused the request: " + e.Code
	if e.Description != "" {
		message += " (" + e.Description + ")"
	}
	return message
}

// AuthorizationCodeRequest redeems an authorization code for a token set.
type AuthorizationCodeRequest struct {
	TokenEndpoint string
	Code          string
	RedirectURI   string
	ClientID      string
	CodeVerifier  string
}

// RefreshRequest renews a token set that has run out.
type RefreshRequest struct {
	TokenEndpoint string
	RefreshToken  string
	ClientID      string
}

// TokenClient talks to the token endpoint of a public OAuth client.
type TokenClient struct {
	HTTPClient *http.Client
	Now        func() time.Time
}

// Exchange redeems an authorization code.
func (c *TokenClient) Exchange(ctx context.Context, req AuthorizationCodeRequest) (Token, error) {
	form := url.Values{
		"grant_type":    {grantAuthorizationCode},
		"code":          {req.Code},
		"redirect_uri":  {req.RedirectURI},
		"client_id":     {req.ClientID},
		"code_verifier": {req.CodeVerifier},
	}
	return c.post(ctx, req.TokenEndpoint, form)
}

// Refresh renews a token set.
func (c *TokenClient) Refresh(ctx context.Context, req RefreshRequest) (Token, error) {
	if req.RefreshToken == "" {
		return Token{}, fmt.Errorf("no refresh token to renew the session with")
	}

	form := url.Values{
		"grant_type":    {grantRefreshToken},
		"refresh_token": {req.RefreshToken},
		"client_id":     {req.ClientID},
	}

	token, err := c.post(ctx, req.TokenEndpoint, form)
	if err != nil {
		return Token{}, err
	}

	// RFC 6749 §6. A refresh response may leave the refresh token out, and then
	// the one that was sent stays valid.
	if token.RefreshToken == "" {
		token.RefreshToken = req.RefreshToken
	}
	return token, nil
}

// tokenResponse is the JSON body of a successful token endpoint call.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (c *TokenClient) post(ctx context.Context, endpoint string, form url.Values) (Token, error) {
	if endpoint == "" {
		return Token{}, fmt.Errorf("no token endpoint to call")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("cannot reach the token endpoint %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Token{}, fmt.Errorf("cannot read the answer of %s: %w", endpoint, err)
	}

	if resp.StatusCode != http.StatusOK {
		return Token{}, tokenFailure(resp.StatusCode, body)
	}

	var decoded tokenResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return Token{}, fmt.Errorf("cannot read the token %s issued: %w", endpoint, err)
	}
	return c.token(decoded)
}

func (c *TokenClient) token(decoded tokenResponse) (Token, error) {
	if decoded.AccessToken == "" {
		return Token{}, fmt.Errorf("the identity provider issued a token set without an access_token")
	}
	if !strings.EqualFold(decoded.TokenType, bearerTokenType) {
		return Token{}, fmt.Errorf("the identity provider issued a %q token, and this CLI can only present a bearer token", decoded.TokenType)
	}
	if decoded.ExpiresIn <= 0 {
		return Token{}, fmt.Errorf("the identity provider issued a token set without an expires_in, so its lifetime cannot be tracked")
	}

	return Token{
		AccessToken:  decoded.AccessToken,
		RefreshToken: decoded.RefreshToken,
		IDToken:      decoded.IDToken,
		TokenType:    decoded.TokenType,
		Scope:        decoded.Scope,
		ExpiresAt:    c.now().Add(time.Duration(decoded.ExpiresIn) * time.Second),
	}, nil
}

func (c *TokenClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *TokenClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func tokenFailure(status int, body []byte) error {
	var decoded struct {
		Code        string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil || decoded.Code == "" {
		return fmt.Errorf("the token endpoint answered %d: %s", status, strings.TrimSpace(string(body)))
	}
	return &TokenError{StatusCode: status, Code: decoded.Code, Description: decoded.Description}
}
