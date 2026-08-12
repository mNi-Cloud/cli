package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// consoleBufferSize matches the buffers the vm-controller reads and writes a
	// console with, so that a frame is not split on the way.
	consoleBufferSize = 10240
	keepAliveInterval = 30 * time.Second
)

// TokenProvider hands out the access token a request has to carry.
type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

// TLSOptions is how a context trusts the servers it talks to.
type TLSOptions struct {
	CACert                string
	InsecureSkipTLSVerify bool
}

// NewHTTPClient builds the HTTP client of one context. The API client and the
// session that renews tokens share it, so the gateway and the identity
// provider are trusted the same way.
func NewHTTPClient(options TLSOptions, timeout time.Duration) (*http.Client, error) {
	tlsConfig, err := options.tlsConfig()
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig

	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

// NewWebSocketDialer builds the dialer that opens the streaming subresources of
// one context. It trusts the servers the HTTP client of the context trusts, but
// carries no deadline of its own: a console lives as long as the user keeps it
// open, so only the handshake is bounded.
func NewWebSocketDialer(options TLSOptions, handshakeTimeout time.Duration) (*websocket.Dialer, error) {
	tlsConfig, err := options.tlsConfig()
	if err != nil {
		return nil, err
	}

	return &websocket.Dialer{
		TLSClientConfig:  tlsConfig,
		HandshakeTimeout: handshakeTimeout,
		Proxy:            http.ProxyFromEnvironment,
		NetDialContext:   (&net.Dialer{Timeout: handshakeTimeout, KeepAlive: keepAliveInterval}).DialContext,
		ReadBufferSize:   consoleBufferSize,
		WriteBufferSize:  consoleBufferSize,
	}, nil
}

func (o TLSOptions) tlsConfig() (*tls.Config, error) {
	config := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: o.InsecureSkipTLSVerify,
	}
	if o.CACert == "" {
		return config, nil
	}

	pem, err := os.ReadFile(o.CACert)
	if err != nil {
		return nil, fmt.Errorf("cannot read the CA certificate %s: %w", o.CACert, err)
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("cannot read the trust store of this machine: %w", err)
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("the CA certificate %s holds no certificate", o.CACert)
	}
	config.RootCAs = pool

	return config, nil
}

// bearerTransport puts a fresh access token on every request. A request whose
// token cannot be produced is not sent at all.
type bearerTransport struct {
	base   http.RoundTripper
	tokens TokenProvider
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.tokens.Token(req.Context())
	if err != nil {
		return nil, err
	}

	authenticated := req.Clone(req.Context())
	authenticated.Header.Set("Authorization", "Bearer "+token)
	return t.base.RoundTrip(authenticated)
}

func withBearerToken(base *http.Client, tokens TokenProvider) *http.Client {
	inner := base.Transport
	if inner == nil {
		inner = http.DefaultTransport
	}

	authenticated := *base
	authenticated.Transport = &bearerTransport{base: inner, tokens: tokens}
	return &authenticated
}
