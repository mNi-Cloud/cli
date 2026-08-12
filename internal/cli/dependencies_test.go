package cli

import (
	"strings"
	"testing"

	"github.com/mNi-Cloud/cli/internal/api"
)

func TestDependenciesListsWhatAResourceNeeds(t *testing.T) {
	env := loggedIn(t)
	env.dependencies[vpcPath+"/clidbg-vpc"] = []api.Dependency{
		{APIVersion: "vpc/v1alpha2", Kind: "Vpc", Name: "shared"},
	}

	out, err := env.run(t, "dependencies", "vpcs", "clidbg-vpc")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !env.sent("GET", vpcPath+"/clidbg-vpc/dependencies") {
		t.Errorf("no request reached the dependencies of the resource, the last one went to %q", env.lastPath())
	}
	if !strings.Contains(out, "Vpc") || !strings.Contains(out, "shared") {
		t.Errorf("dependencies = %q, want the kind and the name", out)
	}
}

func TestDependenciesAsJSON(t *testing.T) {
	env := loggedIn(t)
	env.dependencies[vpcPath+"/clidbg-vpc"] = []api.Dependency{
		{APIVersion: "vpc/v1alpha2", Kind: "Vpc", Name: "shared"},
	}

	out, err := env.run(t, "dependencies", "vpcs", "clidbg-vpc", "-o", "json")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(out, `"name":"shared"`) {
		t.Errorf("dependencies -o json = %q, want JSON", out)
	}
}

func TestDependentsFollowsTheChain(t *testing.T) {
	env := loggedIn(t)
	subnetPath := "/vpc/v1alpha2/tenants/e2etest/subnets"
	env.dependents[vpcPath+"/clidbg-vpc"] = []api.Dependency{
		{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "clidbg-subnet"},
	}
	env.dependents[subnetPath+"/clidbg-subnet"] = []api.Dependency{
		{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "deeper"},
	}

	out, err := env.run(t, "dependents", "vpcs", "clidbg-vpc")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !strings.Contains(out, "clidbg-subnet") || !strings.Contains(out, "deeper") {
		t.Errorf("dependents = %q, want everything a delete would carry with it", out)
	}
}

func TestDependentsDirectStopsAtTheFirstStep(t *testing.T) {
	env := loggedIn(t)
	subnetPath := "/vpc/v1alpha2/tenants/e2etest/subnets"
	env.dependents[vpcPath+"/clidbg-vpc"] = []api.Dependency{
		{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "clidbg-subnet"},
	}
	env.dependents[subnetPath+"/clidbg-subnet"] = []api.Dependency{
		{APIVersion: "vpc/v1alpha2", Kind: "Subnet", Name: "deeper"},
	}

	out, err := env.run(t, "dependents", "vpcs", "clidbg-vpc", "--direct")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !strings.Contains(out, "clidbg-subnet") {
		t.Errorf("dependents --direct = %q, want what depends on the resource itself", out)
	}
	if strings.Contains(out, "deeper") {
		t.Errorf("dependents --direct = %q, want the chain left alone", out)
	}
	if env.sent("GET", subnetPath+"/clidbg-subnet/dependents") {
		t.Error("the chain was read although only the direct dependents were asked for")
	}
}

func TestDependenciesNeedsAResourceAndAName(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "dependencies", "vpcs"); err == nil {
		t.Error("run() error = nil, want a name asked for")
	}
	if _, err := env.run(t, "dependents", "vpcs"); err == nil {
		t.Error("run() error = nil, want a name asked for")
	}
}
