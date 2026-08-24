package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestGuestExecUsesTheProductionContract(t *testing.T) {
	server, captured := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stdout":"ok\n","stderr":"warn\n","exitCode":7}`))
	})
	client := newTestClient(t, server, &staticTokens{token: "access"})

	response, err := client.GuestExec(context.Background(), "tenant-a", "vm-a", GuestExecRequest{
		Argv: []string{"printf", "%s", "a b"}, StdinBase64: "eA==", WorkingDirectory: "/tmp",
	})
	if err != nil {
		t.Fatalf("GuestExec() error = %v", err)
	}
	if response.ExitCode != 7 || response.Stdout != "ok\n" || response.Stderr != "warn\n" {
		t.Fatalf("GuestExec() = %+v", response)
	}
	call := (*captured)[0]
	if call.method != http.MethodPost || call.path != "/vm/v1alpha1/tenants/tenant-a/virtualmachines/vm-a/guest-exec" {
		t.Errorf("request = %s %s", call.method, call.path)
	}
	var body GuestExecRequest
	if err := json.Unmarshal([]byte(call.body), &body); err != nil {
		t.Fatal(err)
	}
	if strings.Join(body.Argv, "|") != "printf|%s|a b" || body.StdinBase64 != "eA==" || body.WorkingDirectory != "/tmp" {
		t.Errorf("body = %+v", body)
	}
}

func TestVMShellPreservesWebSocketFrameTypes(t *testing.T) {
	var path, authorization string
	server, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path, authorization = r.URL.Path, r.Header.Get("Authorization")
		upgrader := websocket.Upgrader{Subprotocols: []string{binarySubprotocol}, CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Upgrade() error = %v", err)
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"exit","exitCode":0}`))
	})
	client := newTestClient(t, server, &staticTokens{token: "access"})
	stream, err := client.VMShell(context.Background(), "e2etest", "vm-a")
	if err != nil {
		t.Fatalf("VMShell() error = %v", err)
	}
	defer stream.Close()
	text, payload, err := stream.ReadFrame()
	if err != nil || !text || !strings.Contains(string(payload), `"type":"exit"`) {
		t.Fatalf("ReadFrame() = %t, %q, %v", text, payload, err)
	}
	if path != "/vm/v1alpha1/tenants/e2etest/virtualmachines/vm-a/shell" || authorization != "Bearer access" {
		t.Errorf("handshake = %q, Authorization %q", path, authorization)
	}
}

func TestKubernetesOperationsUseRawResponses(t *testing.T) {
	server, captured := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/kubeconfig") {
			w.Header().Set("Content-Type", "application/yaml")
			w.Header().Set("X-Kubeconfig-Expires-At", "2026-08-25T12:00:00Z")
			_, _ = w.Write([]byte("apiVersion: v1\n"))
			return
		}
		_, _ = w.Write([]byte(`{"kind":"PodList","items":[]}`))
	})
	client := newTestClient(t, server, &staticTokens{token: "access"})

	kubeconfig, headers, err := client.Kubeconfig(context.Background(), "e2etest", "cluster-a")
	if err != nil || string(kubeconfig) != "apiVersion: v1\n" || headers.Get("X-Kubeconfig-Expires-At") == "" {
		t.Fatalf("Kubeconfig() = %q, %v, %v", kubeconfig, headers, err)
	}
	query := url.Values{"namespace": {"default"}, "labelSelector": {"app=web"}, "limit": {"50"}}
	raw, err := client.ClusterResources(context.Background(), "e2etest", "cluster-a", []string{"pods", "web-1"}, query)
	if err != nil || !strings.Contains(string(raw), "PodList") {
		t.Fatalf("ClusterResources() = %q, %v", raw, err)
	}
	call := (*captured)[1]
	if call.path != "/k8s/v1alpha1/tenants/e2etest/clusters/cluster-a/resources/pods/web-1" {
		t.Errorf("path = %q", call.path)
	}
	if call.query.Get("namespace") != "default" || call.query.Get("labelSelector") != "app=web" || call.query.Get("limit") != "50" {
		t.Errorf("query = %v", call.query)
	}
}

func TestCloudShellSessionLifecycleUsesRawJSON(t *testing.T) {
	server, captured := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"sessions":[{"id":"s1","subnet":"subnet-a","phase":"Running","createdAt":"now"}]}`))
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"s2","subnet":"subnet-b","phase":"Pending","createdAt":"now"}`))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	client := newTestClient(t, server, &staticTokens{token: "access"})

	sessions, err := client.CloudShellSessions(context.Background(), "e2etest")
	if err != nil || len(sessions) != 1 || sessions[0].ID != "s1" {
		t.Fatalf("CloudShellSessions() = %+v, %v", sessions, err)
	}
	created, err := client.CreateCloudShellSession(context.Background(), "e2etest", "subnet-b")
	if err != nil || created.ID != "s2" {
		t.Fatalf("CreateCloudShellSession() = %+v, %v", created, err)
	}
	if err := client.DeleteCloudShellSession(context.Background(), "e2etest", "s2"); err != nil {
		t.Fatalf("DeleteCloudShellSession() error = %v", err)
	}
	if (*captured)[2].path != "/cs/v1alpha1/tenants/e2etest/sessions/s2" {
		t.Errorf("delete path = %q", (*captured)[2].path)
	}
}

func TestRawOperationReportsDirectControllerErrors(t *testing.T) {
	server, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"CloudShell is provisioning"}`))
	})
	client := newTestClient(t, server, &staticTokens{token: "access"})
	_, err := client.CloudShellSessions(context.Background(), "e2etest")
	if err == nil || !strings.Contains(err.Error(), "provisioning") {
		t.Fatalf("CloudShellSessions() error = %v", err)
	}
}
