package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/unstructured"
)

func testObject(name, cidr string) unstructured.Unstructured {
	return unstructured.Unstructured{
		"apiVersion": "vpc/v1alpha2",
		"kind":       "Vpc",
		"metadata":   map[string]any{"name": name},
		"spec":       map[string]any{"cidr": cidr},
	}
}

var testResource = api.APIResource{
	Group:    "vpc",
	Version:  "v1alpha2",
	Resource: "vpcs",
	Kind:     "Vpc",
	Scope:    api.ScopeNamespaced,
	AdditionalPrinterColumns: []api.AdditionalPrinterColumn{
		{Name: "Cidr", Type: "string", JSONPath: ".spec.cidr"},
	},
}

func TestPrintTable(t *testing.T) {
	out := &bytes.Buffer{}

	err := NewPrinter(out).PrintTable(testResource, []unstructured.Unstructured{
		testObject("a", "10.0.0.0/16"),
		testObject("b", "10.1.0.0/16"),
	})
	if err != nil {
		t.Fatalf("PrintTable() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("PrintTable() wrote %d lines, want a header and two rows\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "NAME") || !strings.Contains(lines[0], "CIDR") {
		t.Errorf("header = %q, want the name and the extra column", lines[0])
	}
	if !strings.Contains(lines[1], "10.0.0.0/16") {
		t.Errorf("first row = %q, want the value of the extra column", lines[1])
	}
}

func TestPrintJSON(t *testing.T) {
	out := &bytes.Buffer{}

	if err := NewPrinter(out).Print(testObject("a", "10.0.0.0/16"), FormatJSON); err != nil {
		t.Fatalf("Print() error = %v", err)
	}
	if !strings.Contains(out.String(), `"kind":"Vpc"`) {
		t.Errorf("Print() = %q, want JSON", out)
	}
}

func TestPrintYAML(t *testing.T) {
	out := &bytes.Buffer{}

	if err := NewPrinter(out).Print(testObject("a", "10.0.0.0/16"), FormatYAML); err != nil {
		t.Fatalf("Print() error = %v", err)
	}
	if !strings.Contains(out.String(), "kind: Vpc") {
		t.Errorf("Print() = %q, want YAML", out)
	}
}

func TestPrintJSONPath(t *testing.T) {
	out := &bytes.Buffer{}

	if err := NewPrinter(out).Print(testObject("a", "10.0.0.0/16"), JSONPathPrefix+".spec.cidr"); err != nil {
		t.Fatalf("Print() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "10.0.0.0/16" {
		t.Errorf("Print() = %q, want the value at the path", out)
	}
}

func TestPrintJSONPathThatMatchesNothing(t *testing.T) {
	out := &bytes.Buffer{}

	if err := NewPrinter(out).Print(testObject("a", "10.0.0.0/16"), JSONPathPrefix+".spec.missing"); err == nil {
		t.Fatal("Print() error = nil, want a path that matches nothing reported")
	}
}

func TestPrintUnknownFormat(t *testing.T) {
	out := &bytes.Buffer{}

	err := NewPrinter(out).Print(testObject("a", "10.0.0.0/16"), "toml")
	if err == nil {
		t.Fatal("Print() error = nil, want an unknown format refused")
	}
	if !strings.Contains(err.Error(), "toml") {
		t.Errorf("Print() error = %q, want it to name the format", err)
	}
}

func TestPrintList(t *testing.T) {
	out := &bytes.Buffer{}
	list := unstructured.UnstructuredList{testObject("a", "10.0.0.0/16"), testObject("b", "10.1.0.0/16")}

	if err := NewPrinter(out).Print(list, FormatJSON); err != nil {
		t.Fatalf("Print() error = %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "[") {
		t.Errorf("Print() = %q, want a JSON array", out)
	}
}

// A cell of a table stands beside the other rows, which already say what the
// column is about, so nothing is written into one that holds no value.
func TestPrintTableLeavesAColumnWithoutAValueEmpty(t *testing.T) {
	out := &bytes.Buffer{}
	resource := api.APIResource{
		Group: "vpc", Version: "v1alpha2", Resource: "subnets", Kind: "Subnet",
		AdditionalPrinterColumns: []api.AdditionalPrinterColumn{
			{Name: "Cidr", Type: "string", JSONPath: ".spec.cidr"},
			{Name: "RouteTable", Type: "string", JSONPath: ".spec.routeTable"},
		},
	}

	err := NewPrinter(out).PrintTable(resource, []unstructured.Unstructured{
		testObject("a", "10.0.0.0/16"),
	})
	if err != nil {
		t.Fatalf("PrintTable() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("PrintTable() wrote %d lines, want a header and one row\n%s", len(lines), out)
	}
	if strings.Contains(out.String(), "not found in object") {
		t.Errorf("PrintTable() = %q, want no lookup failure written into the table", out)
	}
	if strings.Contains(lines[1], missingValue) {
		t.Errorf("row = %q, want the column without a value left empty", lines[1])
	}
}

func TestPrintTenants(t *testing.T) {
	out := &bytes.Buffer{}

	err := NewPrinter(out).PrintTenants([]api.Tenant{
		{Name: "e2etest", DisplayName: "", Phase: "Ready", Role: "owner"},
	})
	if err != nil {
		t.Fatalf("PrintTenants() error = %v", err)
	}

	if !strings.Contains(out.String(), "e2etest") || !strings.Contains(out.String(), "owner") {
		t.Errorf("PrintTenants() = %q, want the tenant and the role", out)
	}
	if strings.Contains(out.String(), missingValue) {
		t.Errorf("PrintTenants() = %q, want the empty display name left empty", out)
	}
}

func TestPrintMembers(t *testing.T) {
	out := &bytes.Buffer{}

	err := NewPrinter(out).PrintMembers([]api.Member{
		{User: "u-1234", Roles: []string{"editor", "viewer"}},
	})
	if err != nil {
		t.Fatalf("PrintMembers() error = %v", err)
	}

	if !strings.Contains(out.String(), "editor,viewer") {
		t.Errorf("PrintMembers() = %q, want the roles joined", out)
	}
}

func TestPrintDependencies(t *testing.T) {
	out := &bytes.Buffer{}

	err := NewPrinter(out).PrintDependencies([]api.Dependency{
		{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "clidbg-subnet"},
	})
	if err != nil {
		t.Fatalf("PrintDependencies() error = %v", err)
	}

	for _, want := range []string{"vpc/v1alpha2", "Subnet", "clidbg-subnet"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("PrintDependencies() = %q, want it to hold %s", out, want)
		}
	}
}
