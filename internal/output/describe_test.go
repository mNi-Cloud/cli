package output

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/unstructured"
)

// recordedVPC reads the Vpc a running api-gateway answered with.
func recordedVPC(t *testing.T) unstructured.Unstructured {
	t.Helper()

	raw, err := os.ReadFile("testdata/vpc.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var object unstructured.Unstructured
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return object
}

func describe(t *testing.T, description Description) string {
	t.Helper()

	out := &bytes.Buffer{}
	if err := NewPrinter(out).PrintDescription(description); err != nil {
		t.Fatalf("PrintDescription() error = %v", err)
	}
	return out.String()
}

func TestDescribeWritesWhatIdentifiesTheResource(t *testing.T) {
	out := describe(t, Description{Object: recordedVPC(t)})

	for _, want := range []string{
		"Name:", "clidesc",
		"Kind:", "Vpc",
		"API Version:", "vpc/v1alpha2",
		"Display Name:", "CLI describe demo",
		"Description:", "Used to check the describe output",
		"Labels:", "env=dev",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("describe = %q, want it to hold %q", out, want)
		}
	}
}

// mNi Cloud does not show Kubernetes to its users, so nothing that only makes
// sense inside the cluster reaches the screen.
func TestDescribeKeepsTheClusterOutOfSight(t *testing.T) {
	out := describe(t, Description{Object: recordedVPC(t)})

	for _, hidden := range []string{
		"21f40d54-9bc9-4e47-9fe2-91908e6f564d",
		"3efdd940-063c-46d3-98eb-57d21178e4db",
		"1990726",
		"namespace",
		"resourceVersion",
		"observedGeneration",
	} {
		if strings.Contains(out, hidden) {
			t.Errorf("describe = %q, want %q left out", out, hidden)
		}
	}
}

func TestDescribeWritesThePhaseNearTheTop(t *testing.T) {
	out := describe(t, Description{Object: recordedVPC(t)})

	lines := strings.Split(out, "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, "Phase:") {
			if !strings.Contains(line, "Ready") {
				t.Errorf("phase line = %q, want the phase of the resource", line)
			}
			if index > 5 {
				t.Errorf("the phase stands on line %d, want it near the top", index+1)
			}
			return
		}
	}
	t.Errorf("describe = %q, want a phase", out)
}

func TestDescribeWritesTheConditionsAsATable(t *testing.T) {
	object := recordedVPC(t)
	setConditionTimes(object, time.Now().Add(-48*time.Hour))

	out := describe(t, Description{Object: object})

	header := lineHolding(out, "TYPE")
	for _, want := range []string{"TYPE", "STATUS", "REASON", "AGE", "MESSAGE"} {
		if !strings.Contains(header, want) {
			t.Errorf("conditions header = %q, want a %s column", header, want)
		}
	}

	row := lineHolding(out, "Available")
	for _, want := range []string{"Available", "True", "OK", "2d", "Resource is available"} {
		if !strings.Contains(row, want) {
			t.Errorf("condition row = %q, want it to hold %q", row, want)
		}
	}
}

// An absolute time is harder to read than the time that has passed since, so
// the table carries the age instead.
func TestDescribeWritesNoAbsoluteConditionTime(t *testing.T) {
	out := describe(t, Description{Object: recordedVPC(t)})

	if strings.Contains(out, "2026-08-08T09:38:57Z") {
		t.Errorf("describe = %q, want the age instead of the absolute time", out)
	}
}

func TestDescribeWritesTheSpecAndWhatIsLeftOfTheStatus(t *testing.T) {
	out := describe(t, Description{Object: recordedVPC(t)})

	if !strings.Contains(out, "enforceSecurityGroups") || !strings.Contains(out, "true") {
		t.Errorf("describe = %q, want the spec of the resource", out)
	}
	if !strings.Contains(out, "backingVpc") {
		t.Errorf("describe = %q, want the rest of the status", out)
	}
}

func TestDescribeKeepsTheShapeOfNestedValues(t *testing.T) {
	object := unstructured.Unstructured{
		"apiVersion": "vm/v1alpha1",
		"kind":       "VirtualMachine",
		"metadata":   map[string]any{"name": "runner"},
		"spec": map[string]any{
			"cores": int64(2),
			"rootVolume": map[string]any{
				"size":   "20Gi",
				"source": map[string]any{"name": "ubuntu"},
			},
			"additionalDisks": []any{
				map[string]any{"name": "data", "volumeSource": map[string]any{"name": "disk-1"}},
			},
			"tags": []any{"web", "staging"},
		},
	}

	out := describe(t, Description{Object: object})

	for _, want := range []string{
		"  cores:",
		"  rootVolume:",
		"    size:",
		"    source:",
		"      name:",
		"  additionalDisks:",
		"    - name:",
		"      volumeSource:",
		"        name:",
		"    - web",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("describe = %q, want a line beginning with %q", out, want)
		}
	}
}

func TestDescribeWritesBothSidesOfTheDependencyGraph(t *testing.T) {
	out := describe(t, Description{
		Object:       recordedVPC(t),
		Dependencies: []api.Dependency{{APIVersion: "vpc/v1alpha2", Kind: "Vpc", Name: "shared"}},
		Dependents:   []api.Dependency{{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "clidesc-subnet"}},
	})

	if !strings.Contains(out, "Depends on:") || !strings.Contains(out, "shared") {
		t.Errorf("describe = %q, want what the resource needs", out)
	}
	if !strings.Contains(out, "Needed by:") || !strings.Contains(out, "clidesc-subnet") {
		t.Errorf("describe = %q, want what needs the resource", out)
	}
}

// bareVPC is a resource the server reports nothing about beside its phase: a
// spec that holds nothing, a status whose whole content has a place of its own,
// and none of the metadata a user may fill in.
func bareVPC() unstructured.Unstructured {
	return unstructured.Unstructured{
		"apiVersion": "vpc/v1alpha2",
		"kind":       "Vpc",
		"metadata": map[string]any{
			"name":        "bare",
			"displayName": "",
			"description": "",
			"labels":      map[string]any{},
		},
		"spec": map[string]any{},
		"status": map[string]any{
			"phase":              "Ready",
			"observedGeneration": int64(1),
			"conditions":         []any{},
		},
	}
}

// A resource whose phase stands at the top plainly has a status, so writing
// that it has none would read as a contradiction.
func TestDescribeLeavesAnEmptySectionOut(t *testing.T) {
	out := describe(t, Description{Object: bareVPC()})

	for _, left := range []string{"Spec:", "Status:"} {
		if strings.Contains(out, left) {
			t.Errorf("describe = %q, want %q left out when it holds nothing", out, left)
		}
	}
	if !strings.Contains(lineHolding(out, "Phase:"), "Ready") {
		t.Errorf("describe = %q, want the phase written all the same", out)
	}
}

// Most of the metadata of a resource is left empty on most of them, so a line
// for every field would be a screen of nothing.
func TestDescribeLeavesUnsetMetadataOut(t *testing.T) {
	out := describe(t, Description{Object: bareVPC()})

	for _, left := range []string{"Display Name:", "Description:", "Labels:"} {
		if strings.Contains(out, left) {
			t.Errorf("describe = %q, want %q left out when it is not set", out, left)
		}
	}
}

// Having nothing depend on a resource is what somebody about to delete it wants
// to read, so that side is written even when it is empty.
func TestDescribeSaysWhenNothingDependsOnAResource(t *testing.T) {
	out := describe(t, Description{Object: bareVPC()})

	for _, block := range []string{"Depends on:", "Needed by:", "Conditions:"} {
		if !strings.Contains(lineHolding(out, block), missingValue) {
			t.Errorf("describe = %q, want an empty %s named as such", out, block)
		}
	}
}

// A block that is left out leaves no blank line behind it either.
func TestDescribeRunsNoBlankLinesTogether(t *testing.T) {
	out := describe(t, Description{Object: bareVPC()})

	if strings.Contains(out, "\n\n\n") {
		t.Errorf("describe = %q, want one blank line between the blocks that are written", out)
	}
}

// A cell of a table stands beside the other rows, which already say what the
// column is about, so nothing is written into one that holds no value.
func TestDescribeLeavesAnEmptyConditionCellEmpty(t *testing.T) {
	object := recordedVPC(t)
	status, _ := object["status"].(map[string]any)
	conditions, _ := status["conditions"].([]any)
	first, _ := conditions[0].(map[string]any)
	first["message"] = ""
	first["reason"] = ""

	out := describe(t, Description{
		Object:       object,
		Dependencies: []api.Dependency{{APIVersion: "vpc/v1alpha2", Kind: "Vpc", Name: "shared"}},
		Dependents:   []api.Dependency{{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "clidesc-subnet"}},
	})

	if strings.Contains(out, missingValue) {
		t.Errorf("describe = %q, want the empty cells of the condition left empty", out)
	}
}

func TestDescribeReportsAStatusItCannotRead(t *testing.T) {
	object := recordedVPC(t)
	status, _ := object["status"].(map[string]any)
	status["conditions"] = "not a list of conditions"

	out := &bytes.Buffer{}
	if err := NewPrinter(out).PrintDescription(Description{Object: object}); err == nil {
		t.Fatal("PrintDescription() error = nil, want unreadable conditions reported")
	}
}

func TestHumanDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{duration: 0, want: "0s"},
		{duration: 45 * time.Second, want: "45s"},
		{duration: 5 * time.Minute, want: "5m"},
		{duration: 90 * time.Minute, want: "1h"},
		{duration: 47 * time.Hour, want: "1d"},
		{duration: 48 * time.Hour, want: "2d"},
		{duration: 400 * 24 * time.Hour, want: "1y"},
		{duration: -time.Minute, want: "0s"},
	}
	for _, test := range tests {
		if got := humanDuration(test.duration); got != test.want {
			t.Errorf("humanDuration(%v) = %q, want %q", test.duration, got, test.want)
		}
	}
}

// setConditionTimes moves every condition of an object to one moment, so that
// the age a test reads does not depend on when the answer was recorded.
func setConditionTimes(object unstructured.Unstructured, at time.Time) {
	status, _ := object["status"].(map[string]any)
	conditions, _ := status["conditions"].([]any)
	for _, entry := range conditions {
		condition, _ := entry.(map[string]any)
		condition["lastTransitionTime"] = at.UTC().Format(time.RFC3339)
	}
}

func lineHolding(out, needle string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}
