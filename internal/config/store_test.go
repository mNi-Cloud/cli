package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return NewStoreAt(filepath.Join(dir, "mni", "config.yaml"), filepath.Join(dir, "mni", "credentials.yaml"))
}

func TestStoreLoadConfigWhenFileIsMissing(t *testing.T) {
	store := newTestStore(t)

	cfg, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil for a missing file", err)
	}
	if cfg.CurrentContext != "" || len(cfg.Contexts) != 0 {
		t.Errorf("LoadConfig() = %+v, want an empty config", cfg)
	}
}

func TestStoreConfigRoundTrip(t *testing.T) {
	store := newTestStore(t)

	cfg := &Config{CurrentContext: "e2e"}
	cfg.Put(Context{
		Name:                  "e2e",
		Server:                "https://api.example.com",
		Tenant:                "e2etest",
		CACert:                "/etc/ca.crt",
		InsecureSkipTLSVerify: true,
		OAuth: OAuth{
			Issuer:      "https://auth.example.com",
			ClientID:    "client-cli-sample",
			RedirectURI: "http://localhost:9876/callback",
		},
	})

	if err := store.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	loaded, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if loaded.CurrentContext != "e2e" {
		t.Errorf("CurrentContext = %q, want %q", loaded.CurrentContext, "e2e")
	}
	if len(loaded.Contexts) != 1 {
		t.Fatalf("Contexts length = %d, want 1", len(loaded.Contexts))
	}
	if loaded.Contexts[0] != cfg.Contexts[0] {
		t.Errorf("Contexts[0] = %+v, want %+v", loaded.Contexts[0], cfg.Contexts[0])
	}
}

func TestStoreConfigUsesDocumentedKeys(t *testing.T) {
	store := newTestStore(t)

	cfg := &Config{CurrentContext: "e2e"}
	cfg.Put(Context{
		Name:   "e2e",
		Server: "https://example.test/api",
		OAuth: OAuth{
			Issuer:      "https://issuer.test",
			ClientID:    "client-cli-sample",
			RedirectURI: "http://localhost:9876/callback",
		},
	})
	if err := store.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	raw, err := os.ReadFile(store.ConfigPath())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	for _, key := range []string{"currentContext:", "contexts:", "server:", "oauth:", "issuer:", "clientID:", "redirectURI:"} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("config file is missing key %q\n%s", key, raw)
		}
	}
	for _, key := range []string{"tenant:", "caCert:", "insecureSkipTLSVerify:"} {
		if strings.Contains(string(raw), key) {
			t.Errorf("config file wrote empty key %q\n%s", key, raw)
		}
	}
}

func TestStoreCreatesDirectoryWithTightPermissions(t *testing.T) {
	store := newTestStore(t)

	if err := store.SaveConfig(&Config{}); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	info, err := os.Stat(filepath.Dir(store.ConfigPath()))
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("directory mode = %o, want %o", perm, 0o700)
	}
}

func TestStoreCredentialsRoundTrip(t *testing.T) {
	store := newTestStore(t)
	expiresAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	creds := &Credentials{}
	creds.Put(Credential{
		Context:      "e2e",
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresAt:    expiresAt,
	})
	if err := store.SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	loaded, err := store.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	got, ok := loaded.Find("e2e")
	if !ok {
		t.Fatal("Find(\"e2e\") reported not found")
	}
	if got.AccessToken != "access" || got.RefreshToken != "refresh" {
		t.Errorf("tokens = %+v, want access/refresh", got)
	}
	if !got.ExpiresAt.Equal(expiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, expiresAt)
	}
}

func TestStoreCredentialsFileIsPrivate(t *testing.T) {
	store := newTestStore(t)

	creds := &Credentials{}
	creds.Put(Credential{Context: "e2e", AccessToken: "access"})
	if err := store.SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	info, err := os.Stat(store.CredentialsPath())
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials mode = %o, want %o", perm, 0o600)
	}
}

func TestStoreCredentialsOverwriteKeepsPermissions(t *testing.T) {
	store := newTestStore(t)

	creds := &Credentials{}
	creds.Put(Credential{Context: "e2e", AccessToken: "first"})
	if err := store.SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}
	creds.Put(Credential{Context: "e2e", AccessToken: "second"})
	if err := store.SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	info, err := os.Stat(store.CredentialsPath())
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials mode = %o, want %o", perm, 0o600)
	}

	loaded, err := store.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	got, _ := loaded.Find("e2e")
	if got.AccessToken != "second" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "second")
	}
}

func TestStoreCredentialHelpers(t *testing.T) {
	store := newTestStore(t)

	if _, found, err := store.Credential("e2e"); err != nil || found {
		t.Fatalf("Credential() = (found %v, err %v), want (false, nil)", found, err)
	}

	if err := store.SaveCredential(Credential{Context: "e2e", AccessToken: "access"}); err != nil {
		t.Fatalf("SaveCredential() error = %v", err)
	}

	got, found, err := store.Credential("e2e")
	if err != nil {
		t.Fatalf("Credential() error = %v", err)
	}
	if !found {
		t.Fatal("Credential() reported not found after a save")
	}
	if got.AccessToken != "access" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "access")
	}

	removed, err := store.DeleteCredential("e2e")
	if err != nil {
		t.Fatalf("DeleteCredential() error = %v", err)
	}
	if !removed {
		t.Error("DeleteCredential() = false, want true")
	}
	if _, found, _ := store.Credential("e2e"); found {
		t.Error("Credential() still finds a deleted credential")
	}

	if removed, _ := store.DeleteCredential("e2e"); removed {
		t.Error("DeleteCredential() = true for a credential that is already gone")
	}
}

func TestStoreRejectsBrokenConfig(t *testing.T) {
	store := newTestStore(t)
	if err := os.MkdirAll(filepath.Dir(store.ConfigPath()), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(store.ConfigPath(), []byte("currentContext: [oops"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := store.LoadConfig(); err == nil {
		t.Error("LoadConfig() error = nil, want a parse error")
	}
}
