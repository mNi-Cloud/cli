package api

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestAPIResourceDecodesScope(t *testing.T) {
	const body = `{
		"group": "vpc",
		"version": "v1alpha2",
		"resource": "vpcs",
		"singular": "vpc",
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
	if resource.Singular != "vpc" {
		t.Errorf("Singular = %q, want %q", resource.Singular, "vpc")
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

// servedCatalog holds what a running api-gateway publishes, with the singular
// names it publishes once the CRDs are read for them.
func servedCatalog() APIResourceList {
	return APIResourceList{
		{Group: "vpc", Version: "v1alpha2", Resource: "routetables", Singular: "routetable", Kind: "RouteTable", Aliases: []string{"rt"}},
		{Group: "vpc", Version: "v1alpha2", Resource: "subnets", Singular: "subnet", Kind: "Subnet", Aliases: []string{"subnet"}},
		{Group: "vpc", Version: "v1alpha2", Resource: "vpcs", Singular: "vpc", Kind: "Vpc", Aliases: []string{"vpc"}},
		{Group: "vm", Version: "v1alpha1", Resource: "images", Singular: "image", Kind: "Image"},
		{Group: "vm", Version: "v1alpha1", Resource: "virtualmachines", Singular: "virtualmachine", Kind: "VirtualMachine"},
		{Group: "auth", Version: "v1alpha1", Resource: "roles", Singular: "role", Kind: "Role", Aliases: []string{"role"}},
	}
}

func TestParseResourceRequest(t *testing.T) {
	tests := []struct {
		arg  string
		want ResourceRequest
	}{
		{arg: "virtualmachines", want: ResourceRequest{Name: "virtualmachines"}},
		{arg: "VirtualMachine", want: ResourceRequest{Name: "virtualmachine"}},
		{arg: "VIRTUALMACHINES", want: ResourceRequest{Name: "virtualmachines"}},
		{arg: "virtualmachines.vm", want: ResourceRequest{Name: "virtualmachines", Group: "vm"}},
		{arg: "images.example.com", want: ResourceRequest{Name: "images", Group: "example.com"}},
		{arg: "", want: ResourceRequest{}},
	}

	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			if got := ParseResourceRequest(tt.arg); got != tt.want {
				t.Errorf("ParseResourceRequest(%q) = %+v, want %+v", tt.arg, got, tt.want)
			}
		})
	}
}

func TestResourceRequestString(t *testing.T) {
	tests := []struct {
		request ResourceRequest
		want    string
	}{
		{request: ResourceRequest{Name: "vpcs"}, want: "vpcs"},
		{request: ResourceRequest{Name: "vpcs", Group: "vpc"}, want: "vpcs.vpc"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.request.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAPIResourceListFindByName(t *testing.T) {
	tests := []struct {
		arg  string
		want string
	}{
		{arg: "virtualmachines", want: "virtualmachines.vm"},
		{arg: "virtualmachine", want: "virtualmachines.vm"},
		{arg: "VirtualMachine", want: "virtualmachines.vm"},
		{arg: "VIRTUALMACHINES", want: "virtualmachines.vm"},
		{arg: "rt", want: "routetables.vpc"},
		{arg: "vpc", want: "vpcs.vpc"},
		{arg: "Vpc", want: "vpcs.vpc"},
		{arg: "role", want: "roles.auth"},
		{arg: "virtualmachines.vm", want: "virtualmachines.vm"},
		{arg: "VirtualMachine.vm", want: "virtualmachines.vm"},
	}

	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			found, err := servedCatalog().FindByName(tt.arg)
			if err != nil {
				t.Fatalf("FindByName(%q) error = %v", tt.arg, err)
			}
			if found.FullName() != tt.want {
				t.Errorf("FindByName(%q) = %q, want %q", tt.arg, found.FullName(), tt.want)
			}
		})
	}
}

func TestAPIResourceListFindByNameReportsANameNoResourceAnswersTo(t *testing.T) {
	tests := []string{"nosuch", "virtualmachines.ctr", "vm", ""}

	for _, arg := range tests {
		t.Run(arg, func(t *testing.T) {
			_, err := servedCatalog().FindByName(arg)

			var noMatch *NoResourceMatchError
			if !errors.As(err, &noMatch) {
				t.Fatalf("FindByName(%q) error = %v, want a NoResourceMatchError", arg, err)
			}
			if !strings.Contains(noMatch.Error(), "mni api-resources") {
				t.Errorf("error = %q, want it to point at `mni api-resources`", noMatch)
			}
		})
	}
}

func TestAPIResourceListFindByNameReportsAnAmbiguousName(t *testing.T) {
	list := APIResourceList{
		{Group: "vm", Version: "v1alpha1", Resource: "images", Singular: "image", Kind: "Image"},
		{Group: "ctr", Version: "v1alpha1", Resource: "images", Singular: "image", Kind: "Image"},
	}

	_, err := list.FindByName("images")

	var ambiguous *AmbiguousResourceError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("FindByName(\"images\") error = %v, want an AmbiguousResourceError", err)
	}
	for _, want := range []string{"images.vm", "images.ctr"} {
		if !strings.Contains(ambiguous.Error(), want) {
			t.Errorf("error = %q, want it to list %q", ambiguous, want)
		}
	}

	found, err := list.FindByName("images.ctr")
	if err != nil {
		t.Fatalf("FindByName(\"images.ctr\") error = %v", err)
	}
	if found.FullName() != "images.ctr" {
		t.Errorf("FindByName(\"images.ctr\") = %q, want %q", found.FullName(), "images.ctr")
	}
}

// TestAPIResourceListFindByNamePrefersTheEarlierAxis holds the order the names
// of a resource are tried in, so that a short name added to some CRD later
// cannot take a name another resource already answers to over.
func TestAPIResourceListFindByNamePrefersTheEarlierAxis(t *testing.T) {
	tests := []struct {
		name    string
		catalog APIResourceList
		arg     string
		want    string
	}{
		{
			name: "a plural name beats an alias",
			catalog: APIResourceList{
				{Group: "vpc", Version: "v1alpha2", Resource: "routetables", Singular: "routetable", Kind: "RouteTable", Aliases: []string{"rt"}},
				{Group: "ctr", Version: "v1alpha1", Resource: "runtimes", Singular: "runtime", Kind: "Runtime", Aliases: []string{"routetables"}},
			},
			arg:  "routetables",
			want: "routetables.vpc",
		},
		{
			name: "a singular name beats a kind",
			catalog: APIResourceList{
				{Group: "vm", Version: "v1alpha1", Resource: "images", Singular: "image", Kind: "Image"},
				{Group: "ctr", Version: "v1alpha1", Resource: "containerimages", Singular: "containerimage", Kind: "Image"},
			},
			arg:  "image",
			want: "images.vm",
		},
		{
			name: "a kind beats an alias",
			catalog: APIResourceList{
				{Group: "vm", Version: "v1alpha1", Resource: "virtualmachines", Singular: "virtualmachine", Kind: "VirtualMachine"},
				{Group: "ctr", Version: "v1alpha1", Resource: "containers", Singular: "container", Kind: "Container", Aliases: []string{"virtualmachine"}},
			},
			arg:  "virtualmachine",
			want: "virtualmachines.vm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := tt.catalog.FindByName(tt.arg)
			if err != nil {
				t.Fatalf("FindByName(%q) error = %v", tt.arg, err)
			}
			if found.FullName() != tt.want {
				t.Errorf("FindByName(%q) = %q, want %q", tt.arg, found.FullName(), tt.want)
			}
		})
	}
}

// TestAPIResourceListFindByNameRefusesTheEmptyName keeps a name the catalog
// leaves out, such as the singular of a resource the server serves no singular
// for, from answering to the empty name.
func TestAPIResourceListFindByNameRefusesTheEmptyName(t *testing.T) {
	list := APIResourceList{
		{Group: "vm", Version: "v1alpha1", Resource: "virtualmachines", Kind: "VirtualMachine"},
		{Group: "vpc", Version: "v1alpha2", Resource: "vpcs"},
	}

	for _, arg := range []string{"", ".vm"} {
		if found, err := list.FindByName(arg); err == nil {
			t.Errorf("FindByName(%q) = %q, want the empty name refused", arg, found.FullName())
		}
	}
}

// TestAPIResourceListFindByNameAcceptsAResourceServedAtTwoVersions keeps one
// resource the server serves twice from reading as two resources.
func TestAPIResourceListFindByNameAcceptsAResourceServedAtTwoVersions(t *testing.T) {
	list := APIResourceList{
		{Group: "vpc", Version: "v1alpha1", Resource: "vpcs", Singular: "vpc", Kind: "Vpc"},
		{Group: "vpc", Version: "v1alpha2", Resource: "vpcs", Singular: "vpc", Kind: "Vpc"},
	}

	found, err := list.FindByName("vpcs")
	if err != nil {
		t.Fatalf("FindByName(\"vpcs\") error = %v", err)
	}
	if found.Version != "v1alpha1" {
		t.Errorf("Version = %q, want %q, the first the catalog lists", found.Version, "v1alpha1")
	}
}

// TestEveryNameOfTheRecordedCatalogResolves keeps every name `mni api-resources`
// lists a resource under a name that reaches that resource, so that the table
// tells the user nothing it cannot keep.
func TestEveryNameOfTheRecordedCatalogResolves(t *testing.T) {
	catalog := realCatalog(t)

	for _, resource := range catalog {
		for _, name := range append([]string{resource.Resource}, resource.AlternateNames()...) {
			found, err := catalog.FindByName(name)
			if err != nil {
				t.Errorf("FindByName(%q) error = %v", name, err)
				continue
			}
			if found.FullName() != resource.FullName() {
				t.Errorf("FindByName(%q) = %q, want %q", name, found.FullName(), resource.FullName())
			}
		}
	}
}

func TestAPIResourceAlternateNames(t *testing.T) {
	tests := []struct {
		name     string
		resource APIResource
		want     []string
	}{
		{
			name:     "every name the catalog carries",
			resource: APIResource{Resource: "routetables", Singular: "routetable", Kind: "RouteTable", Aliases: []string{"rt"}},
			want:     []string{"routetable", "rt"},
		},
		{
			name:     "the same name is listed once",
			resource: APIResource{Resource: "vpcs", Singular: "vpc", Kind: "Vpc", Aliases: []string{"vpc"}},
			want:     []string{"vpc"},
		},
		{
			name:     "the plural name is left out",
			resource: APIResource{Resource: "images", Singular: "image", Kind: "Image", Aliases: []string{"images"}},
			want:     []string{"image"},
		},
		{
			name:     "a resource with nothing but a plural name",
			resource: APIResource{Resource: "vpcs"},
			want:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resource.AlternateNames(); !slices.Equal(got, tt.want) {
				t.Errorf("AlternateNames() = %v, want %v", got, tt.want)
			}
		})
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
