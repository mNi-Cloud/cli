// Package auth signs the CLI in to the mNi Cloud identity provider and keeps
// the access token it works with fresh.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// ChallengeMethodS256 is the only code challenge method this CLI offers.
const ChallengeMethodS256 = "S256"

// randomBytes is the entropy behind a verifier or a state. RFC 7636 §4.1 asks
// for at least 32 bytes.
const randomBytes = 32

// PKCE is one proof key pair, as RFC 7636 defines it.
type PKCE struct {
	Verifier  string
	Challenge string
	Method    string
}

// NewPKCE draws a fresh verifier and derives its S256 challenge.
func NewPKCE() (PKCE, error) {
	verifier, err := randomURLSafeString()
	if err != nil {
		return PKCE{}, fmt.Errorf("cannot draw a code verifier: %w", err)
	}

	sum := sha256.Sum256([]byte(verifier))
	return PKCE{
		Verifier:  verifier,
		Challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
		Method:    ChallengeMethodS256,
	}, nil
}

// newState draws the value that ties an authorization response to the request
// that started it.
func newState() (string, error) {
	state, err := randomURLSafeString()
	if err != nil {
		return "", fmt.Errorf("cannot draw a state: %w", err)
	}
	return state, nil
}

func randomURLSafeString() (string, error) {
	raw := make([]byte, randomBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
