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
	Singular                 string                    `json:"singular,omitempty"`
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

// FullName names a resource together with its group, the form that names one
// resource of the catalog and no other.
func (r APIResource) FullName() string {
	return r.Resource + "." + r.Group
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

// nameAxis reads the names of one kind that a resource answers to.
type nameAxis func(APIResource) []string

// nameAxes orders the kinds of name a request is resolved against. A match on
// an earlier axis hides every match on a later one, so that a short name added
// to some CRD later cannot take over a name another resource already answers to.
var nameAxes = []nameAxis{pluralName, singularName, kindName, aliasNames}

// alternateNameAxes are the axes besides the plural name, which is the name the
// catalog lists a resource under.
var alternateNameAxes = nameAxes[1:]

func pluralName(r APIResource) []string { return served(r.Resource) }

func singularName(r APIResource) []string { return served(r.Singular) }

func kindName(r APIResource) []string { return served(strings.ToLower(r.Kind)) }

func aliasNames(r APIResource) []string { return served(r.Aliases...) }

// served drops the names the catalog left out, so that a field the server does
// not fill answers to nothing.
func served(names ...string) []string {
	kept := make([]string, 0, len(names))
	for _, name := range names {
		if name != "" {
			kept = append(kept, name)
		}
	}
	return kept
}

// AlternateNames lists the names a user may type for this resource besides its
// plural name, in the order they are resolved in.
func (r APIResource) AlternateNames() []string {
	names := []string{}
	for _, axis := range alternateNameAxes {
		for _, name := range axis(r) {
			if name == r.Resource || slices.Contains(names, name) {
				continue
			}
			names = append(names, name)
		}
	}
	return names
}

// ResourceRequest is a resource name as the user types it on the command line.
type ResourceRequest struct {
	Name  string
	Group string
}

// ParseResourceRequest reads the <name> and <name>.<group> forms a user may
// type. Everything behind the first dot names the group, so that a group with a
// dot in it stays one group.
func ParseResourceRequest(arg string) ResourceRequest {
	name, group, _ := strings.Cut(strings.ToLower(arg), ".")
	return ResourceRequest{Name: name, Group: group}
}

// String writes a request back the way a user types it.
func (r ResourceRequest) String() string {
	if r.Group == "" {
		return r.Name
	}
	return r.Name + "." + r.Group
}

// NoResourceMatchError reports a request that no resource of the catalog answers
// to.
type NoResourceMatchError struct {
	Request ResourceRequest
}

func (e *NoResourceMatchError) Error() string {
	return fmt.Sprintf("this server serves no resource named %q: run `mni api-resources` to see what it serves", e.Request)
}

// AmbiguousResourceError reports a request that more than one resource answers
// to. Only the group tells them apart.
type AmbiguousResourceError struct {
	Request ResourceRequest
	Matches []APIResource
}

func (e *AmbiguousResourceError) Error() string {
	names := make([]string, 0, len(e.Matches))
	for _, match := range e.Matches {
		names = append(names, match.FullName())
	}
	return fmt.Sprintf("resource %q is ambiguous: %s (give the group too, like %q)",
		e.Request, strings.Join(names, ", "), names[0])
}

// APIResourceList is the resource catalog the server serves.
type APIResourceList []APIResource

// FindByName looks a resource up by a name typed on the command line.
func (l APIResourceList) FindByName(name string) (APIResource, error) {
	return l.Find(ParseResourceRequest(name))
}

// Find looks a resource up by a request. The axes are tried in order, and a
// request that names more than one resource on the same axis is refused instead
// of answered with whichever one the catalog lists first.
func (l APIResourceList) Find(request ResourceRequest) (APIResource, error) {
	candidates := l.inGroup(request.Group)

	for _, axis := range nameAxes {
		matched := candidates.answering(axis, request.Name)
		if len(matched) == 0 {
			continue
		}
		if distinct := matched.distinct(); len(distinct) > 1 {
			return APIResource{}, &AmbiguousResourceError{Request: request, Matches: distinct}
		}
		return matched[0], nil
	}
	return APIResource{}, &NoResourceMatchError{Request: request}
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

// inGroup narrows the catalog to one group. A request without a group asks the
// whole catalog.
func (l APIResourceList) inGroup(group string) APIResourceList {
	if group == "" {
		return l
	}

	kept := APIResourceList{}
	for _, resource := range l {
		if resource.Group == group {
			kept = append(kept, resource)
		}
	}
	return kept
}

func (l APIResourceList) answering(axis nameAxis, name string) APIResourceList {
	matched := APIResourceList{}
	for _, resource := range l {
		if slices.Contains(axis(resource), name) {
			matched = append(matched, resource)
		}
	}
	return matched
}

// distinct keeps one entry per group and plural name. The catalog lists a
// resource once for every version the server serves, and those entries are one
// resource, not an ambiguous name.
func (l APIResourceList) distinct() APIResourceList {
	seen := map[string]bool{}
	kept := APIResourceList{}

	for _, resource := range l {
		if seen[resource.FullName()] {
			continue
		}
		seen[resource.FullName()] = true
		kept = append(kept, resource)
	}
	return kept
}
