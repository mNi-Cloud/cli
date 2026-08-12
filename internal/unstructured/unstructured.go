// Package unstructured holds resources as api-gateway sends them, without a Go
// type per kind.
package unstructured

import (
	"encoding/json"

	"github.com/goccy/go-yaml"
)

type Unstructured map[string]interface{}

func (u Unstructured) Name() string {
	return nestedString(u, "metadata", "name")
}

func (u Unstructured) APIVersion() string {
	return nestedString(u, "apiVersion")
}

func (u Unstructured) Kind() string {
	return nestedString(u, "kind")
}

// Section returns one of the objects a resource is built of, such as its
// metadata, its spec or its status.
func (u Unstructured) Section(name string) map[string]any {
	section, _ := u[name].(map[string]any)
	return section
}

// GetValueByJSONPath reads the one value a JSONPath points at.
func (u Unstructured) GetValueByJSONPath(path string) (string, error) {
	return lookup(map[string]any(u), path)
}

func (u Unstructured) EncodeJSON() (string, error) {
	encoded, err := json.Marshal(u)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (u Unstructured) EncodeYAML() (string, error) {
	encoded, err := yaml.Marshal(u)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func nestedString(obj map[string]any, fields ...string) string {
	if len(fields) == 1 {
		if value, ok := obj[fields[0]].(string); ok {
			return value
		}
		return ""
	} else {
		if value, ok := obj[fields[0]].(map[string]any); ok {
			return nestedString(value, fields[1:]...)
		}
		return ""
	}
}
