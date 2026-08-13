package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/unstructured"
)

type staticTokens struct {
	token string
	err   error
	calls int
}

func (t *staticTokens) Token(context.Context) (string, error) {
	t.calls++
	return t.token, t.err
}

type capturedRequest struct {
	method        string
	path          string
	authorization string
	contentType   string
	body          string
}

func newTestServer(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, *[]capturedRequest) {
	t.Helper()

	captured := &[]capturedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			if _, err := r.Body.Read(body); err != nil && err.Error() != "EOF" {
				t.Errorf("Read() error = %v", err)
			}
		}
		*captured = append(*captured, capturedRequest{
			method:        r.Method,
			path:          r.URL.Path,
			authorization: r.Header.Get("Authorization"),
			contentType:   r.Header.Get("Content-Type"),
			body:          string(body),
		})
		handle(w, r)
	}))
	t.Cleanup(server.Close)
	return server, captured
}

func writeEnvelope(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
}

func newTestClient(t *testing.T, server *httptest.Server, tokens TokenProvider) *Client {
	t.Helper()

	client, err := New(Options{
		Server:     server.URL,
		HTTPClient: server.Client(),
		WebSocket:  newTestDialer(t),
		Tokens:     tokens,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client
}

func newTestDialer(t *testing.T) *websocket.Dialer {
	t.Helper()

	dialer, err := NewWebSocketDialer(TLSOptions{}, 10*time.Second)
	if err != nil {
		t.Fatalf("NewWebSocketDialer() error = %v", err)
	}
	return dialer
}

var (
	namespacedResource = api.APIResource{Group: "vpc", Version: "v1alpha2", Resource: "vpcs", Kind: "Vpc", Scope: api.ScopeNamespaced}
	clusterResource    = api.APIResource{Group: "auth", Version: "v1alpha1", Resource: "tenants", Kind: "Tenant", Scope: api.ScopeCluster}
)

func TestNewRejectsAnIncompleteSetup(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{name: "no server", opts: Options{HTTPClient: http.DefaultClient, WebSocket: newTestDialer(t), Tokens: &staticTokens{}}},
		{name: "no http client", opts: Options{Server: "https://example.test/api", WebSocket: newTestDialer(t), Tokens: &staticTokens{}}},
		{name: "no web socket dialer", opts: Options{Server: "https://example.test/api", HTTPClient: http.DefaultClient, Tokens: &staticTokens{}}},
		{name: "no tokens", opts: Options{Server: "https://example.test/api", HTTPClient: http.DefaultClient, WebSocket: newTestDialer(t)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.opts); err == nil {
				t.Fatal("New() error = nil, want an error")
			}
		})
	}
}

func TestResourceURLCarriesTheTenant(t *testing.T) {
	server, captured := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[[]unstructured.Unstructured]{Success: true})
	})
	client := newTestClient(t, server, &staticTokens{token: "access"})

	resource, err := client.Resource(namespacedResource, "e2etest")
	if err != nil {
		t.Fatalf("Resource() error = %v", err)
	}
	if _, err := resource.List(context.Background()); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if want := "/vpc/v1alpha2/tenants/e2etest/vpcs"; (*captured)[0].path != want {
		t.Errorf("path = %q, want %q", (*captured)[0].path, want)
	}
}

func TestResourceURLOfAClusterScopedResourceHasNoTenant(t *testing.T) {
	server, captured := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[unstructured.Unstructured]{Success: true, Data: unstructured.Unstructured{}})
	})
	client := newTestClient(t, server, &staticTokens{token: "access"})

	resource, err := client.Resource(clusterResource, "e2etest")
	if err != nil {
		t.Fatalf("Resource() error = %v", err)
	}
	if _, err := resource.Get(context.Background(), "some-tenant"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if want := "/auth/v1alpha1/tenants/some-tenant"; (*captured)[0].path != want {
		t.Errorf("path = %q, want %q", (*captured)[0].path, want)
	}
}

func TestResourceRefusesANamespacedResourceWithoutATenant(t *testing.T) {
	server, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	client := newTestClient(t, server, &staticTokens{token: "access"})

	if _, err := client.Resource(namespacedResource, ""); err == nil {
		t.Fatal("Resource() error = nil, want it to refuse an unnamed tenant")
	}
}

func TestRequestsCarryTheAccessToken(t *testing.T) {
	server, captured := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[[]unstructured.Unstructured]{Success: true})
	})
	tokens := &staticTokens{token: "the-token"}
	client := newTestClient(t, server, tokens)

	resource, err := client.Resource(namespacedResource, "e2etest")
	if err != nil {
		t.Fatalf("Resource() error = %v", err)
	}
	if _, err := resource.List(context.Background()); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if want := "Bearer the-token"; (*captured)[0].authorization != want {
		t.Errorf("Authorization = %q, want %q", (*captured)[0].authorization, want)
	}
	if tokens.calls != 1 {
		t.Errorf("token was asked for %d times, want 1", tokens.calls)
	}
}

func TestAPIResourcesIsSentWithoutAToken(t *testing.T) {
	server, captured := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[api.APIResourceList]{
			Success: true,
			Data:    api.APIResourceList{namespacedResource},
		})
	})
	tokens := &staticTokens{err: errors.New("not logged in")}
	client := newTestClient(t, server, tokens)

	resources, err := client.APIResources(context.Background())
	if err != nil {
		t.Fatalf("APIResources() error = %v", err)
	}
	if len(resources) != 1 || resources[0].Resource != "vpcs" {
		t.Errorf("APIResources() = %+v, want the catalog the server serves", resources)
	}

	if (*captured)[0].path != "/api-resources" {
		t.Errorf("path = %q, want %q", (*captured)[0].path, "/api-resources")
	}
	if (*captured)[0].authorization != "" {
		t.Errorf("Authorization = %q, want the public route to be called without one", (*captured)[0].authorization)
	}
	if tokens.calls != 0 {
		t.Errorf("token was asked for %d times on a public route, want 0", tokens.calls)
	}
}

func TestAPIResourcesIsCached(t *testing.T) {
	calls := 0
	server, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeEnvelope(t, w, http.StatusOK, api.Response[api.APIResourceList]{Success: true, Data: api.APIResourceList{namespacedResource}})
	})
	client := newTestClient(t, server, &staticTokens{token: "access"})

	for range 3 {
		if _, err := client.APIResources(context.Background()); err != nil {
			t.Fatalf("APIResources() error = %v", err)
		}
	}
	if calls != 1 {
		t.Errorf("the catalog was fetched %d times, want 1", calls)
	}
}

func TestNotFoundIsReportedWithItsStatus(t *testing.T) {
	server, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusNotFound, api.Response[any]{Success: false, Message: "vpcs \"missing\" not found"})
	})
	client := newTestClient(t, server, &staticTokens{token: "access"})

	resource, err := client.Resource(namespacedResource, "e2etest")
	if err != nil {
		t.Fatalf("Resource() error = %v", err)
	}

	_, err = resource.Get(context.Background(), "missing")
	if !api.IsNotFound(err) {
		t.Fatalf("Get() error = %v, want a 404", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Get() error = %q, want it to carry the message of the server", err)
	}
}

func TestUnauthorizedIsReportedWithItsStatus(t *testing.T) {
	server, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeEnvelope(t, w, http.StatusUnauthorized, api.Response[any]{Success: false, Message: "unauthorized"})
	})
	client := newTestClient(t, server, &staticTokens{token: "stale"})

	resource, _ := client.Resource(namespacedResource, "e2etest")
	if _, err := resource.List(context.Background()); !api.IsUnauthorized(err) {
		t.Fatalf("List() error = %v, want a 401", err)
	}
}

func TestATokenWithoutTheScopeTheAPINeedsIsReported(t *testing.T) {
	server, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope", scope="mni:api"`)
		writeEnvelope(t, w, http.StatusForbidden, api.Response[any]{Success: false, Message: "Forbidden"})
	})
	client := newTestClient(t, server, &staticTokens{token: "issued-elsewhere"})

	resource, _ := client.Resource(namespacedResource, "e2etest")
	if _, err := resource.List(context.Background()); !api.IsInsufficientScope(err) {
		t.Fatalf("List() error = %v, want a refusal that names the missing scope", err)
	}
}

func TestAFailureThatIsNotAnEnvelopeStillCarriesTheStatus(t *testing.T) {
	server, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>gateway down</html>"))
	})
	client := newTestClient(t, server, &staticTokens{token: "access"})

	resource, _ := client.Resource(namespacedResource, "e2etest")
	_, err := resource.List(context.Background())

	code, ok := api.StatusCode(err)
	if !ok || code != http.StatusBadGateway {
		t.Fatalf("List() error = %v, want it to carry status 502", err)
	}
}

func TestASuccessFlagThatIsFalseIsAFailure(t *testing.T) {
	server, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[any]{Success: false, Message: "something went wrong"})
	})
	client := newTestClient(t, server, &staticTokens{token: "access"})

	resource, _ := client.Resource(namespacedResource, "e2etest")
	_, err := resource.List(context.Background())
	if err == nil {
		t.Fatal("List() error = nil, want the failed envelope reported")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("List() error = %q, want it to carry the message", err)
	}
}

func TestATokenFailureStopsTheRequest(t *testing.T) {
	calls := 0
	server, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeEnvelope(t, w, http.StatusOK, api.Response[[]unstructured.Unstructured]{Success: true})
	})

	sentinel := errors.New("run mni login")
	client := newTestClient(t, server, &staticTokens{err: sentinel})

	resource, _ := client.Resource(namespacedResource, "e2etest")
	_, err := resource.List(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("List() error = %v, want the token failure", err)
	}
	if calls != 0 {
		t.Errorf("the server was called %d times without a token, want 0", calls)
	}
}

func TestMe(t *testing.T) {
	server, captured := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[api.Identity]{
			Success: true,
			Data:    api.Identity{UserID: "u-1234", Username: "tester", Scopes: []string{"openid"}},
		})
	})
	client := newTestClient(t, server, &staticTokens{token: "access"})

	identity, err := client.Me(context.Background())
	if err != nil {
		t.Fatalf("Me() error = %v", err)
	}
	if identity.Username != "tester" {
		t.Errorf("Username = %q, want %q", identity.Username, "tester")
	}
	if (*captured)[0].path != "/me" {
		t.Errorf("path = %q, want %q", (*captured)[0].path, "/me")
	}
	if (*captured)[0].authorization == "" {
		t.Error("Authorization is empty, want /me to be authenticated")
	}
}

func TestTenants(t *testing.T) {
	server, captured := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[[]api.Tenant]{
			Success: true,
			Data:    []api.Tenant{{Name: "e2etest", Phase: "Ready", Role: "owner"}},
		})
	})
	client := newTestClient(t, server, &staticTokens{token: "access"})

	tenants, err := client.Tenants(context.Background())
	if err != nil {
		t.Fatalf("Tenants() error = %v", err)
	}
	if len(tenants) != 1 || tenants[0].Name != "e2etest" {
		t.Errorf("Tenants() = %+v, want the one tenant the server serves", tenants)
	}
	if (*captured)[0].path != "/tenants" {
		t.Errorf("path = %q, want %q", (*captured)[0].path, "/tenants")
	}
}

func TestServerURLTrailingSlashIsIgnored(t *testing.T) {
	server, captured := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[api.APIResourceList]{Success: true})
	})

	client, err := New(Options{
		Server:     server.URL + "/",
		HTTPClient: server.Client(),
		WebSocket:  newTestDialer(t),
		Tokens:     &staticTokens{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := client.APIResources(context.Background()); err != nil {
		t.Fatalf("APIResources() error = %v", err)
	}
	if (*captured)[0].path != "/api-resources" {
		t.Errorf("path = %q, want %q", (*captured)[0].path, "/api-resources")
	}
}
