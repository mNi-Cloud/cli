package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// LoginRequest is the public client registration a login runs against.
type LoginRequest struct {
	Issuer      string
	ClientID    string
	RedirectURI string
	Scopes      []string
}

func (r LoginRequest) validate() error {
	missing := []string{}
	if r.Issuer == "" {
		missing = append(missing, "issuer")
	}
	if r.ClientID == "" {
		missing = append(missing, "client ID")
	}
	if r.RedirectURI == "" {
		missing = append(missing, "redirect URI")
	}
	if len(r.Scopes) == 0 {
		missing = append(missing, "scopes")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("cannot log in without %v", missing)
}

// Flow runs the authorization code flow of RFC 8252 for a native client.
type Flow struct {
	HTTPClient  *http.Client
	OpenBrowser func(authorizationURL string) error
	Output      io.Writer
	Now         func() time.Time
}

// Run sends the user through the browser and comes back with a token set.
func (f *Flow) Run(ctx context.Context, req LoginRequest) (Token, error) {
	if err := req.validate(); err != nil {
		return Token{}, err
	}

	metadata, err := Discoverer{HTTPClient: f.HTTPClient}.Discover(ctx, req.Issuer)
	if err != nil {
		return Token{}, err
	}

	pkce, err := NewPKCE()
	if err != nil {
		return Token{}, err
	}
	state, err := newState()
	if err != nil {
		return Token{}, err
	}

	callback, err := newCallbackServer(req.RedirectURI)
	if err != nil {
		return Token{}, err
	}
	defer callback.Close()
	callback.Start()

	target, err := authorizationURL(metadata.AuthorizationEndpoint, authorizationParams{
		ClientID:      req.ClientID,
		RedirectURI:   callback.RedirectURI(),
		Scopes:        req.Scopes,
		State:         state,
		CodeChallenge: pkce.Challenge,
	})
	if err != nil {
		return Token{}, err
	}

	f.invite(target)

	result, err := callback.Wait(ctx)
	if err != nil {
		return Token{}, err
	}
	if err := verifyCallback(result, state, metadata); err != nil {
		return Token{}, err
	}

	tokens := &TokenClient{HTTPClient: f.HTTPClient, Now: f.Now}
	return tokens.Exchange(ctx, AuthorizationCodeRequest{
		TokenEndpoint: metadata.TokenEndpoint,
		Code:          result.Code,
		RedirectURI:   callback.RedirectURI(),
		ClientID:      req.ClientID,
		CodeVerifier:  pkce.Verifier,
	})
}

// invite prints the authorization URL and tries to open it. The URL is printed
// either way so that a machine without a browser can still be logged in.
func (f *Flow) invite(target string) {
	out := f.output()
	fmt.Fprintf(out, "Open this URL to sign in:\n\n  %s\n\n", target)

	open := f.OpenBrowser
	if open == nil {
		open = OpenBrowser
	}
	if err := open(target); err != nil {
		fmt.Fprintf(out, "Could not open a browser (%v). Open the URL above by hand.\n", err)
		return
	}
	fmt.Fprintln(out, "Waiting for the browser to come back...")
}

func (f *Flow) output() io.Writer {
	if f.Output != nil {
		return f.Output
	}
	return io.Discard
}

func verifyCallback(result callbackResult, state string, metadata Metadata) error {
	if result.State != state {
		return fmt.Errorf("the login came back with a state this CLI did not send, so it was dropped")
	}

	// RFC 9207 §2.4. A provider that promises to name itself has to, and the
	// name it gives has to be the one that was asked.
	if metadata.AuthorizationResponseIssParameterSupported && result.Issuer == "" {
		return fmt.Errorf("the login came back without the issuer the provider promised to send")
	}
	if result.Issuer != "" && strings.TrimSuffix(result.Issuer, "/") != strings.TrimSuffix(metadata.Issuer, "/") {
		return fmt.Errorf("the login came back from issuer %q instead of %q", result.Issuer, metadata.Issuer)
	}
	return nil
}

type authorizationParams struct {
	ClientID      string
	RedirectURI   string
	Scopes        []string
	State         string
	CodeChallenge string
}

func authorizationURL(endpoint string, params authorizationParams) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("authorization endpoint %q is not a URL: %w", endpoint, err)
	}

	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", params.ClientID)
	query.Set("redirect_uri", params.RedirectURI)
	query.Set("scope", strings.Join(params.Scopes, " "))
	query.Set("state", params.State)
	query.Set("code_challenge", params.CodeChallenge)
	query.Set("code_challenge_method", ChallengeMethodS256)
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}
