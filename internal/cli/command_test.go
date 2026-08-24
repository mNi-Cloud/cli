package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/auth"
	"github.com/mNi-Cloud/cli/internal/config"
	"github.com/mNi-Cloud/cli/internal/unstructured"
)

var (
	testVPC = api.APIResource{
		Group: "vpc", Version: "v1alpha2", Resource: "vpcs", Singular: "vpc", Kind: "Vpc",
		Scope: api.ScopeNamespaced, Aliases: []string{"vpc"},
		AdditionalPrinterColumns: []api.AdditionalPrinterColumn{
			{Name: "Phase", Type: "string", JSONPath: ".status.phase"},
			{Name: "Gateway", Type: "string", JSONPath: ".status.gateway"},
		},
		SpecSchema: &api.Schema{
			Type: "object",
			Properties: map[string]*api.Schema{
				"enforceSecurityGroups": {Type: "boolean"},
				"peerings": {
					Type: "array",
					Items: &api.Schema{
						Type:       "object",
						Required:   []string{"target"},
						Properties: map[string]*api.Schema{"target": {Type: "string"}},
					},
				},
			},
		},
		StatusSchema: &api.Schema{
			Type: "object",
			Properties: map[string]*api.Schema{
				"phase":      {Type: "string", Default: "Pending", Enum: []any{"Pending", "Ready"}},
				"backingVpc": {Type: "string"},
			},
		},
	}
	testSubnet = api.APIResource{
		Group: "vpc", Version: "v1alpha2", Resource: "subnets", Singular: "subnet", Kind: "Subnet",
		Scope: api.ScopeNamespaced,
		SpecSchema: &api.Schema{
			Type:     "object",
			Required: []string{"cidr", "vpc"},
			Properties: map[string]*api.Schema{
				"cidr":       {Type: "string"},
				"vpc":        {Type: "string"},
				"routeTable": {Type: "string", Description: "RouteTable selects the routing policy for this Subnet."},
			},
		},
		StatusSchema: &api.Schema{
			Type:       "object",
			Properties: map[string]*api.Schema{"phase": {Type: "string"}},
		},
	}
	testTenantResource = api.APIResource{
		Group: "auth", Version: "v1alpha1", Resource: "tenants", Singular: "tenant", Kind: "Tenant",
		Scope: api.ScopeCluster,
	}
	testMachine = api.APIResource{
		Group: "vm", Version: "v1alpha1", Resource: "virtualmachines", Singular: "virtualmachine", Kind: "VirtualMachine",
		Scope: api.ScopeNamespaced, Aliases: []string{"vm"},
	}
	testContainer = api.APIResource{
		Group: "ctr", Version: "v1alpha", Resource: "containers", Singular: "container", Kind: "Container",
		Scope: api.ScopeNamespaced, Aliases: []string{"ctr"},
	}
	testSSHKey = api.APIResource{
		Group: "vm", Version: "v1alpha1", Resource: "sshkeys", Kind: "SSHKey",
		Scope: api.ScopeNamespaced, Aliases: []string{"sshkey"},
	}
)

const (
	machinePath   = "/vm/v1alpha1/tenants/e2etest/virtualmachines"
	containerPath = "/ctr/v1alpha/tenants/e2etest/containers"
	sshKeyPath    = "/vm/v1alpha1/tenants/e2etest/sshkeys"
)

// streamUpgrader answers the handshake of a subresource that carries a stream,
// the way a controller does.
var streamUpgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

// recorded is one request the fake gateway answered, with its body kept.
type recorded struct {
	*http.Request
	Body string
}

type rawResponse struct {
	Status      int
	ContentType string
	Body        string
}

type testEnv struct {
	server   *httptest.Server
	store    *config.Store
	requests []recorded

	// dependencies is what /dependencies answers, by the path of the resource
	// that is asked about.
	dependencies map[string][]api.Dependency
	// dependents is what /dependents answers, by the path of the resource that
	// is asked about.
	dependents map[string][]api.Dependency
	// failures is the answer a path is refused with, by that path.
	failures map[string]api.Response[any]
	// streams is what a subresource that carries a stream sends before it
	// leaves, by the path of that subresource. Every entry is one frame.
	streams map[string][][]byte
	// awaitFrame holds back what a stream sends until the client sent this
	// frame, the way a command that reads to the end of its input waits.
	awaitFrame map[string][]byte
	// raw answers controller endpoints whose successful bodies are not gateway
	// envelopes, such as guest exec, kubeconfig, and Kubernetes resources.
	raw map[string]rawResponse

	// mutex guards received, which a stream fills while the test reads it.
	mutex sync.Mutex
	// received holds the frames a stream was sent, by the path of it.
	received map[string][][]byte
	// object is what one addressed resource answers, by its path.
	object map[string]unstructured.Unstructured
	// objects is what a resource collection answers.
	objects unstructured.UnstructuredList
	// tenants is what /tenants answers.
	tenants []api.Tenant
	// members is what /tenants/{tenant}/members answers.
	members []api.Member
}

// newTestEnv points a profile at a fake gateway that serves an empty result for
// anything the test does not set up.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	env := &testEnv{
		dependencies: map[string][]api.Dependency{},
		dependents:   map[string][]api.Dependency{},
		failures:     map[string]api.Response[any]{},
		streams:      map[string][][]byte{},
		awaitFrame:   map[string][]byte{},
		raw:          map[string]rawResponse{},
		received:     map[string][][]byte{},
		object:       map[string]unstructured.Unstructured{},
		objects:      unstructured.UnstructuredList{},
		tenants:      []api.Tenant{},
		members:      []api.Member{},
	}
	env.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		env.requests = append(env.requests, recorded{Request: r.Clone(r.Context()), Body: string(body)})
		w.Header().Set("Content-Type", "application/json")

		if failure, refused := env.failures[r.URL.Path]; refused {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(failure)
			return
		}

		if websocket.IsWebSocketUpgrade(r) {
			env.serveStream(w, r)
			return
		}
		if response, ok := env.raw[r.URL.Path]; ok {
			if response.ContentType != "" {
				w.Header().Set("Content-Type", response.ContentType)
			}
			status := response.Status
			if status == 0 {
				status = http.StatusOK
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(response.Body))
			return
		}

		switch {
		case r.URL.Path == "/api-resources":
			writeJSON(w, api.APIResourceList{testVPC, testSubnet, testTenantResource, testMachine, testContainer, testSSHKey})

		case strings.HasSuffix(r.URL.Path, "/dependencies"):
			writeJSON(w, env.dependencies[strings.TrimSuffix(r.URL.Path, "/dependencies")])

		case strings.HasSuffix(r.URL.Path, "/dependents"):
			writeJSON(w, env.dependents[strings.TrimSuffix(r.URL.Path, "/dependents")])

		case r.URL.Path == "/tenants" && r.Method == http.MethodGet:
			writeJSON(w, env.tenants)

		case strings.HasSuffix(r.URL.Path, "/members") || strings.Contains(r.URL.Path, "/members/"):
			writeJSON(w, env.members)

		case strings.HasPrefix(r.URL.Path, "/tenants"):
			writeJSON(w, api.Tenant{Name: "made", Phase: "Pending", Role: "owner"})

		default:
			if object, addressed := env.object[r.URL.Path]; addressed {
				writeJSON(w, object)
				return
			}
			writeJSON(w, env.objects)
		}
	}))
	t.Cleanup(env.server.Close)

	dir := t.TempDir()
	t.Setenv("MNI_CONFIG", filepath.Join(dir, "config.yaml"))
	t.Setenv("MNI_CREDENTIALS", filepath.Join(dir, "credentials.yaml"))
	env.store = config.NewStoreAt(filepath.Join(dir, "config.yaml"), filepath.Join(dir, "credentials.yaml"))

	return env
}

// serveStream answers a WebSocket upgrade the way a controller does: it echoes
// the subprotocol back, sends what the test set up and then leaves.
func (e *testEnv) serveStream(w http.ResponseWriter, r *http.Request) {
	header := http.Header{}
	if protocols := websocket.Subprotocols(r); len(protocols) > 0 {
		header.Set("Sec-WebSocket-Protocol", protocols[0])
	}

	conn, err := streamUpgrader.Upgrade(w, r, header)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	awaited := make(chan struct{})
	go e.readStream(conn, r.URL.Path, awaited)

	if _, holds := e.awaitFrame[r.URL.Path]; holds {
		select {
		case <-awaited:
		case <-time.After(5 * time.Second):
			// The frame never came. Answering anyway leaves the test to say
			// what is missing instead of waiting for the whole test run to
			// time out.
		}
	}

	for _, frame := range e.streams[r.URL.Path] {
		if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			return
		}
	}

	goodbye := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	_ = conn.WriteControl(websocket.CloseMessage, goodbye, time.Now().Add(time.Second))
}

// readStream keeps what a client sends, and says when the frame the stream
// waits for arrived.
func (e *testEnv) readStream(conn *websocket.Conn, path string, awaited chan<- struct{}) {
	want, holds := e.awaitFrame[path]
	came := false

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			if holds && !came {
				close(awaited)
			}
			return
		}

		e.mutex.Lock()
		e.received[path] = append(e.received[path], bytes.Clone(payload))
		e.mutex.Unlock()

		if holds && !came && bytes.Equal(payload, want) {
			came = true
			close(awaited)
		}
	}
}

// framesOf returns the frames a stream was sent.
func (e *testEnv) framesOf(path string) [][]byte {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return append([][]byte{}, e.received[path]...)
}

func (e *testEnv) writeContext(t *testing.T, tenant string) {
	t.Helper()

	profile := &config.Config{CurrentContext: "test"}
	profile.Put(config.Context{
		Name:   "test",
		Server: e.server.URL,
		Tenant: tenant,
		OAuth: config.OAuth{
			Issuer:      "https://issuer.test",
			ClientID:    "client-cli-sample",
			RedirectURI: "http://localhost:9876/callback",
		},
	})
	if err := e.store.SaveConfig(profile); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
}

func (e *testEnv) writeCredential(t *testing.T) {
	t.Helper()

	err := e.store.SaveCredential(config.Credential{
		Context:     "test",
		AccessToken: "the-token",
		ExpiresAt:   time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("SaveCredential() error = %v", err)
	}
}

// run runs a command the way a pipe would: nothing can be asked.
func (e *testEnv) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return e.runWith(t, nil, false, args...)
}

// runAnswering runs a command as a terminal would, with the answer to every
// question typed ahead of time.
func (e *testEnv) runAnswering(t *testing.T, answer string, args ...string) (string, error) {
	t.Helper()
	return e.runWith(t, strings.NewReader(answer), true, args...)
}

func (e *testEnv) runWith(t *testing.T, in io.Reader, interactive bool, args ...string) (string, error) {
	t.Helper()

	out := &bytes.Buffer{}
	deps := NewDeps(in, out, out)
	deps.Interactive = interactive

	command := NewCommandFor("test", deps)
	err := command.Run(context.Background(), append([]string{"mni"}, args...))
	return out.String(), err
}

// runSplit runs a command with the two output streams kept apart, the way a
// shell that redirects only one of them sees them.
func (e *testEnv) runSplit(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	return e.runSplitWith(t, nil, args...)
}

// runSplitWith runs a command over an input stream that is no terminal, the way
// a shell that pipes something into it does.
func (e *testEnv) runSplitWith(t *testing.T, in io.Reader, args ...string) (string, string, error) {
	t.Helper()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	deps := NewDeps(in, out, errOut)

	command := NewCommandFor("test", deps)
	err := command.Run(context.Background(), append([]string{"mni"}, args...))
	return out.String(), errOut.String(), err
}

// queryOf returns the query of the first request that reached a path.
func (e *testEnv) queryOf(path string) url.Values {
	for _, req := range e.requests {
		if req.URL.Path == path {
			return req.URL.Query()
		}
	}
	return nil
}

func (e *testEnv) lastPath() string {
	if len(e.requests) == 0 {
		return ""
	}
	return e.requests[len(e.requests)-1].URL.Path
}

// sent reports whether a request of a method reached a path.
func (e *testEnv) sent(method, path string) bool {
	for _, req := range e.requests {
		if req.Method == method && req.URL.Path == path {
			return true
		}
	}
	return false
}

// bodyOf returns the body of the first request of a method to a path.
func (e *testEnv) bodyOf(method, path string) string {
	for _, req := range e.requests {
		if req.Method == method && req.URL.Path == path {
			return req.Body
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, data any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
}

func TestGetAddressesTheTenantOfTheContext(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "e2etest")
	env.writeCredential(t)

	if _, err := env.run(t, "get", "vpcs", "-o", "json"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if want := "/vpc/v1alpha2/tenants/e2etest/vpcs"; env.lastPath() != want {
		t.Errorf("path = %q, want %q", env.lastPath(), want)
	}
}

// TestGetTakesEveryNameOfAResource keeps the forms of a name the command line
// takes. Every form comes from the catalog the server serves, so the CLI holds
// no table of short names of its own.
func TestGetTakesEveryNameOfAResource(t *testing.T) {
	tests := []struct {
		arg  string
		want string
	}{
		{arg: "virtualmachines", want: machinePath},
		{arg: "virtualmachine", want: machinePath},
		{arg: "VirtualMachine", want: machinePath},
		{arg: "VIRTUALMACHINES", want: machinePath},
		{arg: "vm", want: machinePath},
		{arg: "virtualmachines.vm", want: machinePath},
		{arg: "VirtualMachine.vm", want: machinePath},
	}

	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			env := loggedIn(t)

			if _, err := env.run(t, "get", tt.arg, "-o", "json"); err != nil {
				t.Fatalf("run() error = %v", err)
			}
			if env.lastPath() != tt.want {
				t.Errorf("path = %q, want %q", env.lastPath(), tt.want)
			}
		})
	}
}

func TestGetOfAResourceOfAnotherGroupIsNotFound(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "get", "virtualmachines.ctr")

	var noMatch *api.NoResourceMatchError
	if !errors.As(err, &noMatch) {
		t.Fatalf("run() error = %v, want a NoResourceMatchError", err)
	}
	if !strings.Contains(err.Error(), "mni api-resources") {
		t.Errorf("run() error = %q, want it to point at `mni api-resources`", err)
	}
}

func TestTenantFlagAfterTheSubcommandWins(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "e2etest")
	env.writeCredential(t)

	if _, err := env.run(t, "get", "vpc", "-t", "other", "-o", "json"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if want := "/vpc/v1alpha2/tenants/other/vpcs"; env.lastPath() != want {
		t.Errorf("path = %q, want %q", env.lastPath(), want)
	}
}

func TestTenantFlagBeforeTheSubcommandWins(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "e2etest")
	env.writeCredential(t)

	if _, err := env.run(t, "--tenant", "other", "get", "vpcs", "-o", "json"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if want := "/vpc/v1alpha2/tenants/other/vpcs"; env.lastPath() != want {
		t.Errorf("path = %q, want %q", env.lastPath(), want)
	}
}

func TestGetOfAClusterScopedResourceNeedsNoTenant(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "")
	env.writeCredential(t)

	if _, err := env.run(t, "get", "tenants", "-o", "json"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if want := "/auth/v1alpha1/tenants"; env.lastPath() != want {
		t.Errorf("path = %q, want %q", env.lastPath(), want)
	}
}

func TestGetOfANamespacedResourceWithoutATenant(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "")
	env.writeCredential(t)

	_, err := env.run(t, "get", "vpcs")
	if !errors.Is(err, config.ErrNoTenant) {
		t.Fatalf("run() error = %v, want ErrNoTenant", err)
	}
}

func TestRequestsCarryTheStoredToken(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "e2etest")
	env.writeCredential(t)

	if _, err := env.run(t, "get", "vpcs", "-o", "json"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	last := env.requests[len(env.requests)-1]
	if got := last.Header.Get("Authorization"); got != "Bearer the-token" {
		t.Errorf("Authorization = %q, want the stored token", got)
	}
}

func TestGetWithoutASessionAsksForALogin(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "e2etest")

	_, err := env.run(t, "get", "vpcs")
	if !errors.Is(err, auth.ErrLoginRequired) {
		t.Fatalf("run() error = %v, want ErrLoginRequired", err)
	}
	if !strings.Contains(UserFacing(err).Error(), "run `mni login") {
		t.Errorf("UserFacing() = %q, want the advice on its own", UserFacing(err))
	}

	for _, req := range env.requests {
		if req.URL.Path != "/api-resources" {
			t.Errorf("a request went to %q without a token, want only the public catalog", req.URL.Path)
		}
	}
}

func TestGetWithoutASessionAsksForALoginBeforeATenant(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "")

	_, err := env.run(t, "get", "vpcs")
	if !errors.Is(err, auth.ErrLoginRequired) {
		t.Fatalf("run() error = %v, want ErrLoginRequired", err)
	}
	if errors.Is(err, config.ErrNoTenant) {
		t.Errorf("run() error = %v, want the login asked for instead of the tenant", err)
	}
}

func TestContextFlagPicksAnotherContext(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "e2etest")
	env.writeCredential(t)

	_, err := env.run(t, "--context", "nope", "get", "vpcs")
	var notFound *config.ContextNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("run() error = %v, want a ContextNotFoundError", err)
	}
	if notFound.Name != "nope" {
		t.Errorf("Name = %q, want %q", notFound.Name, "nope")
	}
}

func TestWithoutAnyContext(t *testing.T) {
	env := newTestEnv(t)

	if _, err := env.run(t, "get", "vpcs"); !errors.Is(err, config.ErrNoCurrentContext) {
		t.Fatalf("run() error = %v, want ErrNoCurrentContext", err)
	}
}

func TestConfigUseTenant(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "")

	if _, err := env.run(t, "config", "use-tenant", "picked"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	profile, err := env.store.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	target, _ := profile.Find("test")
	if target.Tenant != "picked" {
		t.Errorf("Tenant = %q, want %q", target.Tenant, "picked")
	}
}

func TestConfigGetContextsMarksTheCurrentOne(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "e2etest")

	out, err := env.run(t, "config", "get-contexts")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(out, "*") || !strings.Contains(out, "test") {
		t.Errorf("get-contexts = %q, want the current context marked", out)
	}
}

func TestLogout(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "e2etest")
	env.writeCredential(t)

	if _, err := env.run(t, "logout"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if _, found, err := env.store.Credential("test"); err != nil || found {
		t.Errorf("Credential() = (found %v, err %v), want the session forgotten", found, err)
	}
}

func TestLogoutWithoutASession(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "e2etest")

	out, err := env.run(t, "logout")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(out, "no session") {
		t.Errorf("logout = %q, want it to say there was nothing to forget", out)
	}
}

func TestLoginNeedsAWholeContext(t *testing.T) {
	env := newTestEnv(t)

	_, err := env.run(t, "login", "--context", "new", "--server", env.server.URL, "--issuer", "")
	if err == nil {
		t.Fatal("run() error = nil, want the emptied OAuth setting reported")
	}
	if !strings.Contains(err.Error(), "issuer") {
		t.Errorf("run() error = %q, want it to name what is missing", err)
	}
}

func TestAPIResourcesRunsWithoutASession(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "e2etest")

	out, err := env.run(t, "api-resources")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(out, "vpcs") {
		t.Errorf("api-resources = %q, want the catalog", out)
	}
}

// TestAPIResourcesWritesTheOtherNamesOfAResource keeps the names a user may type
// findable, because a name nobody is told about is a name nobody uses.
func TestAPIResourcesWritesTheOtherNamesOfAResource(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "e2etest")

	out, err := env.run(t, "api-resources")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !strings.Contains(out, "ALIASES") {
		t.Errorf("api-resources = %q, want a column of the other names", out)
	}
	row := rowOf(t, out, "virtualmachines")
	if !strings.Contains(row, "virtualmachine,vm") {
		t.Errorf("row = %q, want the singular name and the alias, without the plural one", row)
	}
}

// rowOf returns the line of a table that names one resource.
func rowOf(t *testing.T, out, resource string) string {
	t.Helper()

	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, resource) {
			return line
		}
	}
	t.Fatalf("output holds no row for %s:\n%s", resource, out)
	return ""
}

// TestOnlyThePublicCommandsRunWithoutASession keeps the list of commands that
// work before a login a decision somebody makes, instead of whatever the order
// of the calls inside an action happens to be.
func TestOnlyThePublicCommandsRunWithoutASession(t *testing.T) {
	public := map[string]bool{
		"login":         true,
		"logout":        true,
		"config":        true,
		"api-resources": true,
		"explain":       true,
		"completion":    true,
		"help":          true,
	}

	root := NewCommandFor("test", NewDeps(nil, io.Discard, io.Discard))
	for _, command := range root.Commands {
		want := !public[command.Name]
		if got := command.Before != nil; got != want {
			t.Errorf("command %q asks for a session = %v, want %v", command.Name, got, want)
		}
	}
}

const vpcPath = "/vpc/v1alpha2/tenants/e2etest/vpcs"

// loggedIn sets up a profile with a session, so that a command can be run.
func loggedIn(t *testing.T) *testEnv {
	t.Helper()

	env := newTestEnv(t)
	env.writeContext(t, "e2etest")
	env.writeCredential(t)
	return env
}

func TestDeleteStopsWhenTheAnswerIsNo(t *testing.T) {
	env := loggedIn(t)

	out, err := env.runAnswering(t, "n\n", "delete", "vpcs", "clidbg-vpc")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if env.sent(http.MethodDelete, vpcPath+"/clidbg-vpc") {
		t.Error("a DELETE was sent although the answer was no")
	}
	if !strings.Contains(out, "not deleted") {
		t.Errorf("delete = %q, want it to say nothing was deleted", out)
	}
}

func TestDeleteStopsWhenTheAnswerIsEmpty(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.runAnswering(t, "\n", "delete", "vpcs", "clidbg-vpc"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if env.sent(http.MethodDelete, vpcPath+"/clidbg-vpc") {
		t.Error("a DELETE was sent although the question was not answered")
	}
}

func TestDeleteGoesOnWhenTheAnswerIsYes(t *testing.T) {
	env := loggedIn(t)

	out, err := env.runAnswering(t, "y\n", "delete", "vpcs", "clidbg-vpc")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !env.sent(http.MethodDelete, vpcPath+"/clidbg-vpc") {
		t.Error("no DELETE was sent although the answer was yes")
	}
	if !strings.Contains(out, "vpcs/clidbg-vpc deleted") {
		t.Errorf("delete = %q, want it to report the delete", out)
	}
}

func TestDeleteShowsWhatItCarriesWithIt(t *testing.T) {
	env := loggedIn(t)
	env.dependents[vpcPath+"/clidbg-vpc"] = []api.Dependency{
		{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "clidbg-subnet"},
	}

	out, err := env.runAnswering(t, "n\n", "delete", "vpcs", "clidbg-vpc")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !strings.Contains(out, "Subnet") || !strings.Contains(out, "clidbg-subnet") {
		t.Errorf("delete = %q, want the dependent named before the question", out)
	}
}

func TestDeleteFollowsTheChainOfDependents(t *testing.T) {
	env := loggedIn(t)
	subnetPath := "/vpc/v1alpha2/tenants/e2etest/subnets"
	env.dependents[vpcPath+"/clidbg-vpc"] = []api.Dependency{
		{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "clidbg-subnet"},
	}
	env.dependents[subnetPath+"/clidbg-subnet"] = []api.Dependency{
		{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "deeper"},
	}

	out, err := env.runAnswering(t, "n\n", "delete", "vpcs", "clidbg-vpc")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !strings.Contains(out, "deeper") {
		t.Errorf("delete = %q, want what the dependent itself carries with it", out)
	}
}

func TestDeleteWithoutATerminalNeedsTheYesFlag(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "delete", "vpcs", "clidbg-vpc")
	if err == nil {
		t.Fatal("run() error = nil, want a delete that cannot ask refused")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("run() error = %q, want it to name --yes", err)
	}
	if env.sent(http.MethodDelete, vpcPath+"/clidbg-vpc") {
		t.Error("a DELETE was sent although nothing could be asked")
	}
}

func TestDeleteWithTheYesFlagAsksNothing(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "delete", "vpcs", "clidbg-vpc", "--yes"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !env.sent(http.MethodDelete, vpcPath+"/clidbg-vpc") {
		t.Error("no DELETE was sent although --yes was given")
	}
	if env.sent(http.MethodGet, vpcPath+"/clidbg-vpc/dependents") {
		t.Error("the dependents were read although nothing was going to be asked")
	}
}

func TestGetTableLeavesAColumnWithoutAValueEmpty(t *testing.T) {
	env := loggedIn(t)
	env.objects = unstructured.UnstructuredList{{
		"apiVersion": "vpc/v1alpha2",
		"kind":       "Vpc",
		"metadata":   map[string]any{"name": "clidbg-vpc"},
		"status":     map[string]any{"phase": "Ready"},
	}}

	out, err := env.run(t, "get", "vpcs")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("get wrote %d lines, want a header and one row:\n%s", len(lines), out)
	}
	if strings.Contains(out, "not found in object") {
		t.Errorf("get = %q, want no lookup failure written to the output", out)
	}
	if !strings.Contains(lines[1], "Ready") {
		t.Errorf("row = %q, want the value the resource has", lines[1])
	}
	if strings.Contains(lines[1], "<none>") {
		t.Errorf("row = %q, want the column without a value left empty", lines[1])
	}
}

func TestGetWithAJSONPathThatMatchesNothingFails(t *testing.T) {
	env := loggedIn(t)
	env.objects = unstructured.UnstructuredList{{
		"metadata": map[string]any{"name": "clidbg-vpc"},
	}}

	out, err := env.run(t, "get", "vpcs", "-o", "jsonpath=.nope")
	if err == nil {
		t.Fatal("run() error = nil, want a path that matches nothing reported")
	}
	if strings.Contains(out, "not found in object") {
		t.Errorf("get wrote %q to the output, want the failure reported as an error only", out)
	}
}

func TestTenantsList(t *testing.T) {
	env := loggedIn(t)
	env.tenants = []api.Tenant{{Name: "e2etest", Phase: "Ready", Role: "owner"}}

	out, err := env.run(t, "tenants")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if env.lastPath() != "/tenants" {
		t.Errorf("path = %q, want %q", env.lastPath(), "/tenants")
	}
	if !strings.Contains(out, "e2etest") || !strings.Contains(out, "owner") {
		t.Errorf("tenants = %q, want the tenant and the role", out)
	}
}

func TestTenantListSubcommandReadsTheSamePath(t *testing.T) {
	env := loggedIn(t)
	env.tenants = []api.Tenant{{Name: "e2etest", Phase: "Ready", Role: "owner"}}

	if _, err := env.run(t, "tenant", "list"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if env.lastPath() != "/tenants" {
		t.Errorf("path = %q, want %q", env.lastPath(), "/tenants")
	}
}

func TestTenantsListAsJSON(t *testing.T) {
	env := loggedIn(t)
	env.tenants = []api.Tenant{{Name: "e2etest", Phase: "Ready", Role: "owner"}}

	out, err := env.run(t, "tenants", "-o", "json")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(out, `"name":"e2etest"`) {
		t.Errorf("tenants -o json = %q, want JSON", out)
	}
}

func TestTenantsCreateSendsTheNameAndTheMetadata(t *testing.T) {
	env := loggedIn(t)

	out, err := env.run(t, "tenants", "create", "clitest", "--display-name", "CLI test", "--description", "for the CLI")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	body := env.bodyOf(http.MethodPost, "/tenants")
	for _, want := range []string{`"name":"clitest"`, `"displayName":"CLI test"`, `"description":"for the CLI"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body = %q, want it to hold %s", body, want)
		}
	}
	if !strings.Contains(out, "created") {
		t.Errorf("create = %q, want it to report the tenant", out)
	}
}

func TestTenantsCreateWithoutAName(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "tenants", "create"); err == nil {
		t.Fatal("run() error = nil, want a name asked for")
	}
}

func TestTenantsDeleteAsksFirst(t *testing.T) {
	env := loggedIn(t)

	out, err := env.runAnswering(t, "n\n", "tenants", "delete", "clitest")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if env.sent(http.MethodDelete, "/tenants/clitest") {
		t.Error("a DELETE was sent although the answer was no")
	}
	if !strings.Contains(out, "not deleted") {
		t.Errorf("delete = %q, want it to say nothing was deleted", out)
	}
}

func TestTenantsDeleteGoesOnWhenAllowed(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.runAnswering(t, "y\n", "tenants", "delete", "clitest"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !env.sent(http.MethodDelete, "/tenants/clitest") {
		t.Error("no DELETE was sent although the answer was yes")
	}
}

func TestTenantsDeleteWithoutATerminalNeedsTheYesFlag(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "tenants", "delete", "clitest")
	if err == nil {
		t.Fatal("run() error = nil, want a delete that cannot ask refused")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("run() error = %q, want it to name --yes", err)
	}
	if env.sent(http.MethodDelete, "/tenants/clitest") {
		t.Error("a DELETE was sent although nothing could be asked")
	}
}

func TestTenantsMembers(t *testing.T) {
	env := loggedIn(t)
	env.members = []api.Member{{User: "u-1234", Roles: []string{"editor"}}}

	out, err := env.run(t, "tenants", "members", "e2etest")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if env.lastPath() != "/tenants/e2etest/members" {
		t.Errorf("path = %q, want %q", env.lastPath(), "/tenants/e2etest/members")
	}
	if !strings.Contains(out, "u-1234") || !strings.Contains(out, "editor") {
		t.Errorf("members = %q, want the member and the role", out)
	}
}

func TestTenantsAddMemberSendsTheUsername(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "tenants", "add-member", "e2etest", "alice", "--role", "editor"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	body := env.bodyOf(http.MethodPost, "/tenants/e2etest/members")
	if !strings.Contains(body, `"username":"alice"`) {
		t.Errorf("body = %q, want the username", body)
	}
	if !strings.Contains(body, `"roles":["editor"]`) {
		t.Errorf("body = %q, want the roles", body)
	}
}

func TestTenantsRemoveMember(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "tenants", "remove-member", "e2etest", "alice"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !env.sent(http.MethodDelete, "/tenants/e2etest/members/alice") {
		t.Errorf("no DELETE reached the member, requests went to %q", env.lastPath())
	}
}

func TestTenantsMembersWithoutATenant(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "tenants", "members"); err == nil {
		t.Fatal("run() error = nil, want a tenant asked for")
	}
}
