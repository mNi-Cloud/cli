package api

import "fmt"

// Dependency is one resource api-gateway names in the dependency graph of
// another. It stands on either side of a link: what a resource needs, and what
// needs a resource. mNi Cloud deletes a resource together with everything that
// depends on it, so the second side is what a delete carries with it.
type Dependency struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
}

// GroupVersion splits the apiVersion a dependency is reported under.
func (d Dependency) GroupVersion() (string, string, error) {
	group, version, err := SplitAPIVersion(d.APIVersion)
	if err != nil {
		return "", "", fmt.Errorf("the server reported %s under an unusable apiVersion: %w", d, err)
	}
	return group, version, nil
}

// String names a dependency the way the user typed its kind.
func (d Dependency) String() string {
	return d.Kind + "/" + d.Name
}
