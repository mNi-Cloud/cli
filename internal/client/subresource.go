package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

// binarySubprotocol names the frames a stream is made of. The server answers a
// handshake with the first subprotocol it is offered, and a client that names
// none is left without one, so the stream says here what it carries.
const binarySubprotocol = "binary"

// SubresourceClient calls one subresource of one object. api-gateway relays a
// subresource to the controller that serves the resource, which either runs an
// operation or hands out a stream of bytes.
type SubresourceClient interface {
	// Post runs the operation the subresource stands for.
	Post(ctx context.Context) error
	// Connect opens the subresource as a stream of bytes.
	Connect(ctx context.Context) (Stream, error)
}

// SubresourceOption sets one detail of how a subresource is addressed.
type SubresourceOption func(*subresourceClient)

// WithQuery hands the subresource the parameters it takes, such as how many
// lines of a log to read or what command to run. Both the operation and the
// stream carry them, because a subresource takes the same parameters whichever
// way it is called.
func WithQuery(query url.Values) SubresourceOption {
	return func(s *subresourceClient) { s.query = query }
}

type subresourceClient struct {
	url        string
	query      url.Values
	httpClient *http.Client
	dialer     *websocket.Dialer
	tokens     TokenProvider
}

func (s *subresourceClient) Post(ctx context.Context) error {
	_, err := post[any](ctx, s.httpClient, s.target())
	return err
}

func (s *subresourceClient) Connect(ctx context.Context) (Stream, error) {
	target, err := websocketURL(s.target())
	if err != nil {
		return nil, err
	}

	token, err := s.tokens.Token(ctx)
	if err != nil {
		return nil, err
	}
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)

	// The dialer is shared by every stream of the context, and gorilla reads the
	// subprotocol off the dialer rather than the header, so it is set on a copy.
	dialer := *s.dialer
	dialer.Subprotocols = []string{binarySubprotocol}

	conn, response, err := dialer.DialContext(ctx, target, header)
	if err != nil {
		return nil, handshakeError(err, response)
	}
	return newStream(conn), nil
}

// target is the address of the subresource, with the parameters it was given.
func (s *subresourceClient) target() string {
	if len(s.query) == 0 {
		return s.url
	}
	return s.url + "?" + s.query.Encode()
}
