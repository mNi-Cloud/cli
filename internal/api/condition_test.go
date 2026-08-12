package api

import (
	"encoding/json"
	"testing"
	"time"
)

// recordedConditions is the status a running api-gateway answered a Vpc with.
const recordedConditions = `[
  {
    "lastTransitionTime": "2026-08-08T09:38:57Z",
    "message": "Resource is healthy",
    "reason": "OK",
    "status": "False",
    "type": "Degraded"
  },
  {
    "lastTransitionTime": "2026-08-08T09:38:57Z",
    "message": "Resource is available",
    "reason": "OK",
    "status": "True",
    "type": "Available"
  }
]`

func TestConditionsDecodeFromTheAnswer(t *testing.T) {
	var conditions []Condition
	if err := json.Unmarshal([]byte(recordedConditions), &conditions); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(conditions) != 2 {
		t.Fatalf("read %d conditions, want 2", len(conditions))
	}

	first := conditions[0]
	if first.Type != "Degraded" || first.Status != "False" || first.Reason != "OK" {
		t.Errorf("first condition = %+v, want the Degraded one", first)
	}
	if first.Message != "Resource is healthy" {
		t.Errorf("Message = %q, want the recorded one", first.Message)
	}

	want := time.Date(2026, 8, 8, 9, 38, 57, 0, time.UTC)
	if !first.LastTransitionTime.Equal(want) {
		t.Errorf("LastTransitionTime = %v, want %v", first.LastTransitionTime, want)
	}
}
