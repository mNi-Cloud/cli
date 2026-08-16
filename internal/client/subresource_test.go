package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mNi-Cloud/cli/internal/api"
)

const startPath = "/vm/v1alpha1/tenants/e2etest/virtualmachines/a/start"

var vmResource = api.APIResource{
	Group: "vm", Version: "v1alpha1", Resource: "virtualmachines", Kind: "VirtualMachine",
	Scope: api.ScopeNamespaced,
}

func newSubresource(t *testing.T, name, subresource string, handle func(w http.ResponseWriter, r *http.Request), options ...SubresourceOption) (SubresourceClient, *[]capturedRequest) {
	t.Helper()

	server, captured := newTestServer(t, handle)
	client := newTestClient(t, server, &staticTokens{token: "access"})

	subresourceClient, err := client.Subresource(vmResource, "e2etest", name, subresource, options...)
	if err != nil {
		t.Fatalf("Subresource() error = %v", err)
	}
	return subresourceClient, captured
}

// echoConsole answers a WebSocket upgrade the way vm-controller does: it
// answers with the subprotocol the client asked for and speaks binary frames.
func echoConsole(t *testing.T) func(w http.ResponseWriter, r *http.Request) {
	t.Helper()

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	return func(w http.ResponseWriter, r *http.Request) {
		header := http.Header{}
		if protocols := websocket.Subprotocols(r); len(protocols) > 0 {
			header.Set("Sec-WebSocket-Protocol", protocols[0])
		}

		conn, err := upgrader.Upgrade(w, r, header)
		if err != nil {
			t.Errorf("Upgrade() error = %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		for {
			messageType, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if messageType != websocket.BinaryMessage {
				t.Errorf("message type = %d, want %d", messageType, websocket.BinaryMessage)
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
				return
			}
		}
	}
}

func TestSubresourcePostCallsTheSubresourceOfTheObject(t *testing.T) {
	subresource, captured := newSubresource(t, "a", "start", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[any]{Success: true})
	})

	if err := subresource.Post(context.Background()); err != nil {
		t.Fatalf("Post() error = %v", err)
	}

	call := (*captured)[0]
	if call.method != http.MethodPost {
		t.Errorf("method = %q, want POST", call.method)
	}
	if call.path != startPath {
		t.Errorf("path = %q, want %q", call.path, startPath)
	}
	if call.authorization != "Bearer access" {
		t.Errorf("Authorization = %q, want the access token", call.authorization)
	}
}

func TestSubresourcePostReportsWhatTheGatewayAnswered(t *testing.T) {
	subresource, _ := newSubresource(t, "a", "start", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusConflict, api.Response[any]{
			Message: `virtual machine "a" is already running`,
		})
	})

	err := subresource.Post(context.Background())

	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("Post() error = %v, want an api.Error", err)
	}
	if apiErr.StatusCode != http.StatusConflict {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusConflict)
	}
	if !strings.Contains(apiErr.Message, "already running") {
		t.Errorf("Message = %q, want the message of the gateway", apiErr.Message)
	}
}

func TestSubresourceConnectCopiesBytesBothWays(t *testing.T) {
	subresource, captured := newSubresource(t, "a", "serial", echoConsole(t))

	stream, err := subresource.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer func() { _ = stream.Close() }()

	sent := []byte("login: ")
	if _, err := stream.Write(sent); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	received := make([]byte, len(sent))
	if _, err := io.ReadFull(stream, received); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if string(received) != string(sent) {
		t.Errorf("Read() = %q, want %q", received, sent)
	}

	call := (*captured)[0]
	if want := "/vm/v1alpha1/tenants/e2etest/virtualmachines/a/serial"; call.path != want {
		t.Errorf("path = %q, want %q", call.path, want)
	}
	if call.authorization != "Bearer access" {
		t.Errorf("Authorization = %q, want the access token on the handshake", call.authorization)
	}
}

func TestSubresourceConnectAsksForABinaryStream(t *testing.T) {
	var asked []string
	subresource, _ := newSubresource(t, "a", "vnc", func(w http.ResponseWriter, r *http.Request) {
		asked = websocket.Subprotocols(r)
		echoConsole(t)(w, r)
	})

	stream, err := subresource.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer func() { _ = stream.Close() }()

	if len(asked) != 1 || asked[0] != binarySubprotocol {
		t.Errorf("subprotocols = %v, want only %q", asked, binarySubprotocol)
	}
}

func TestSubresourceConnectReportsARejectedHandshake(t *testing.T) {
	subresource, _ := newSubresource(t, "a", "serial", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusConflict, api.Response[any]{
			Message: `virtual machine "a" is not running`,
		})
	})

	_, err := subresource.Connect(context.Background())

	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("Connect() error = %v, want an api.Error", err)
	}
	if apiErr.StatusCode != http.StatusConflict {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusConflict)
	}
	if !strings.Contains(apiErr.Message, "is not running") {
		t.Errorf("Message = %q, want the message of the gateway", apiErr.Message)
	}
}

func TestSubresourceConnectReportsAHandshakeRefusedForTheScope(t *testing.T) {
	subresource, _ := newSubresource(t, "a", "serial", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope", scope="mni:api"`)
		writeEnvelope(t, w, http.StatusForbidden, api.Response[any]{Message: "Forbidden"})
	})

	if _, err := subresource.Connect(context.Background()); !api.IsInsufficientScope(err) {
		t.Fatalf("Connect() error = %v, want a refusal that names the missing scope", err)
	}
}

func TestSubresourceConnectEndsWhenTheConsoleCloses(t *testing.T) {
	subresource, _ := newSubresource(t, "a", "serial", func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Upgrade() error = %v", err)
			return
		}

		goodbye := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		if err := conn.WriteControl(websocket.CloseMessage, goodbye, time.Now().Add(time.Second)); err != nil {
			t.Errorf("WriteControl() error = %v", err)
		}
		_ = conn.Close()
	})

	stream, err := subresource.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer func() { _ = stream.Close() }()

	if _, err := io.ReadAll(stream); err != nil {
		t.Errorf("ReadAll() error = %v, want the close read as the end of the stream", err)
	}
}

func TestSubresourceCloseTellsTheConsoleItEnded(t *testing.T) {
	closed := make(chan int, 1)
	subresource, _ := newSubresource(t, "a", "serial", func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Upgrade() error = %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		_, _, readErr := conn.ReadMessage()
		code := websocket.CloseAbnormalClosure
		var closeErr *websocket.CloseError
		if errors.As(readErr, &closeErr) {
			code = closeErr.Code
		}
		closed <- code
	})

	stream, err := subresource.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case code := <-closed:
		if code != websocket.CloseNormalClosure {
			t.Errorf("close code = %d, want %d", code, websocket.CloseNormalClosure)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the console was not told that the stream ended")
	}
}

func TestSubresourceWithoutAQueryAddressesThePathOnItsOwn(t *testing.T) {
	subresource, captured := newSubresource(t, "a", "start", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[any]{Success: true})
	})

	if err := subresource.Post(context.Background()); err != nil {
		t.Fatalf("Post() error = %v", err)
	}

	if got := (*captured)[0].query; len(got) != 0 {
		t.Errorf("query = %v, want none", got)
	}
}

func TestSubresourcePostCarriesTheQuery(t *testing.T) {
	query := url.Values{}
	query.Set("tail", "10")
	query.Add("command", "sh")
	query.Add("command", "-c")

	subresource, captured := newSubresource(t, "a", "logs", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[any]{Success: true})
	}, WithQuery(query))

	if err := subresource.Post(context.Background()); err != nil {
		t.Fatalf("Post() error = %v", err)
	}

	call := (*captured)[0]
	if want := "/vm/v1alpha1/tenants/e2etest/virtualmachines/a/logs"; call.path != want {
		t.Errorf("path = %q, want %q", call.path, want)
	}
	if got := call.query.Get("tail"); got != "10" {
		t.Errorf("tail = %q, want %q", got, "10")
	}
	if got := call.query["command"]; len(got) != 2 || got[0] != "sh" || got[1] != "-c" {
		t.Errorf("command = %v, want the words in the order they were given", got)
	}
}

func TestSubresourceConnectCarriesTheQuery(t *testing.T) {
	query := url.Values{}
	query.Set("follow", "true")
	query.Add("command", "sh")
	query.Add("command", "-c")

	subresource, captured := newSubresource(t, "a", "logs", echoConsole(t), WithQuery(query))

	stream, err := subresource.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer func() { _ = stream.Close() }()

	call := (*captured)[0]
	if want := "/vm/v1alpha1/tenants/e2etest/virtualmachines/a/logs"; call.path != want {
		t.Errorf("path = %q, want %q", call.path, want)
	}
	if got := call.query.Get("follow"); got != "true" {
		t.Errorf("follow = %q, want %q", got, "true")
	}
	if got := call.query["command"]; len(got) != 2 || got[0] != "sh" || got[1] != "-c" {
		t.Errorf("command = %v, want the words in the order they were given", got)
	}
}

func TestSubresourceStreamCarriesWholeFrames(t *testing.T) {
	subresource, _ := newSubresource(t, "a", "exec", echoConsole(t))

	stream, err := subresource.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer func() { _ = stream.Close() }()

	for _, frame := range []string{"one", "a longer one", "two"} {
		if err := stream.WriteMessage([]byte(frame)); err != nil {
			t.Fatalf("WriteMessage() error = %v", err)
		}
		read, err := stream.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage() error = %v", err)
		}
		if string(read) != frame {
			t.Errorf("ReadMessage() = %q, want %q", read, frame)
		}
	}
}

func TestSubresourceNeedsAName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(server.Close)

	client := newTestClient(t, server, &staticTokens{token: "access"})

	if _, err := client.Subresource(vmResource, "e2etest", "", "start"); err == nil {
		t.Error("Subresource() error = nil, want a name asked for")
	}
	if _, err := client.Subresource(vmResource, "e2etest", "a", ""); err == nil {
		t.Error("Subresource() error = nil, want a subresource asked for")
	}
	if _, err := client.Subresource(vmResource, "", "a", "start"); err == nil {
		t.Error("Subresource() error = nil, want a tenant asked for")
	}
}
