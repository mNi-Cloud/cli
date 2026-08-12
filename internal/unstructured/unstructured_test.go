package unstructured

import (
	"encoding/json"
	"strings"
	"testing"
)

// gatewayVPC is the answer api-gateway gives for a Vpc, copied off the wire.
const gatewayVPC = `{
  "apiVersion": "vpc/v1alpha2",
  "kind": "Vpc",
  "metadata": {
    "description": "",
    "displayName": "",
    "generation": 2,
    "labels": {},
    "name": "clidbg-vpc",
    "namespace": "21f40d54-9bc9-4e47-9fe2-91908e6f564d",
    "resourceVersion": "1940085",
    "shadow": false,
    "uid": "9299a7f7-4e94-433b-a17a-1e551065744f"
  },
  "spec": {},
  "status": {
    "backingVpc": "a20d44ebie08o2gra8oszobc5p",
    "conditions": [
      {
        "lastTransitionTime": "2026-08-08T08:06:13Z",
        "message": "Resource is available",
        "reason": "OK",
        "status": "True",
        "type": "Available"
      }
    ],
    "observedGeneration": 2,
    "phase": "Ready"
  }
}`

func decodeVPC(t *testing.T) Unstructured {
	t.Helper()

	var object Unstructured
	if err := json.Unmarshal([]byte(gatewayVPC), &object); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return object
}

func TestDecodeKeepsWholeNumbersWhole(t *testing.T) {
	object := decodeVPC(t)

	metadata, ok := object["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata = %T, want an object", object["metadata"])
	}
	generation, ok := metadata["generation"].(int64)
	if !ok {
		t.Fatalf("metadata.generation = %T (%v), want an int64", metadata["generation"], metadata["generation"])
	}
	if generation != 2 {
		t.Errorf("metadata.generation = %d, want 2", generation)
	}
}

func TestDecodeKeepsFractionsAsFloats(t *testing.T) {
	var object Unstructured
	if err := json.Unmarshal([]byte(`{"spec":{"ratio":2.5}}`), &object); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	spec := object["spec"].(map[string]any)
	ratio, ok := spec["ratio"].(float64)
	if !ok {
		t.Fatalf("spec.ratio = %T, want a float64", spec["ratio"])
	}
	if ratio != 2.5 {
		t.Errorf("spec.ratio = %v, want 2.5", ratio)
	}
}

func TestDecodeKeepsLargeIntegersExact(t *testing.T) {
	var object Unstructured
	if err := json.Unmarshal([]byte(`{"status":{"bytes":9007199254740993}}`), &object); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	status := object["status"].(map[string]any)
	if got, ok := status["bytes"].(int64); !ok || got != 9007199254740993 {
		t.Errorf("status.bytes = %v (%T), want the exact integer", status["bytes"], status["bytes"])
	}
}

func TestEncodeYAMLWritesWholeNumbersWithoutADecimalPoint(t *testing.T) {
	encoded, err := decodeVPC(t).EncodeYAML()
	if err != nil {
		t.Fatalf("EncodeYAML() error = %v", err)
	}

	if !strings.Contains(encoded, "generation: 2\n") {
		t.Errorf("EncodeYAML() = %q, want `generation: 2`", encoded)
	}
	if !strings.Contains(encoded, "observedGeneration: 2\n") {
		t.Errorf("EncodeYAML() = %q, want `observedGeneration: 2`", encoded)
	}
	if strings.Contains(encoded, "2.0") {
		t.Errorf("EncodeYAML() = %q, want no number turned into a float", encoded)
	}
	if strings.Contains(encoded, `generation: "2"`) {
		t.Errorf("EncodeYAML() = %q, want no number turned into a string", encoded)
	}
}

func TestEncodeJSONWritesWholeNumbersWithoutADecimalPoint(t *testing.T) {
	encoded, err := decodeVPC(t).EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON() error = %v", err)
	}

	if !strings.Contains(encoded, `"generation":2`) {
		t.Errorf("EncodeJSON() = %q, want `\"generation\":2`", encoded)
	}
}

func TestGetValueByJSONPathReadsAWholeNumberAsAnInteger(t *testing.T) {
	value, err := decodeVPC(t).GetValueByJSONPath(".metadata.generation")
	if err != nil {
		t.Fatalf("GetValueByJSONPath() error = %v", err)
	}
	if value != "2" {
		t.Errorf("GetValueByJSONPath() = %q, want %q", value, "2")
	}
}

func TestGetValueByJSONPathReadsAString(t *testing.T) {
	value, err := decodeVPC(t).GetValueByJSONPath(".status.phase")
	if err != nil {
		t.Fatalf("GetValueByJSONPath() error = %v", err)
	}
	if value != "Ready" {
		t.Errorf("GetValueByJSONPath() = %q, want %q", value, "Ready")
	}
}

func TestGetValueByJSONPathReportsAPathThatMatchesNothing(t *testing.T) {
	value, err := decodeVPC(t).GetValueByJSONPath(".spec.missing")
	if err == nil {
		t.Fatalf("GetValueByJSONPath() error = nil, want the missing path reported (got %q)", value)
	}
	if value != "" {
		t.Errorf("GetValueByJSONPath() = %q, want no value beside the error", value)
	}
}

func TestListDecodeKeepsWholeNumbersWhole(t *testing.T) {
	var list UnstructuredList
	if err := json.Unmarshal([]byte("["+gatewayVPC+"]"), &list); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}

	encoded, err := list.EncodeYAML()
	if err != nil {
		t.Fatalf("EncodeYAML() error = %v", err)
	}
	if strings.Contains(encoded, "2.0") {
		t.Errorf("EncodeYAML() = %q, want no number turned into a float", encoded)
	}
}

func TestListGetValueByJSONPathReportsAPathThatMatchesNothing(t *testing.T) {
	var list UnstructuredList
	if err := json.Unmarshal([]byte("["+gatewayVPC+"]"), &list); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if _, err := list.GetValueByJSONPath("[0].spec.missing"); err == nil {
		t.Fatal("GetValueByJSONPath() error = nil, want the missing path reported")
	}
}

func TestListGetValueByJSONPathReadsAValue(t *testing.T) {
	var list UnstructuredList
	if err := json.Unmarshal([]byte("["+gatewayVPC+"]"), &list); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	value, err := list.GetValueByJSONPath("[0].metadata.name")
	if err != nil {
		t.Fatalf("GetValueByJSONPath() error = %v", err)
	}
	if value != "clidbg-vpc" {
		t.Errorf("GetValueByJSONPath() = %q, want %q", value, "clidbg-vpc")
	}
}

func TestFromTurnsATypedValueIntoAnObject(t *testing.T) {
	type tenant struct {
		Name  string `json:"name"`
		Phase string `json:"phase"`
	}

	object, err := From(tenant{Name: "e2etest", Phase: "Ready"})
	if err != nil {
		t.Fatalf("From() error = %v", err)
	}
	if object.Name() != "" {
		t.Errorf("Name() = %q, want the metadata of a resource to stay empty", object.Name())
	}

	value, err := object.GetValueByJSONPath(".phase")
	if err != nil {
		t.Fatalf("GetValueByJSONPath() error = %v", err)
	}
	if value != "Ready" {
		t.Errorf("GetValueByJSONPath() = %q, want %q", value, "Ready")
	}
}

func TestListFromTurnsATypedSliceIntoAList(t *testing.T) {
	type tenant struct {
		Name string `json:"name"`
	}

	list, err := ListFrom([]tenant{{Name: "e2etest"}, {Name: "clitest"}})
	if err != nil {
		t.Fatalf("ListFrom() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}

	encoded, err := list.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON() error = %v", err)
	}
	if !strings.Contains(encoded, `"name":"e2etest"`) {
		t.Errorf("EncodeJSON() = %q, want the tenants", encoded)
	}
}
