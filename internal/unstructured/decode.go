package unstructured

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oliveagle/jsonpath"
)

// UnmarshalJSON reads one object of an api-gateway answer.
//
// It decodes numbers with UseNumber and turns them back into Go numbers by
// hand, because the encoding/json default reads every number as a float64 and
// a YAML encoder then writes `generation: 1` back as `generation: 1.0`.
func (u *Unstructured) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var decoded map[string]any
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	if err := normalizeMap(decoded); err != nil {
		return err
	}

	*u = decoded
	return nil
}

// From turns a typed value into an object so that it can be written in the
// same output formats as a resource.
func From(value any) (Unstructured, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var object Unstructured
	if err := json.Unmarshal(encoded, &object); err != nil {
		return nil, err
	}
	return object, nil
}

// Into reads a value taken out of an object into a typed value.
func Into(value any, target any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}

// ListFrom turns a typed slice into a list of objects.
func ListFrom(value any) (UnstructuredList, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var list UnstructuredList
	if err := json.Unmarshal(encoded, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func normalizeMap(object map[string]any) error {
	for key, value := range object {
		normalized, err := normalizeNumbers(value)
		if err != nil {
			return err
		}
		object[key] = normalized
	}
	return nil
}

func normalizeNumbers(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, normalizeMap(typed)

	case []any:
		for index, item := range typed {
			normalized, err := normalizeNumbers(item)
			if err != nil {
				return nil, err
			}
			typed[index] = normalized
		}
		return typed, nil

	case json.Number:
		return parseNumber(typed)

	default:
		return value, nil
	}
}

// parseNumber keeps a whole number whole. A number that fits no Go number is
// refused rather than rounded, so that no value is quietly changed.
func parseNumber(value json.Number) (any, error) {
	if whole, err := value.Int64(); err == nil {
		return whole, nil
	}

	fraction, err := value.Float64()
	if err != nil {
		return nil, fmt.Errorf("cannot read the number %s of the answer: %w", value, err)
	}
	return fraction, nil
}

// lookup reads the one value a JSONPath points at. A path that matches nothing
// is an error: only the caller knows whether that is worth reporting.
func lookup(document any, path string) (string, error) {
	query := path
	if !strings.HasPrefix(query, "$") {
		query = "$" + query
	}

	result, err := jsonpath.JsonPathLookup(document, query)
	if err != nil {
		return "", fmt.Errorf("no value at JSONPath %q: %w", path, err)
	}

	// A path read over a list answers with the values it collected. Collecting
	// nothing is how a path that names nothing looks there, because the library
	// only reports a missing key when it reads a single object.
	if collected, ok := result.([]any); ok && len(collected) == 0 {
		return "", fmt.Errorf("no value at JSONPath %q", path)
	}

	if text, ok := result.(string); ok {
		return text, nil
	}
	return fmt.Sprint(result), nil
}
