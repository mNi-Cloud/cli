package cli

import (
	"strings"
	"testing"
)

func TestExplainWritesTheFieldsOfASpec(t *testing.T) {
	env := loggedIn(t)

	out, err := env.run(t, "explain", "subnets")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	for _, want := range []string{"Subnet", "vpc/v1alpha2", "spec <object>", "cidr <string>", "-required-"} {
		if !strings.Contains(out, want) {
			t.Errorf("explain = %q, want it to hold %q", out, want)
		}
	}
}

func TestExplainDigsIntoAField(t *testing.T) {
	env := loggedIn(t)

	out, err := env.run(t, "explain", "subnets.vpc")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !strings.Contains(out, "spec.vpc <string>") {
		t.Errorf("explain = %q, want the path and the type of the field", out)
	}
}

func TestExplainDigsThroughAList(t *testing.T) {
	env := loggedIn(t)

	out, err := env.run(t, "explain", "vpcs.peerings.target")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !strings.Contains(out, "spec.peerings.target <string>") {
		t.Errorf("explain = %q, want a field of the objects the list holds", out)
	}
}

func TestExplainReadsTheStatusSchema(t *testing.T) {
	env := loggedIn(t)

	out, err := env.run(t, "explain", "vpcs", "--status")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !strings.Contains(out, "status <object>") || !strings.Contains(out, "phase <string>") {
		t.Errorf("explain --status = %q, want the status of the resource", out)
	}
}

func TestExplainRecursiveWritesTheWholeTree(t *testing.T) {
	env := loggedIn(t)

	out, err := env.run(t, "explain", "vpcs", "--recursive")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !strings.Contains(out, "    target <string>") {
		t.Errorf("explain --recursive = %q, want the fields under the list as well", out)
	}
}

func TestExplainOfAFieldThatIsNotThere(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "explain", "subnets.nowhere")
	if err == nil {
		t.Fatal("run() error = nil, want a field that is not there refused")
	}
	if !strings.Contains(err.Error(), "nowhere") {
		t.Errorf("run() error = %q, want it to name the field", err)
	}
}

func TestExplainOfAnUnknownResource(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "explain", "nosuch")
	if err == nil {
		t.Fatal("run() error = nil, want an unknown resource refused")
	}
	if !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("run() error = %q, want it to name the resource", err)
	}
}

func TestExplainNeedsAResource(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "explain"); err == nil {
		t.Error("run() error = nil, want a resource asked for")
	}
}

// The catalog carries the schemas and api-gateway publishes it without a token,
// so a manifest can be written before logging in.
func TestExplainRunsWithoutASession(t *testing.T) {
	env := newTestEnv(t)
	env.writeContext(t, "e2etest")

	out, err := env.run(t, "explain", "subnets")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(out, "cidr <string>") {
		t.Errorf("explain = %q, want the fields of the resource", out)
	}
}

func TestExplainOfAResourceWithoutASchema(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "explain", "tenants")
	if err == nil {
		t.Fatal("run() error = nil, want a resource without a schema reported")
	}
	if !strings.Contains(err.Error(), "tenants") {
		t.Errorf("run() error = %q, want it to name the resource", err)
	}
}
