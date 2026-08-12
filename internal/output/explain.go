package output

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mNi-Cloud/cli/internal/api"
)

const (
	// descriptionWidth is how wide the text of an explanation is laid out.
	descriptionWidth = 80
	// headingWidth lines the headings of an explanation up with each other.
	headingWidth = 10
	// requiredMark tells that a manifest has to carry a field.
	requiredMark = " -required-"
)

// Explanation is one schema as `mni explain` shows it.
type Explanation struct {
	Kind       string
	APIVersion string

	// Path names the field inside the resource, beginning at the spec or the
	// status, so that the reader sees where a field sits in a manifest.
	Path string

	Schema *api.Schema

	// Recursive writes the whole tree under a field instead of one step of it.
	Recursive bool
}

// PrintExplanation writes what a manifest may hold under one field.
func (p Printer) PrintExplanation(explanation Explanation) error {
	if explanation.Schema == nil {
		return errors.New("there is no schema to explain " + explanation.Kind + " with")
	}

	if err := p.explainHeader(explanation); err != nil {
		return err
	}
	if err := p.explainDescription(explanation.Schema); err != nil {
		return err
	}
	return p.explainFields(explanation)
}

// explainHeading is one of the lines that stand above an explanation.
type explainHeading struct {
	label string
	value string
}

func (p Printer) explainHeader(explanation Explanation) error {
	schema := explanation.Schema

	headings := []explainHeading{
		{label: "KIND:", value: explanation.Kind},
		{label: "VERSION:", value: explanation.APIVersion},
		{label: "FIELD:", value: explanation.Path + " <" + schema.TypeName() + ">"},
	}
	if allowed := joinValues(schema.Enum); allowed != "" {
		headings = append(headings, explainHeading{label: "ENUM:", value: allowed})
	}
	if schema.Default != nil {
		headings = append(headings, explainHeading{label: "DEFAULT:", value: fmt.Sprint(schema.Default)})
	}

	for _, heading := range headings {
		if _, err := fmt.Fprintf(p.Out, "%-*s%s\n", headingWidth, heading.label, heading.value); err != nil {
			return err
		}
	}
	return nil
}

func (p Printer) explainDescription(schema *api.Schema) error {
	if schema.Description == "" {
		return nil
	}

	if _, err := fmt.Fprint(p.Out, "\nDESCRIPTION:\n"); err != nil {
		return err
	}
	return p.writeWrapped(schema.Description, indentStep+indentStep)
}

func (p Printer) explainFields(explanation Explanation) error {
	schema := explanation.Schema
	fields := schema.Fields()
	if len(fields) == 0 {
		return nil
	}

	if _, err := fmt.Fprint(p.Out, "\nFIELDS:\n"); err != nil {
		return err
	}
	if explanation.Recursive {
		return p.writeFieldTree(schema, indentStep)
	}

	for index, field := range fields {
		if index > 0 {
			if _, err := fmt.Fprintln(p.Out); err != nil {
				return err
			}
		}
		if err := p.writeField(schema, field); err != nil {
			return err
		}
	}
	return nil
}

// writeField writes one field with everything a person needs to fill it in.
func (p Printer) writeField(parent *api.Schema, field api.NamedSchema) error {
	if err := p.writeFieldHeading(parent, field, indentStep); err != nil {
		return err
	}

	body := indentStep + indentStep
	if err := p.writeWrapped(field.Schema.Description, body); err != nil {
		return err
	}
	if allowed := joinValues(field.Schema.Enum); allowed != "" {
		if err := p.writeWrapped("Enum: "+allowed, body); err != nil {
			return err
		}
	}
	if field.Schema.Default == nil {
		return nil
	}
	return p.writeWrapped(fmt.Sprint("Default: ", field.Schema.Default), body)
}

// writeFieldTree writes the shape of a whole subtree. It leaves the
// descriptions out, because a resource carries more text than fits on a screen
// once every field of every step is written.
func (p Printer) writeFieldTree(schema *api.Schema, indent string) error {
	for _, field := range schema.Fields() {
		if err := p.writeFieldHeading(schema, field, indent); err != nil {
			return err
		}
		if err := p.writeFieldTree(field.Schema, indent+indentStep); err != nil {
			return err
		}
	}
	return nil
}

func (p Printer) writeFieldHeading(parent *api.Schema, field api.NamedSchema, indent string) error {
	heading := indent + field.Name + " <" + field.Schema.TypeName() + ">"
	if parent.Requires(field.Name) {
		heading += requiredMark
	}

	_, err := fmt.Fprintln(p.Out, heading)
	return err
}

func (p Printer) writeWrapped(text, indent string) error {
	for _, line := range wrap(text, indent, descriptionWidth) {
		if _, err := fmt.Fprintln(p.Out, line); err != nil {
			return err
		}
	}
	return nil
}

// wrap lays text out at one width. A description comes from a Go source file
// and is wrapped where that file was, so it is folded into one paragraph first
// and laid out again.
func wrap(text, indent string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	lines := []string{}
	line := indent + words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = indent + word
			continue
		}
		line += " " + word
	}
	return append(lines, line)
}

func joinValues(values []any) string {
	if len(values) == 0 {
		return ""
	}

	written := make([]string, 0, len(values))
	for _, value := range values {
		written = append(written, fmt.Sprint(value))
	}
	return strings.Join(written, ", ")
}
