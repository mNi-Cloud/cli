package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mNi-Cloud/cli/internal/api"
)

// closeWriteWait bounds the goodbye a stream sends when it is closed.
const closeWriteWait = 5 * time.Second

// Stream is one open connection to a subresource. Both directions carry raw
// bytes, whatever the subresource makes of them.
type Stream interface {
	io.ReadWriteCloser
}

// stream carries bytes over the binary frames of a WebSocket. One reader and
// one writer may use it at a time, which is what copying in both directions
// needs.
type stream struct {
	conn   *websocket.Conn
	reader io.Reader

	closeOnce sync.Once
	closeErr  error
}

func newStream(conn *websocket.Conn) Stream {
	return &stream{conn: conn}
}

// Read hands out the payload of the frames the server sends. A frame is a unit
// of the connection, not of the console, so a read that empties one goes on
// with the next.
func (s *stream) Read(p []byte) (int, error) {
	for {
		if s.reader == nil {
			_, reader, err := s.conn.NextReader()
			if err != nil {
				return 0, endOfStream(err)
			}
			s.reader = reader
		}

		read, err := s.reader.Read(p)
		if errors.Is(err, io.EOF) {
			s.reader = nil
			if read == 0 {
				continue
			}
			return read, nil
		}
		if err != nil {
			s.reader = nil
			return read, endOfStream(err)
		}

		return read, nil
	}
}

func (s *stream) Write(p []byte) (int, error) {
	if err := s.conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *stream) Close() error {
	s.closeOnce.Do(func() {
		// The goodbye tells the server the stream ended on purpose, so that it
		// releases the console instead of waiting for a connection that is gone.
		goodbye := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		_ = s.conn.WriteControl(websocket.CloseMessage, goodbye, time.Now().Add(closeWriteWait))
		s.closeErr = s.conn.Close()
	})
	return s.closeErr
}

// endOfStream reads a server that closed the connection as the end of the
// stream. A console the machine ended is not a failure of the CLI.
func endOfStream(err error) error {
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return io.EOF
	}
	return err
}

// websocketURL turns the address a subresource is served on into the one a
// WebSocket is opened on.
func websocketURL(target string) (string, error) {
	parsed, err := url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("cannot open a stream to %s: %w", target, err)
	}

	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	default:
		return "", fmt.Errorf("cannot open a stream to %s: %q is no address of an api-gateway", target, parsed.Scheme)
	}
	return parsed.String(), nil
}

// handshakeError reads the answer a refused handshake carries, so that a stream
// that cannot be opened is reported the way a failed request is. Whoever
// refused it, the gateway or the controller behind it, answers in the same
// shape as every other endpoint.
func handshakeError(err error, response *http.Response) error {
	if !errors.Is(err, websocket.ErrBadHandshake) || response == nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()

	raw, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return err
	}

	var envelope api.Response[any]
	decodeErr := json.Unmarshal(raw, &envelope)
	return &api.Error{
		StatusCode: response.StatusCode,
		Message:    failureMessage(envelope, decodeErr, raw),
		Challenge:  response.Header.Get("WWW-Authenticate"),
	}
}
