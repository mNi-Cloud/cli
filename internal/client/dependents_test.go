package client

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/mNi-Cloud/cli/internal/api"
)

// graph is what a fake gateway answers on the two dependency endpoints, by the
// path of the resource that is asked about.
type graph map[string][]api.Dependency

func newGraphClient(t *testing.T, catalog api.APIResourceList, dependencies, dependents graph) (*Client, *[]capturedRequest) {
	t.Helper()

	server, captured := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api-resources":
			writeEnvelope(t, w, http.StatusOK, api.Response[api.APIResourceList]{Success: true, Data: catalog})

		case strings.HasSuffix(r.URL.Path, "/dependencies"):
			writeEnvelope(t, w, http.StatusOK, api.Response[[]api.Dependency]{
				Success: true,
				Data:    dependencies[strings.TrimSuffix(r.URL.Path, "/dependencies")],
			})

		case strings.HasSuffix(r.URL.Path, "/dependents"):
			writeEnvelope(t, w, http.StatusOK, api.Response[[]api.Dependency]{
				Success: true,
				Data:    dependents[strings.TrimSuffix(r.URL.Path, "/dependents")],
			})

		default:
			t.Errorf("a request reached %q, want only the dependency endpoints", r.URL.Path)
		}
	})

	return newTestClient(t, server, &staticTokens{token: "access"}), captured
}

func TestResourceClientDependenciesReadsWhatAResourceNeeds(t *testing.T) {
	subnet := api.Dependency{APIVersion: "vpc/v1alpha2", Kind: "Vpc", Name: "clidbg-vpc"}
	client, captured := newGraphClient(t,
		api.APIResourceList{namespacedResource},
		graph{"/vpc/v1alpha2/tenants/e2etest/vpcs/a": {subnet}},
		graph{},
	)

	resource, err := client.Resource(namespacedResource, "e2etest")
	if err != nil {
		t.Fatalf("Resource() error = %v", err)
	}

	found, err := resource.Dependencies(context.Background(), "a")
	if err != nil {
		t.Fatalf("Dependencies() error = %v", err)
	}
	if len(found) != 1 || found[0] != subnet {
		t.Errorf("Dependencies() = %+v, want %+v", found, subnet)
	}

	want := "/vpc/v1alpha2/tenants/e2etest/vpcs/a/dependencies"
	if (*captured)[0].path != want {
		t.Errorf("path = %q, want %q", (*captured)[0].path, want)
	}
}

func TestResourceClientDependentsReadsOnlyTheDirectOnes(t *testing.T) {
	subnet := api.Dependency{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "clidbg-subnet"}
	client, captured := newGraphClient(t,
		api.APIResourceList{namespacedResource},
		graph{},
		graph{
			"/vpc/v1alpha2/tenants/e2etest/vpcs/a":                {subnet},
			"/vpc/v1alpha2/tenants/e2etest/subnets/clidbg-subnet": {{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "deeper"}},
		},
	)

	resource, err := client.Resource(namespacedResource, "e2etest")
	if err != nil {
		t.Fatalf("Resource() error = %v", err)
	}

	found, err := resource.Dependents(context.Background(), "a")
	if err != nil {
		t.Fatalf("Dependents() error = %v", err)
	}
	if len(found) != 1 || found[0] != subnet {
		t.Errorf("Dependents() = %+v, want only the direct dependent %+v", found, subnet)
	}
	if len(*captured) != 1 {
		t.Errorf("%d requests were sent, want the chain left alone", len(*captured))
	}
}

func TestClientDependentsFollowsTheChain(t *testing.T) {
	subnetResource := api.APIResource{
		Group: "vpc", Version: "v1alpha2", Resource: "subnets", Kind: "Subnet", Scope: api.ScopeNamespaced,
	}
	client, _ := newGraphClient(t,
		api.APIResourceList{namespacedResource, subnetResource},
		graph{},
		graph{
			"/vpc/v1alpha2/tenants/e2etest/vpcs/a":                {{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "clidbg-subnet"}},
			"/vpc/v1alpha2/tenants/e2etest/subnets/clidbg-subnet": {{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "deeper"}},
		},
	)

	found, err := client.Dependents(context.Background(), namespacedResource, "e2etest", "a")
	if err != nil {
		t.Fatalf("Dependents() error = %v", err)
	}

	names := make([]string, len(found))
	for i, dependency := range found {
		names[i] = dependency.Name
	}
	if len(names) != 2 || names[0] != "clidbg-subnet" || names[1] != "deeper" {
		t.Errorf("Dependents() = %v, want the whole chain", names)
	}
}
