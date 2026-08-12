package output

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mNi-Cloud/cli/internal/api"
)

// recordedSchema reads a schema a running api-gateway published for a resource.
func recordedSchema(t *testing.T, name string) *api.Schema {
	t.Helper()

	raw, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var schema api.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return &schema
}

func explain(t *testing.T, explanation Explanation) string {
	t.Helper()

	out := &bytes.Buffer{}
	if err := NewPrinter(out).PrintExplanation(explanation); err != nil {
		t.Fatalf("PrintExplanation() error = %v", err)
	}
	return out.String()
}

func subnetSpec(t *testing.T) Explanation {
	t.Helper()

	return Explanation{
		Kind:       "Subnet",
		APIVersion: "vpc/v1alpha2",
		Path:       "spec",
		Schema:     recordedSchema(t, "subnet-spec-schema"),
	}
}

func TestExplainNamesTheResourceAndTheField(t *testing.T) {
	out := explain(t, subnetSpec(t))

	for _, want := range []string{"KIND:", "Subnet", "VERSION:", "vpc/v1alpha2", "FIELD:", "spec <object>"} {
		if !strings.Contains(out, want) {
			t.Errorf("explain = %q, want it to hold %q", out, want)
		}
	}
}

func TestExplainWritesEveryFieldWithItsType(t *testing.T) {
	out := explain(t, subnetSpec(t))

	for _, want := range []string{"cidr <string>", "routeTable <string>", "vpc <string>"} {
		if !strings.Contains(out, want) {
			t.Errorf("explain = %q, want it to hold %q", out, want)
		}
	}
}

func TestExplainMarksTheRequiredFields(t *testing.T) {
	out := explain(t, subnetSpec(t))

	if !strings.Contains(lineHolding(out, "cidr <string>"), "-required-") {
		t.Errorf("explain = %q, want cidr marked as required", out)
	}
	if strings.Contains(lineHolding(out, "routeTable <string>"), "-required-") {
		t.Errorf("explain = %q, want routeTable left unmarked", out)
	}
}

// A description written in a Go source file is wrapped where the source was,
// so it is folded and wrapped again to the width of the output.
func TestExplainRewrapsADescription(t *testing.T) {
	out := explain(t, subnetSpec(t))

	if !strings.Contains(out, "RouteTable selects the routing policy for this Subnet.") {
		t.Errorf("explain = %q, want the description of routeTable", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) > descriptionWidth {
			t.Errorf("line = %q is %d columns wide, want at most %d", line, len(line), descriptionWidth)
		}
	}
}

func TestExplainWritesTheValuesAFieldAllows(t *testing.T) {
	out := explain(t, Explanation{
		Kind:       "Vpc",
		APIVersion: "vpc/v1alpha2",
		Path:       "status",
		Schema:     recordedSchema(t, "vpc-status-schema"),
	})

	enum := lineHolding(out, "Enum:")
	for _, want := range []string{"Pending", "Provisioning", "Ready", "Error", "Deleting"} {
		if !strings.Contains(enum, want) {
			t.Errorf("enum line = %q, want it to hold %q", enum, want)
		}
	}
	if !strings.Contains(lineHolding(out, "Default:"), "Pending") {
		t.Errorf("explain = %q, want the default of the phase", out)
	}
}

func TestExplainNamesAListByWhatItHolds(t *testing.T) {
	out := explain(t, Explanation{
		Kind:       "VirtualMachine",
		APIVersion: "vm/v1alpha1",
		Path:       "spec",
		Schema:     recordedSchema(t, "virtualmachine-spec-schema"),
	})

	if !strings.Contains(out, "additionalDisks <[]object>") {
		t.Errorf("explain = %q, want the list named by what it holds", out)
	}
	if !strings.Contains(out, "cores <integer|string>") {
		t.Errorf("explain = %q, want a field of either type named as such", out)
	}
}

// Without --recursive only one step is written, so that the fields of a
// resource fit on a screen.
func TestExplainStopsAtTheFirstStep(t *testing.T) {
	out := explain(t, Explanation{
		Kind:       "VirtualMachine",
		APIVersion: "vm/v1alpha1",
		Path:       "spec",
		Schema:     recordedSchema(t, "virtualmachine-spec-schema"),
	})

	if strings.Contains(out, "generatedDiskName") {
		t.Errorf("explain = %q, want the fields of rootVolume left alone", out)
	}
}

func TestExplainRecursiveWritesTheWholeTree(t *testing.T) {
	explanation := Explanation{
		Kind:       "VirtualMachine",
		APIVersion: "vm/v1alpha1",
		Path:       "spec",
		Schema:     recordedSchema(t, "virtualmachine-spec-schema"),
		Recursive:  true,
	}

	out := explain(t, explanation)

	for _, want := range []string{
		"  rootVolume <object>",
		"    source <object>",
		"      name <string>",
		"  additionalDisks <[]object>",
		"    volumeSource <object>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("explain --recursive = %q, want a line beginning with %q", out, want)
		}
	}
}

// The tree of a whole resource is long, so --recursive writes the shape alone.
func TestExplainRecursiveLeavesTheDescriptionsOut(t *testing.T) {
	explanation := subnetSpec(t)
	explanation.Recursive = true

	out := explain(t, explanation)

	if strings.Contains(out, "RouteTable selects") {
		t.Errorf("explain --recursive = %q, want the descriptions left out", out)
	}
}

func TestExplainOfALeafWritesNoFields(t *testing.T) {
	status := recordedSchema(t, "vpc-status-schema")
	phase, ok := status.Field("phase")
	if !ok {
		t.Fatal("the recorded status holds no phase")
	}

	out := explain(t, Explanation{
		Kind:       "Vpc",
		APIVersion: "vpc/v1alpha2",
		Path:       "status.phase",
		Schema:     phase,
	})

	if !strings.Contains(out, "status.phase <string>") {
		t.Errorf("explain = %q, want the path and the type of the field", out)
	}
	if strings.Contains(out, "FIELDS:") {
		t.Errorf("explain = %q, want no field list under a field that holds none", out)
	}
	if !strings.Contains(lineHolding(out, "ENUM:"), "Ready") {
		t.Errorf("explain = %q, want the values the field allows", out)
	}
}

func TestExplainWritesTheDescriptionOfTheFieldItself(t *testing.T) {
	spec := recordedSchema(t, "subnet-spec-schema")
	routeTable, ok := spec.Field("routeTable")
	if !ok {
		t.Fatal("the recorded spec holds no routeTable")
	}

	out := explain(t, Explanation{
		Kind:       "Subnet",
		APIVersion: "vpc/v1alpha2",
		Path:       "spec.routeTable",
		Schema:     routeTable,
	})

	if !strings.Contains(out, "DESCRIPTION:") {
		t.Errorf("explain = %q, want the description of the field", out)
	}
}

func TestExplainWithoutASchema(t *testing.T) {
	out := &bytes.Buffer{}

	err := NewPrinter(out).PrintExplanation(Explanation{Kind: "Vpc", APIVersion: "vpc/v1alpha2", Path: "spec"})
	if err == nil {
		t.Fatal("PrintExplanation() error = nil, want a missing schema reported")
	}
}
