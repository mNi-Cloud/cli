package unstructured

import (
	"encoding/json"

	"github.com/goccy/go-yaml"
)

type UnstructuredList []Unstructured

// GetValueByJSONPath reads the one value a JSONPath points at.
func (u UnstructuredList) GetValueByJSONPath(path string) (string, error) {
	return lookup([]Unstructured(u), path)
}

func (u UnstructuredList) EncodeJSON() (string, error) {
	encoded, err := json.Marshal(u)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (u UnstructuredList) EncodeYAML() (string, error) {
	encoded, err := yaml.Marshal(u)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
