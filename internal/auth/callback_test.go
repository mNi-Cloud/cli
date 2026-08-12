package auth

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func startTestCallbackServer(t *testing.T) *callbackServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	redirect := &url.URL{Scheme: "http", Host: listener.Addr().String(), Path: "/callback"}
	server := newCallbackServerOn(redirect, listener)
	server.Start()
	t.Cleanup(func() { _ = server.Close() })
	return server
}

func TestCallbackServerReceivesTheCode(t *testing.T) {
	server := startTestCallbackServer(t)

	resp, err := http.Get(server.RedirectURI() + "?code=the-code&state=the-state&iss=https%3A%2F%2Fissuer.test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(strings.ToLower(string(body)), "signed in") {
		t.Errorf("callback page = %q, want it to say the login worked", body)
	}

	result, err := server.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if result.Code != "the-code" {
		t.Errorf("Code = %q, want %q", result.Code, "the-code")
	}
	if result.State != "the-state" {
		t.Errorf("State = %q, want %q", result.State, "the-state")
	}
	if result.Issuer != "https://issuer.test" {
		t.Errorf("Issuer = %q, want %q", result.Issuer, "https://issuer.test")
	}
}

func TestCallbackServerReportsTheAuthorizationError(t *testing.T) {
	server := startTestCallbackServer(t)

	resp, err := http.Get(server.RedirectURI() + "?error=access_denied&error_description=the+user+said+no&state=s")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if !strings.Contains(string(body), "access_denied") {
		t.Errorf("callback page = %q, want it to show the error", body)
	}

	_, err = server.Wait(context.Background())
	if err == nil {
		t.Fatal("Wait() error = nil, want the authorization error")
	}
	if !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("Wait() error = %q, want it to carry the error code", err)
	}
}

func TestCallbackServerRejectsAResponseWithoutACode(t *testing.T) {
	server := startTestCallbackServer(t)

	resp, err := http.Get(server.RedirectURI() + "?state=s")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	resp.Body.Close()

	if _, err := server.Wait(context.Background()); err == nil {
		t.Fatal("Wait() error = nil, want an error about the missing code")
	}
}

func TestCallbackServerIgnoresOtherPaths(t *testing.T) {
	server := startTestCallbackServer(t)

	resp, err := http.Get("http://" + server.listener.Addr().String() + "/favicon.ico")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := server.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Wait() error = %v, want the deadline to run out", err)
	}
}

func TestCallbackServerStopsWhenTheContextEnds(t *testing.T) {
	server := startTestCallbackServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := server.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Wait() error = %v, want context.Canceled", err)
	}
}

func TestNewCallbackServerRejectsNonLoopbackRedirects(t *testing.T) {
	tests := []string{
		"http://example.test/callback",
		"http://0.0.0.0:9876/callback",
		"https://192.168.1.1:9876/callback",
	}

	for _, redirect := range tests {
		t.Run(redirect, func(t *testing.T) {
			server, err := newCallbackServer(redirect)
			if err == nil {
				_ = server.Close()
				t.Fatalf("newCallbackServer(%q) error = nil, want it refused", redirect)
			}
			if !strings.Contains(err.Error(), "loopback") {
				t.Errorf("newCallbackServer(%q) error = %q, want it to mention loopback", redirect, err)
			}
		})
	}
}

func TestNewCallbackServerRejectsMalformedRedirects(t *testing.T) {
	for _, redirect := range []string{"", "://nope", "localhost:9876/callback"} {
		if server, err := newCallbackServer(redirect); err == nil {
			_ = server.Close()
			t.Errorf("newCallbackServer(%q) error = nil, want it refused", redirect)
		}
	}
}

func TestNewCallbackServerKeepsARedirectThatNamesItsPort(t *testing.T) {
	redirect := freeLoopbackRedirect(t)

	server, err := newCallbackServer(redirect)
	if err != nil {
		t.Fatalf("newCallbackServer() error = %v", err)
	}
	defer server.Close()

	if server.RedirectURI() != redirect {
		t.Errorf("RedirectURI() = %q, want the URI as it was written %q", server.RedirectURI(), redirect)
	}
}

func TestNewCallbackServerTakesAFreePortWhenTheRedirectAsksForPortZero(t *testing.T) {
	server, err := newCallbackServer("http://127.0.0.1:0/callback")
	if err != nil {
		t.Fatalf("newCallbackServer() error = %v", err)
	}
	defer server.Close()

	redirect, err := url.Parse(server.RedirectURI())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if redirect.Host != server.listener.Addr().String() {
		t.Errorf("RedirectURI() = %q, want the address the listener took %q", server.RedirectURI(), server.listener.Addr())
	}
	if redirect.Port() == "0" {
		t.Errorf("RedirectURI() = %q, want the port the OS handed out", server.RedirectURI())
	}
	if redirect.Path != "/callback" {
		t.Errorf("path = %q, want %q", redirect.Path, "/callback")
	}
}

func TestNewCallbackServerKeepsTheHostOfADynamicRedirect(t *testing.T) {
	server, err := newCallbackServer("http://localhost:0/callback")
	if err != nil {
		t.Fatalf("newCallbackServer() error = %v", err)
	}
	defer server.Close()

	redirect, err := url.Parse(server.RedirectURI())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if redirect.Hostname() != "localhost" {
		t.Errorf("RedirectURI() = %q, want the host it was given", server.RedirectURI())
	}
	if _, port, _ := net.SplitHostPort(server.listener.Addr().String()); redirect.Port() != port {
		t.Errorf("RedirectURI() = %q, want port %q", server.RedirectURI(), port)
	}
}

func TestTwoDynamicCallbackServersRunSideBySide(t *testing.T) {
	first, err := newCallbackServer("http://127.0.0.1:0/callback")
	if err != nil {
		t.Fatalf("newCallbackServer() error = %v", err)
	}
	defer first.Close()

	second, err := newCallbackServer("http://127.0.0.1:0/callback")
	if err != nil {
		t.Fatalf("newCallbackServer() error = %v, want a second login to listen as well", err)
	}
	defer second.Close()

	if first.RedirectURI() == second.RedirectURI() {
		t.Errorf("both logins listen on %q, want a port of their own", first.RedirectURI())
	}
}

func TestNewCallbackServerBindsTheRequestedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	taken := listener.Addr().String()
	defer listener.Close()

	if server, err := newCallbackServer("http://" + taken + "/callback"); err == nil {
		_ = server.Close()
		t.Error("newCallbackServer() error = nil on a port that is already taken")
	}
}
