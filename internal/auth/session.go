package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mNi-Cloud/cli/internal/config"
)

// refreshLeeway renews an access token slightly before it dies, so that a
// request cannot be sent with a token that runs out on its way to the server.
const refreshLeeway = 30 * time.Second

// ErrLoginRequired reports that only a new browser login can go on.
var ErrLoginRequired = errors.New("login required")

// LoginRequiredError names the context whose session has to be renewed.
type LoginRequiredError struct {
	Context string
	Reason  error
}

func (e *LoginRequiredError) Error() string {
	message := fmt.Sprintf("not logged in to context %q: run `mni login --context %s`", e.Context, e.Context)
	if e.Reason != nil {
		message += " (" + e.Reason.Error() + ")"
	}
	return message
}

func (e *LoginRequiredError) Unwrap() []error {
	if e.Reason == nil {
		return []error{ErrLoginRequired}
	}
	return []error{ErrLoginRequired, e.Reason}
}

// CredentialStore is where a session reads and writes the tokens it manages.
type CredentialStore interface {
	Credential(contextName string) (config.Credential, bool, error)
	SaveCredential(cred config.Credential) error
}

// Session hands out the access token of one context and renews it when it runs
// out. It never falls back to an anonymous request: a caller that cannot be
// authenticated is told to log in again.
type Session struct {
	ContextName string
	OAuth       config.OAuth
	Store       CredentialStore
	Tokens      *TokenClient
	Metadata    MetadataSource
	Now         func() time.Time
	Leeway      time.Duration

	mu    sync.Mutex
	held  config.Credential
	found bool
}

// NewSession builds the session of one context over an HTTP client that
// already carries its TLS settings.
func NewSession(contextName string, oauth config.OAuth, store CredentialStore, httpClient *http.Client) *Session {
	return &Session{
		ContextName: contextName,
		OAuth:       oauth,
		Store:       store,
		Tokens:      &TokenClient{HTTPClient: httpClient},
		Metadata:    Discoverer{HTTPClient: httpClient},
		Leeway:      refreshLeeway,
	}
}

// Token returns an access token that is good to send right now.
func (s *Session) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.load(); err != nil {
		return "", err
	}
	if !s.held.Expired(s.now(), s.Leeway) {
		return s.held.AccessToken, nil
	}
	return s.refresh(ctx)
}

func (s *Session) load() error {
	if s.found {
		return nil
	}

	cred, found, err := s.Store.Credential(s.ContextName)
	if err != nil {
		return err
	}
	if !found {
		return &LoginRequiredError{Context: s.ContextName}
	}

	s.held, s.found = cred, true
	return nil
}

func (s *Session) refresh(ctx context.Context) (string, error) {
	if s.held.RefreshToken == "" {
		return "", &LoginRequiredError{
			Context: s.ContextName,
			Reason:  errors.New("the session has run out and there is no refresh token"),
		}
	}

	metadata, err := s.Metadata.Discover(ctx, s.OAuth.Issuer)
	if err != nil {
		return "", err
	}

	token, err := s.Tokens.Refresh(ctx, RefreshRequest{
		TokenEndpoint: metadata.TokenEndpoint,
		RefreshToken:  s.held.RefreshToken,
		ClientID:      s.OAuth.ClientID,
	})
	if err != nil {
		var tokenErr *TokenError
		if errors.As(err, &tokenErr) {
			return "", &LoginRequiredError{Context: s.ContextName, Reason: err}
		}
		return "", err
	}

	renewed := NewCredential(s.ContextName, token)
	if err := s.Store.SaveCredential(renewed); err != nil {
		return "", err
	}

	s.held = renewed
	return s.held.AccessToken, nil
}

func (s *Session) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// NewCredential turns a fresh token set into the record that is stored on disk.
func NewCredential(contextName string, token Token) config.Credential {
	return config.Credential{
		Context:      contextName,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.ExpiresAt,
	}
}
