package config

import (
	"net/url"
	"testing"
)

func TestDefaultRedirectURIAsksForAFreePort(t *testing.T) {
	redirect, err := url.Parse(DefaultRedirectURI)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", DefaultRedirectURI, err)
	}

	if redirect.Port() != "0" {
		t.Errorf("DefaultRedirectURI = %q, want port 0 so that the OS picks a free one", DefaultRedirectURI)
	}
	if redirect.Path != "/callback" {
		t.Errorf("DefaultRedirectURI = %q, want path %q", DefaultRedirectURI, "/callback")
	}
}
