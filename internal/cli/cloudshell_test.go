package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestCloudShellSessionCommandsUseTheSessionAPI(t *testing.T) {
	env := loggedIn(t)
	collection := "/cs/v1alpha1/tenants/e2etest/sessions"
	env.raw[collection] = rawResponse{Body: `{"sessions":[{"id":"s1","subnet":"subnet-a","phase":"Running","createdAt":"now"}]}`}
	out, err := env.run(t, "cs", "list")
	if err != nil || !strings.Contains(out, `"id": "s1"`) {
		t.Fatalf("cs list output = %q, error = %v", out, err)
	}

	env.raw[collection] = rawResponse{Status: http.StatusCreated, Body: `{"id":"s2","subnet":"subnet-b","phase":"Pending","createdAt":"now"}`}
	out, err = env.run(t, "cs", "create", "subnet-b")
	if err != nil || !strings.Contains(out, `"id": "s2"`) {
		t.Fatalf("cs create output = %q, error = %v", out, err)
	}
	if !strings.Contains(env.requests[len(env.requests)-1].Body, `"subnet":"subnet-b"`) {
		t.Errorf("create body = %q", env.requests[len(env.requests)-1].Body)
	}

	deletePath := collection + "/s2"
	env.raw[deletePath] = rawResponse{Status: http.StatusNoContent}
	out, err = env.run(t, "cs", "delete", "s2")
	if err != nil || !strings.Contains(out, "cloudshell/s2 deleted") || !env.sent(http.MethodDelete, deletePath) {
		t.Fatalf("cs delete output = %q, error = %v", out, err)
	}
}
