package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"

func TestNewPKCEVerifierShape(t *testing.T) {
	pkce, err := NewPKCE()
	if err != nil {
		t.Fatalf("NewPKCE() error = %v", err)
	}

	if len(pkce.Verifier) < 43 || len(pkce.Verifier) > 128 {
		t.Errorf("verifier length = %d, want between 43 and 128", len(pkce.Verifier))
	}
	for _, r := range pkce.Verifier {
		if !strings.ContainsRune(unreserved, r) {
			t.Fatalf("verifier carries %q, which RFC 7636 does not allow", r)
		}
	}
}

func TestNewPKCEChallengeIsSHA256OfVerifier(t *testing.T) {
	pkce, err := NewPKCE()
	if err != nil {
		t.Fatalf("NewPKCE() error = %v", err)
	}

	if pkce.Method != ChallengeMethodS256 {
		t.Errorf("Method = %q, want %q", pkce.Method, ChallengeMethodS256)
	}

	sum := sha256.Sum256([]byte(pkce.Verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if pkce.Challenge != want {
		t.Errorf("Challenge = %q, want %q", pkce.Challenge, want)
	}
}

func TestNewPKCEIsFreshEveryTime(t *testing.T) {
	seen := map[string]bool{}
	for range 32 {
		pkce, err := NewPKCE()
		if err != nil {
			t.Fatalf("NewPKCE() error = %v", err)
		}
		if seen[pkce.Verifier] {
			t.Fatalf("NewPKCE() produced the verifier %q twice", pkce.Verifier)
		}
		seen[pkce.Verifier] = true
	}
}

func TestNewStateIsFreshEveryTime(t *testing.T) {
	seen := map[string]bool{}
	for range 32 {
		state, err := newState()
		if err != nil {
			t.Fatalf("newState() error = %v", err)
		}
		if state == "" {
			t.Fatal("newState() returned an empty state")
		}
		if seen[state] {
			t.Fatalf("newState() produced %q twice", state)
		}
		seen[state] = true
	}
}
