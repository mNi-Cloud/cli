package auth

import (
	"context"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// shutdownGrace is how long the loopback server is given to finish writing the
// page the browser is still reading.
const shutdownGrace = 2 * time.Second

// dynamicPort in a redirect URI asks the OS for a free port.
const dynamicPort = "0"

// AuthorizationError is an RFC 6749 §4.1.2.1 error handed back on the redirect.
type AuthorizationError struct {
	Code        string
	Description string
}

func (e *AuthorizationError) Error() string {
	message := "the identity provider refused the login: " + e.Code
	if e.Description != "" {
		message += " (" + e.Description + ")"
	}
	return message
}

// callbackResult is the authorization response the browser handed back.
type callbackResult struct {
	Code   string
	State  string
	Issuer string
}

type callbackOutcome struct {
	result callbackResult
	err    error
}

// callbackServer is the loopback listener that catches the redirect. RFC 8252
// §7.3 has a native client take its authorization response this way.
type callbackServer struct {
	redirectURI *url.URL
	listener    net.Listener
	server      *http.Server
	outcomes    chan callbackOutcome
	answered    sync.Once
}

// newCallbackServer binds the loopback address a redirect URI names.
func newCallbackServer(redirectURI string) (*callbackServer, error) {
	parsed, err := parseLoopbackRedirect(redirectURI)
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", loopbackAddress(parsed))
	if err != nil {
		return nil, fmt.Errorf("cannot listen on %s for the login redirect: %w", parsed.Host, err)
	}

	bound, err := boundRedirect(parsed, listener)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	return newCallbackServerOn(bound, listener), nil
}

func newCallbackServerOn(redirectURI *url.URL, listener net.Listener) *callbackServer {
	callback := &callbackServer{
		redirectURI: redirectURI,
		listener:    listener,
		outcomes:    make(chan callbackOutcome, 1),
	}
	callback.server = &http.Server{
		Handler:           http.HandlerFunc(callback.handle),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return callback
}

// RedirectURI returns the address the identity provider must redirect to.
func (s *callbackServer) RedirectURI() string {
	return s.redirectURI.String()
}

// Start serves the redirect until the server is closed.
func (s *callbackServer) Start() {
	go func() {
		_ = s.server.Serve(s.listener)
	}()
}

// Wait blocks until the browser comes back or the context ends.
func (s *callbackServer) Wait(ctx context.Context) (callbackResult, error) {
	select {
	case outcome := <-s.outcomes:
		return outcome.result, outcome.err
	case <-ctx.Done():
		return callbackResult{}, ctx.Err()
	}
}

// Close stops the loopback server, leaving the last page time to reach the browser.
func (s *callbackServer) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *callbackServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != s.redirectURI.Path {
		http.NotFound(w, r)
		return
	}

	query := r.URL.Query()
	outcome := readCallback(query)

	page := successPage
	if outcome.err != nil {
		page = failurePage
	}
	renderCallbackPage(w, page, outcome.err)

	s.answered.Do(func() { s.outcomes <- outcome })
}

func readCallback(query url.Values) callbackOutcome {
	if code := query.Get("error"); code != "" {
		return callbackOutcome{err: &AuthorizationError{
			Code:        code,
			Description: query.Get("error_description"),
		}}
	}

	code := query.Get("code")
	if code == "" {
		return callbackOutcome{err: fmt.Errorf("the identity provider redirected back without an authorization code")}
	}

	return callbackOutcome{result: callbackResult{
		Code:   code,
		State:  query.Get("state"),
		Issuer: query.Get("iss"),
	}}
}

func parseLoopbackRedirect(redirectURI string) (*url.URL, error) {
	parsed, err := url.Parse(redirectURI)
	if err != nil {
		return nil, fmt.Errorf("redirect URI %q is not a URL: %w", redirectURI, err)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("redirect URI %q names no host", redirectURI)
	}

	if !isLoopbackHost(parsed.Hostname()) {
		return nil, fmt.Errorf("redirect URI %q does not point at a loopback address, and the CLI only listens on loopback", redirectURI)
	}
	if parsed.Scheme != "http" {
		return nil, fmt.Errorf("redirect URI %q is not http, and the loopback listener speaks plain http", redirectURI)
	}
	if parsed.Path == "" {
		return nil, fmt.Errorf("redirect URI %q names no path", redirectURI)
	}
	return parsed, nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func loopbackAddress(redirectURI *url.URL) string {
	port := redirectURI.Port()
	if port == "" {
		port = "80"
	}
	return net.JoinHostPort(redirectURI.Hostname(), port)
}

// boundRedirect names the port the listener took. A redirect URI that asks for
// port 0 leaves the choice to the OS, which RFC 8252 §7.3 allows a native app
// to do. The port has to be read back before the login starts, because the
// authorization request and the token request carry the same redirect URI
// (RFC 6749 §4.1.3).
func boundRedirect(redirectURI *url.URL, listener net.Listener) (*url.URL, error) {
	if redirectURI.Port() != dynamicPort {
		return redirectURI, nil
	}

	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return nil, fmt.Errorf("the login listens on %s, which names no port to be redirected to", listener.Addr())
	}

	bound := *redirectURI
	bound.Host = net.JoinHostPort(redirectURI.Hostname(), strconv.Itoa(address.Port))
	return &bound, nil
}

func renderCallbackPage(w http.ResponseWriter, page *template.Template, cause error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	reason := ""
	if cause != nil {
		reason = cause.Error()
	}
	_ = page.Execute(w, reason)

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

var (
	successPage = template.Must(template.New("success").Parse(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>mNi Cloud</title></head>
<body style="font-family: sans-serif; margin: 4rem auto; max-width: 32rem;">
<h1>Signed in</h1>
<p>You can close this tab and go back to the terminal.</p>
</body>
</html>
`))

	failurePage = template.Must(template.New("failure").Parse(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>mNi Cloud</title></head>
<body style="font-family: sans-serif; margin: 4rem auto; max-width: 32rem;">
<h1>Sign in failed</h1>
<p>{{.}}</p>
<p>Go back to the terminal to see what to do next.</p>
</body>
</html>
`))
)
