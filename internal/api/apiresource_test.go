package api

import (
	"encoding/json"
	"testing"
)

func TestAPIResourceDecodesScope(t *testing.T) {
	const body = `{
		"group": "vpc",
		"version": "v1alpha2",
		"resource": "vpcs",
		"kind": "Vpc",
		"scope": "Namespaced",
		"aliases": ["vpc"],
		"additionalPrinterColumns": [{"name": "Cidr", "type": "string", "jsonPath": ".spec.cidr"}]
	}`

	var resource APIResource
	if err := json.Unmarshal([]byte(body), &resource); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if resource.Scope != ScopeNamespaced {
		t.Errorf("Scope = %q, want %q", resource.Scope, ScopeNamespaced)
	}
	if !resource.Namespaced() {
		t.Error("Namespaced() = false, want true")
	}
	if len(resource.AdditionalPrinterColumns) != 1 {
		t.Fatalf("AdditionalPrinterColumns length = %d, want 1", len(resource.AdditionalPrinterColumns))
	}
	if resource.AdditionalPrinterColumns[0].JSONPath != ".spec.cidr" {
		t.Errorf("JSONPath = %q, want %q", resource.AdditionalPrinterColumns[0].JSONPath, ".spec.cidr")
	}
}

func TestAPIResourceClusterScope(t *testing.T) {
	resource := APIResource{Scope: ScopeCluster}
	if resource.Namespaced() {
		t.Error("Namespaced() = true for a cluster scoped resource, want false")
	}
}

func TestAPIResourceMatches(t *testing.T) {
	resource := APIResource{Resource: "vpcs", Kind: "Vpc", Aliases: []string{"vpc"}}

	tests := []struct {
		name string
		want bool
	}{
		{name: "vpcs", want: true},
		{name: "vpc", want: true},
		{name: "Vpc", want: false},
		{name: "subnets", want: false},
		{name: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resource.Matches(tt.name); got != tt.want {
				t.Errorf("Matches(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestAPIResourceListFindByName(t *testing.T) {
	list := APIResourceList{
		{Group: "vpc", Version: "v1alpha2", Resource: "vpcs", Kind: "Vpc", Aliases: []string{"vpc"}},
		{Group: "vm", Version: "v1alpha1", Resource: "virtualmachines", Kind: "VirtualMachine", Aliases: []string{"vm"}},
	}

	found, ok := list.FindByName("vm")
	if !ok {
		t.Fatal("FindByName(\"vm\") reported not found")
	}
	if found.Kind != "VirtualMachine" {
		t.Errorf("Kind = %q, want %q", found.Kind, "VirtualMachine")
	}

	if _, ok := list.FindByName("nope"); ok {
		t.Error("FindByName(\"nope\") reported found")
	}
}

func TestAPIResourceListFindByKind(t *testing.T) {
	list := APIResourceList{
		{Group: "vpc", Version: "v1alpha2", Resource: "vpcs", Kind: "Vpc"},
		{Group: "vpc", Version: "v1alpha1", Resource: "vpcs", Kind: "Vpc"},
	}

	found, ok := list.FindByKind("vpc", "v1alpha1", "Vpc")
	if !ok {
		t.Fatal("FindByKind() reported not found")
	}
	if found.Version != "v1alpha1" {
		t.Errorf("Version = %q, want %q", found.Version, "v1alpha1")
	}

	if _, ok := list.FindByKind("vpc", "v1alpha3", "Vpc"); ok {
		t.Error("FindByKind() matched a version the server does not serve")
	}
}
