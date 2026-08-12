package api

import (
	"fmt"
	"slices"
	"strings"
)

// SplitAPIVersion splits the group/version form a manifest and an answer name a
// resource with.
func SplitAPIVersion(apiVersion string) (string, string, error) {
	group, version, found := strings.Cut(apiVersion, "/")
	if !found || group == "" || version == "" {
		return "", "", fmt.Errorf("apiVersion %q is not in the group/version form", apiVersion)
	}
	return group, version, nil
}

// Scope tells whether objects of a resource belong to a tenant.
type Scope string

const (
	ScopeNamespaced Scope = "Namespaced"
	ScopeCluster    Scope = "Cluster"
)

// APIResource is one entry of the api-gateway resource catalog.
type APIResource struct {
	Group                    string                    `json:"group"`
	Version                  string                    `json:"version"`
	Resource                 string                    `json:"resource"`
	Kind                     string                    `json:"kind"`
	Scope                    Scope                     `json:"scope"`
	Aliases                  []string                  `json:"aliases"`
	AdditionalPrinterColumns []AdditionalPrinterColumn `json:"additionalPrinterColumns,omitempty"`
	SpecSchema               *Schema                   `json:"specSchema,omitempty"`
	StatusSchema             *Schema                   `json:"statusSchema,omitempty"`
}

// APIVersion names the group and the version of a resource the way a manifest
// writes them.
func (r APIResource) APIVersion() string {
	return r.Group + "/" + r.Version
}

// AdditionalPrinterColumn is one extra column of the table output.
type AdditionalPrinterColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	JSONPath string `json:"jsonPath"`
}

// Namespaced reports whether the resource is addressed under a tenant.
func (r APIResource) Namespaced() bool {
	return r.Scope == ScopeNamespaced
}

// Matches reports whether a name typed on the command line names this resource.
func (r APIResource) Matches(name string) bool {
	if name == "" {
		return false
	}
	return r.Resource == name || slices.Contains(r.Aliases, name)
}

// APIResourceList is the resource catalog the server serves.
type APIResourceList []APIResource

// FindByName looks a resource up by its plural name or one of its aliases.
func (l APIResourceList) FindByName(name string) (APIResource, bool) {
	for _, resource := range l {
		if resource.Matches(name) {
			return resource, true
		}
	}
	return APIResource{}, false
}

// FindByKind looks a resource up by the group, version and kind a manifest names.
func (l APIResourceList) FindByKind(group, version, kind string) (APIResource, bool) {
	for _, resource := range l {
		if resource.Group == group && resource.Version == version && resource.Kind == kind {
			return resource, true
		}
	}
	return APIResource{}, false
}
