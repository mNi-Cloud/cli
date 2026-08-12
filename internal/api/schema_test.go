package api

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// realCatalog reads the answer a running api-gateway gave to /api-resources.
func realCatalog(t *testing.T) APIResourceList {
	t.Helper()

	raw, err := os.ReadFile("testdata/api-resources.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var answer Response[APIResourceList]
	if err := json.Unmarshal(raw, &answer); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !answer.Success {
		t.Fatal("the recorded answer reports no success")
	}
	return answer.Data
}

func resourceNamed(t *testing.T, catalog APIResourceList, name string) APIResource {
	t.Helper()

	resource, ok := catalog.FindByName(name)
	if !ok {
		t.Fatalf("the recorded catalog serves no %s", name)
	}
	return resource
}

func TestCatalogCarriesTheSchemasOfEveryResource(t *testing.T) {
	for _, resource := range realCatalog(t) {
		if resource.SpecSchema == nil {
			t.Errorf("%s carries no spec schema", resource.Resource)
		}
		if resource.StatusSchema == nil {
			t.Errorf("%s carries no status schema", resource.Resource)
		}
	}
}

func TestSchemaOfASection(t *testing.T) {
	subnets := resourceNamed(t, realCatalog(t), "subnets")

	spec, err := subnets.Schema(SpecSchemaSection)
	if err != nil {
		t.Fatalf("Schema(spec) error = %v", err)
	}
	if _, ok := spec.Field("cidr"); !ok {
		t.Error("the spec of a Subnet holds no cidr")
	}

	status, err := subnets.Schema(StatusSchemaSection)
	if err != nil {
		t.Fatalf("Schema(status) error = %v", err)
	}
	if _, ok := status.Field("phase"); !ok {
		t.Error("the status of a Subnet holds no phase")
	}
}

func TestSchemaOfAnUnknownSection(t *testing.T) {
	subnets := resourceNamed(t, realCatalog(t), "subnets")

	if _, err := subnets.Schema("metadata"); err == nil {
		t.Error("Schema() error = nil, want an unknown section refused")
	}
}

func TestSchemaReadsRequiredFields(t *testing.T) {
	subnets := resourceNamed(t, realCatalog(t), "subnets")
	spec, err := subnets.Schema(SpecSchemaSection)
	if err != nil {
		t.Fatalf("Schema() error = %v", err)
	}

	if !spec.Requires("cidr") {
		t.Error("cidr is required of a Subnet but is not reported as such")
	}
	if spec.Requires("routeTable") {
		t.Error("routeTable is optional on a Subnet but is reported as required")
	}
}

func TestSchemaReadsDescriptionsAndEnums(t *testing.T) {
	catalog := realCatalog(t)

	subnetSpec, _ := resourceNamed(t, catalog, "subnets").Schema(SpecSchemaSection)
	routeTable, ok := subnetSpec.Field("routeTable")
	if !ok {
		t.Fatal("the spec of a Subnet holds no routeTable")
	}
	if routeTable.Description == "" {
		t.Error("routeTable carries no description")
	}

	vpcStatus, _ := resourceNamed(t, catalog, "vpcs").Schema(StatusSchemaSection)
	phase, ok := vpcStatus.Field("phase")
	if !ok {
		t.Fatal("the status of a Vpc holds no phase")
	}
	if len(phase.Enum) != 5 {
		t.Errorf("phase holds %d values, want the five phases of the recorded catalog", len(phase.Enum))
	}
	if phase.Default != "Pending" {
		t.Errorf("phase default = %v, want Pending", phase.Default)
	}
}

func TestSchemaTypeName(t *testing.T) {
	catalog := realCatalog(t)
	machineSpec, _ := resourceNamed(t, catalog, "virtualmachines").Schema(SpecSchemaSection)
	vpcStatus, _ := resourceNamed(t, catalog, "vpcs").Schema(StatusSchemaSection)

	tests := []struct {
		name   string
		schema *Schema
		want   string
	}{
		{name: "object", schema: machineSpec, want: "object"},
		{name: "string", schema: fieldOf(t, machineSpec, "subnet"), want: "string"},
		{name: "array of objects", schema: fieldOf(t, machineSpec, "additionalDisks"), want: "[]object"},
		{name: "either of two types", schema: fieldOf(t, machineSpec, "cores"), want: "integer|string"},
		{name: "array of conditions", schema: fieldOf(t, vpcStatus, "conditions"), want: "[]object"},
		{name: "integer", schema: fieldOf(t, vpcStatus, "observedGeneration"), want: "integer"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.schema.TypeName(); got != test.want {
				t.Errorf("TypeName() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSchemaResolvesAFieldPath(t *testing.T) {
	spec, _ := resourceNamed(t, realCatalog(t), "virtualmachines").Schema(SpecSchemaSection)

	resolved, err := spec.Resolve([]string{"rootVolume", "source", "name"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := resolved.TypeName(); got != "string" {
		t.Errorf("TypeName() = %q, want %q", got, "string")
	}
}

// A field of the objects a list holds is named without saying that the field
// sits in a list, the way a manifest names it.
func TestSchemaResolvesThroughAList(t *testing.T) {
	spec, _ := resourceNamed(t, realCatalog(t), "routetables").Schema(SpecSchemaSection)

	resolved, err := spec.Resolve([]string{"routes", "target", "type"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(resolved.Enum) != 2 {
		t.Errorf("Enum = %v, want the two kinds of route target", resolved.Enum)
	}
}

func TestSchemaResolveNamesTheStepThatIsMissing(t *testing.T) {
	spec, _ := resourceNamed(t, realCatalog(t), "virtualmachines").Schema(SpecSchemaSection)

	_, err := spec.Resolve([]string{"rootVolume", "nowhere", "name"})
	if err == nil {
		t.Fatal("Resolve() error = nil, want a path that names nothing refused")
	}
	if !strings.Contains(err.Error(), "rootVolume.nowhere") {
		t.Errorf("Resolve() error = %q, want it to name the step that went wrong", err)
	}
}

func TestSchemaResolveOfAnEmptyPath(t *testing.T) {
	spec, _ := resourceNamed(t, realCatalog(t), "subnets").Schema(SpecSchemaSection)

	resolved, err := spec.Resolve(nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved != spec {
		t.Error("Resolve(nil) answered another schema, want the schema itself")
	}
}

func TestSchemaFields(t *testing.T) {
	spec, _ := resourceNamed(t, realCatalog(t), "subnets").Schema(SpecSchemaSection)

	want := []string{"cidr", "routeTable", "vpc"}
	got := spec.Fields()
	if len(got) != len(want) {
		t.Fatalf("Fields() = %v, want %v", got, want)
	}
	for index, name := range want {
		if got[index].Name != name {
			t.Errorf("Fields()[%d].Name = %q, want %q", index, got[index].Name, name)
		}
		if got[index].Schema == nil {
			t.Errorf("Fields()[%d].Schema is missing", index)
		}
	}
}

// A field of the objects a list holds is read off the list itself, so that a
// list needs no step of its own.
func TestSchemaFieldsOfAList(t *testing.T) {
	spec, _ := resourceNamed(t, realCatalog(t), "routetables").Schema(SpecSchemaSection)
	routes := fieldOf(t, spec, "routes")

	want := []string{"destination", "target"}
	got := routes.Fields()
	if len(got) != len(want) {
		t.Fatalf("Fields() = %v, want %v", got, want)
	}
	for index, name := range want {
		if got[index].Name != name {
			t.Errorf("Fields()[%d].Name = %q, want %q", index, got[index].Name, name)
		}
	}
}

func fieldOf(t *testing.T, schema *Schema, name string) *Schema {
	t.Helper()

	field, ok := schema.Field(name)
	if !ok {
		t.Fatalf("the schema holds no field named %q", name)
	}
	return field
}
