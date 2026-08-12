package api

import (
	"fmt"
	"slices"
	"strings"
)

// SchemaSection is the part of a resource a schema describes. api-gateway
// publishes one schema for what a manifest writes and one for what the server
// reports back.
type SchemaSection string

const (
	SpecSchemaSection   SchemaSection = "spec"
	StatusSchemaSection SchemaSection = "status"
)

// Schema is the part of the OpenAPI schema api-gateway publishes that says what
// a field is. It holds what a person writing a manifest needs and leaves the
// rest of the schema alone.
type Schema struct {
	Type        string             `json:"type,omitempty"`
	Description string             `json:"description,omitempty"`
	Default     any                `json:"default,omitempty"`
	Enum        []any              `json:"enum,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Items       *Schema            `json:"items,omitempty"`

	// AnyOf holds the types a field takes when it takes more than one, such as a
	// quantity written either as a number or as a string.
	AnyOf []*Schema `json:"anyOf,omitempty"`
}

// Schema returns the schema of one part of a resource.
func (r APIResource) Schema(section SchemaSection) (*Schema, error) {
	var schema *Schema
	switch section {
	case SpecSchemaSection:
		schema = r.SpecSchema
	case StatusSchemaSection:
		schema = r.StatusSchema
	default:
		return nil, fmt.Errorf("a resource has no %q to explain", section)
	}

	if schema == nil {
		return nil, fmt.Errorf("this server publishes no %s schema for %s", section, r.Resource)
	}
	return schema, nil
}

// Field returns the schema of one field. A list is looked into, so that a field
// of the objects a list holds is named the way a manifest names it.
func (s *Schema) Field(name string) (*Schema, bool) {
	field, ok := s.holder().Properties[name]
	return field, ok
}

// NamedSchema is one field of a schema, under the name it is written with.
type NamedSchema struct {
	Name   string
	Schema *Schema
}

// Fields returns the fields of a schema, in the order they read best.
func (s *Schema) Fields() []NamedSchema {
	holder := s.holder()

	names := make([]string, 0, len(holder.Properties))
	for name := range holder.Properties {
		names = append(names, name)
	}
	slices.Sort(names)

	fields := make([]NamedSchema, 0, len(names))
	for _, name := range names {
		fields = append(fields, NamedSchema{Name: name, Schema: holder.Properties[name]})
	}
	return fields
}

// Requires reports whether a manifest has to carry a field.
func (s *Schema) Requires(name string) bool {
	return slices.Contains(s.holder().Required, name)
}

// Resolve walks a field path from a schema. It answers with an error naming the
// step that leads nowhere, so that the user learns where the path went wrong.
func (s *Schema) Resolve(path []string) (*Schema, error) {
	current := s
	for index, name := range path {
		field, ok := current.Field(name)
		if !ok {
			return nil, fmt.Errorf("no field is named %q", strings.Join(path[:index+1], "."))
		}
		current = field
	}
	return current, nil
}

// TypeName names the type of a field the way a manifest would hold it.
func (s *Schema) TypeName() string {
	if len(s.AnyOf) > 0 {
		return strings.Join(s.anyOfTypes(), "|")
	}
	if s.Items != nil {
		return "[]" + s.Items.TypeName()
	}
	return s.Type
}

func (s *Schema) anyOfTypes() []string {
	names := make([]string, 0, len(s.AnyOf))
	for _, alternative := range s.AnyOf {
		names = append(names, alternative.TypeName())
	}
	return names
}

// holder returns the schema the fields of a value belong to: a list carries
// them on what it holds rather than on itself.
func (s *Schema) holder() *Schema {
	if s.Items != nil {
		return s.Items
	}
	return s
}
