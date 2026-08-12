package cli

import (
	"strings"
	"testing"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/unstructured"
)

func describedVPC() unstructured.Unstructured {
	return unstructured.Unstructured{
		"apiVersion": "vpc/v1alpha2",
		"kind":       "Vpc",
		"metadata": map[string]any{
			"name":            "clidesc",
			"namespace":       "21f40d54-9bc9-4e47-9fe2-91908e6f564d",
			"uid":             "3efdd940-063c-46d3-98eb-57d21178e4db",
			"resourceVersion": "1990726",
			"displayName":     "CLI describe demo",
			"labels":          map[string]any{"env": "dev"},
		},
		"spec": map[string]any{"enforceSecurityGroups": true},
		"status": map[string]any{
			"backingVpc": "a20d44eb",
			"phase":      "Ready",
			"conditions": []any{
				map[string]any{
					"type":               "Available",
					"status":             "True",
					"reason":             "OK",
					"message":            "Resource is available",
					"lastTransitionTime": "2026-08-08T09:38:57Z",
				},
			},
		},
	}
}

func TestDescribeReadsTheResourceAndBothSidesOfItsGraph(t *testing.T) {
	env := loggedIn(t)
	env.object[vpcPath+"/clidesc"] = describedVPC()
	env.dependencies[vpcPath+"/clidesc"] = []api.Dependency{
		{APIVersion: "vpc/v1alpha2", Kind: "Vpc", Name: "shared"},
	}
	env.dependents[vpcPath+"/clidesc"] = []api.Dependency{
		{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "clidesc-subnet"},
	}

	out, err := env.run(t, "describe", "vpcs", "clidesc")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	for _, want := range []string{"clidesc", "Ready", "Available", "shared", "clidesc-subnet"} {
		if !strings.Contains(out, want) {
			t.Errorf("describe = %q, want it to hold %q", out, want)
		}
	}
	if !env.sent("GET", vpcPath+"/clidesc/dependencies") {
		t.Error("no request reached the dependencies of the resource")
	}
	if !env.sent("GET", vpcPath+"/clidesc/dependents") {
		t.Error("no request reached the dependents of the resource")
	}
}

func TestDescribeShowsNoNamespace(t *testing.T) {
	env := loggedIn(t)
	env.object[vpcPath+"/clidesc"] = describedVPC()

	out, err := env.run(t, "describe", "vpcs", "clidesc")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if strings.Contains(out, "21f40d54-9bc9-4e47-9fe2-91908e6f564d") {
		t.Errorf("describe = %q, want the namespace of the tenant kept inside the gateway", out)
	}
}

func TestDescribeNeedsAResourceAndAName(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "describe", "vpcs"); err == nil {
		t.Error("run() error = nil, want a name asked for")
	}
	if _, err := env.run(t, "describe"); err == nil {
		t.Error("run() error = nil, want a resource asked for")
	}
}

func TestDescribeOfAnUnknownResource(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "describe", "nosuch", "name")
	if err == nil {
		t.Fatal("run() error = nil, want an unknown resource refused")
	}
	if !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("run() error = %q, want it to name the resource", err)
	}
}
