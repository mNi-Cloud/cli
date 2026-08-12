package config

import (
	"testing"
	"time"
)

func TestCredentialsPutAndFind(t *testing.T) {
	creds := &Credentials{}
	creds.Put(Credential{Context: "a", AccessToken: "one"})
	creds.Put(Credential{Context: "b", AccessToken: "two"})
	creds.Put(Credential{Context: "a", AccessToken: "three"})

	if len(creds.Credentials) != 2 {
		t.Fatalf("Credentials length = %d, want 2", len(creds.Credentials))
	}

	got, ok := creds.Find("a")
	if !ok {
		t.Fatal("Find(\"a\") reported not found")
	}
	if got.AccessToken != "three" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "three")
	}

	if _, ok := creds.Find("c"); ok {
		t.Error("Find(\"c\") reported found")
	}
}

func TestCredentialsRemove(t *testing.T) {
	creds := &Credentials{}
	creds.Put(Credential{Context: "a"})

	if !creds.Remove("a") {
		t.Fatal("Remove(\"a\") = false, want true")
	}
	if len(creds.Credentials) != 0 {
		t.Errorf("Credentials length = %d, want 0", len(creds.Credentials))
	}
	if creds.Remove("a") {
		t.Error("Remove(\"a\") = true for a credential that is already gone")
	}
}

func TestCredentialExpired(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt time.Time
		leeway    time.Duration
		want      bool
	}{
		{name: "well ahead", expiresAt: now.Add(time.Hour), leeway: time.Minute, want: false},
		{name: "inside leeway", expiresAt: now.Add(30 * time.Second), leeway: time.Minute, want: true},
		{name: "already past", expiresAt: now.Add(-time.Second), leeway: 0, want: true},
		{name: "zero value counts as expired", expiresAt: time.Time{}, leeway: 0, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := Credential{ExpiresAt: tt.expiresAt}
			if got := cred.Expired(now, tt.leeway); got != tt.want {
				t.Errorf("Expired() = %v, want %v", got, tt.want)
			}
		})
	}
}
