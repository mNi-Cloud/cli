package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestKubeconfigWritesTheRawYAML(t *testing.T) {
	env := loggedIn(t)
	path := "/k8s/v1alpha1/tenants/e2etest/clusters/cluster-a/kubeconfig"
	env.raw[path] = rawResponse{ContentType: "application/yaml", Body: "apiVersion: v1\nclusters: []\n"}
	out, err := env.run(t, "k8s", "kubeconfig", "cluster-a")
	if err != nil || out != "apiVersion: v1\nclusters: []\n" {
		t.Fatalf("kubeconfig output = %q, error = %v", out, err)
	}
	if !env.sent(http.MethodPost, path) {
		t.Errorf("no POST reached %q", path)
	}
}

func TestClusterResourcesCarriesFilters(t *testing.T) {
	env := loggedIn(t)
	path := "/k8s/v1alpha1/tenants/e2etest/clusters/cluster-a/resources/pods/web"
	env.raw[path] = rawResponse{Body: `{"kind":"Pod","metadata":{"name":"web"}}`}
	out, err := env.run(t, "k8s", "resources", "cluster-a", "pods", "web", "--namespace", "default", "--label-selector", "app=web", "--limit", "10")
	if err != nil || !strings.Contains(out, `"kind": "Pod"`) {
		t.Fatalf("resources output = %q, error = %v", out, err)
	}
	request := env.requests[len(env.requests)-1]
	if request.URL.Query().Get("namespace") != "default" || request.URL.Query().Get("labelSelector") != "app=web" || request.URL.Query().Get("limit") != "10" {
		t.Errorf("query = %v", request.URL.Query())
	}
}
