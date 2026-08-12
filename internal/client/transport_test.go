package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tlsConfigOf(client *http.Client) (*tls.Config, error) {
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("transport is %T, want *http.Transport", client.Transport)
	}
	return transport.TLSClientConfig, nil
}

func TestNewHTTPClientWithoutTLSOptions(t *testing.T) {
	client, err := NewHTTPClient(TLSOptions{}, 5*time.Second)
	if err != nil {
		t.Fatalf("NewHTTPClient() error = %v", err)
	}
	if client.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want %v", client.Timeout, 5*time.Second)
	}
}

func TestNewHTTPClientSkippingVerification(t *testing.T) {
	client, err := NewHTTPClient(TLSOptions{InsecureSkipTLSVerify: true}, time.Second)
	if err != nil {
		t.Fatalf("NewHTTPClient() error = %v", err)
	}

	config, err := tlsConfigOf(client)
	if err != nil {
		t.Fatalf("tlsConfigOf() error = %v", err)
	}
	if !config.InsecureSkipVerify {
		t.Error("InsecureSkipVerify = false, want true")
	}
}

func TestNewHTTPClientLoadsACACert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ca.crt")
	if err := os.WriteFile(path, []byte(testCACert), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client, err := NewHTTPClient(TLSOptions{CACert: path}, time.Second)
	if err != nil {
		t.Fatalf("NewHTTPClient() error = %v", err)
	}

	config, err := tlsConfigOf(client)
	if err != nil {
		t.Fatalf("tlsConfigOf() error = %v", err)
	}
	if config.RootCAs == nil {
		t.Fatal("RootCAs is nil, want the CA loaded")
	}
}

func TestNewHTTPClientRejectsAMissingCACert(t *testing.T) {
	_, err := NewHTTPClient(TLSOptions{CACert: filepath.Join(t.TempDir(), "nope.crt")}, time.Second)
	if err == nil {
		t.Fatal("NewHTTPClient() error = nil, want the missing file reported")
	}
}

func TestNewHTTPClientRejectsABrokenCACert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ca.crt")
	if err := os.WriteFile(path, []byte("this is not a certificate"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := NewHTTPClient(TLSOptions{CACert: path}, time.Second)
	if err == nil {
		t.Fatal("NewHTTPClient() error = nil, want the broken file reported")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("NewHTTPClient() error = %q, want it to name the file", err)
	}
}

// testCACert is a self signed certificate used only to check that a PEM file
// is read into the trust pool.
const testCACert = `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`
