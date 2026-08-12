package output

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/unstructured"
)

const (
	// indentStep is how far one step into a nested value moves the text.
	indentStep = "  "
	// itemMarker stands in front of the first field of an entry of a list. It is
	// as wide as one step, so that everything under an entry lines up with it.
	itemMarker = "- "
	// valueGap is what stands between the name of a field and its value.
	valueGap = "   "
)

// hiddenStatusFields are the parts of a status describe does not write where
// the server put them: the phase and the conditions have a place of their own,
// and the observed generation belongs to the controller behind a resource
// rather than to the person reading about it.
var hiddenStatusFields = []string{"phase", "conditions", "observedGeneration"}

// Description is one resource as `mni describe` shows it: the object itself,
// together with both sides of its dependency graph.
type Description struct {
	Object       unstructured.Unstructured
	Dependencies []api.Dependency
	Dependents   []api.Dependency
}

// blocks puts one blank line between the blocks that are written, and none in
// front of the ones that are left out.
type blocks struct {
	out     io.Writer
	written bool
}

// next opens the block that is about to be written.
func (b *blocks) next() error {
	if !b.written {
		b.written = true
		return nil
	}

	_, err := fmt.Fprintln(b.out)
	return err
}

// PrintDescription writes one resource for a person to read.
//
// It writes what the user of a cloud has a use for and leaves the bookkeeping
// of the cluster behind api-gateway out, because mNi Cloud does not show
// Kubernetes to its users. A part of a resource the server left empty is left
// out as well, except for the two sides of the dependency graph: having nothing
// depend on a resource is what somebody about to delete it wants to read.
func (p Printer) PrintDescription(description Description) error {
	object := description.Object
	status := object.Section("status")

	conditions, err := conditionsOf(status)
	if err != nil {
		return err
	}

	written := &blocks{out: p.Out}
	if err := p.describeHeader(written, object, status); err != nil {
		return err
	}
	if err := p.describeSection(written, "Spec", object.Section("spec"), nil); err != nil {
		return err
	}
	if err := p.describeSection(written, "Status", status, hiddenStatusFields); err != nil {
		return err
	}
	if err := p.describeConditions(written, conditions); err != nil {
		return err
	}
	if err := p.describeGraph(written, "Depends on", description.Dependencies); err != nil {
		return err
	}
	return p.describeGraph(written, "Needed by", description.Dependents)
}

func (p Printer) describeHeader(written *blocks, object unstructured.Unstructured, status map[string]any) error {
	if err := written.next(); err != nil {
		return err
	}
	metadata := object.Section("metadata")

	table := NewWriter(p.Out)
	table.WriteRow("Name:", object.Name())
	table.WriteRow("Kind:", object.Kind())
	table.WriteRow("API Version:", object.APIVersion())
	table.WriteRow("Phase:", scalar(status["phase"]))
	writeIfSet(table, "Display Name:", text(metadata["displayName"]))
	writeIfSet(table, "Description:", text(metadata["description"]))
	writeIfSet(table, "Labels:", joinLabels(metadata["labels"]))
	if shadow, _ := metadata["shadow"].(bool); shadow {
		table.WriteRow("Shadow:", "true")
	}
	return table.Flush()
}

// describeSection writes a spec or a status as the server sent it, keeping the
// shape of what it holds. A section that holds nothing to write is left out,
// because a resource whose phase stands above it plainly has a status.
func (p Printer) describeSection(written *blocks, name string, fields map[string]any, hidden []string) error {
	group := fieldsOf(indentStep, fields, hidden)
	if len(group) == 0 {
		return nil
	}

	if err := written.next(); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(p.Out, name+":"); err != nil {
		return err
	}
	return p.writeGroup(group)
}

// describeConditions writes what the controller behind a resource made of it.
// A condition carries the moment it last changed, which reads better as the
// time that has passed since than as a date.
func (p Printer) describeConditions(written *blocks, conditions []api.Condition) error {
	if err := written.next(); err != nil {
		return err
	}
	table := NewWriter(p.Out)

	if len(conditions) == 0 {
		table.WriteRow("Conditions:", missingValue)
		return table.Flush()
	}

	table.WriteLine("Conditions:")
	table.WriteHeader(indentStep+"Type", "Status", "Reason", "Age", "Message")
	for _, condition := range conditions {
		table.WriteRow(
			indentStep+condition.Type,
			condition.Status,
			condition.Reason,
			age(condition.LastTransitionTime),
			condition.Message,
		)
	}
	return table.Flush()
}

func (p Printer) describeGraph(written *blocks, name string, dependencies []api.Dependency) error {
	if err := written.next(); err != nil {
		return err
	}
	table := NewWriter(p.Out)

	if len(dependencies) == 0 {
		table.WriteRow(name+":", missingValue)
		return table.Flush()
	}

	table.WriteLine(name + ":")
	table.WriteHeader(indentStep+"Kind", "Name")
	for _, dependency := range dependencies {
		table.WriteRow(indentStep+dependency.Kind, dependency.Name)
	}
	return table.Flush()
}

// writeIfSet writes a line only when the server reported something to write.
// mNi Cloud leaves most of the metadata of a resource empty, so a line for
// every field would be a line of nothing on most screens.
func writeIfSet(table Writer, label, value string) {
	if value == "" {
		return
	}
	table.WriteRow(label, value)
}

// conditionsOf reads the conditions out of a status. A status that holds
// something else than conditions under that name is reported, because the
// conditions are the point of describing a resource.
func conditionsOf(status map[string]any) ([]api.Condition, error) {
	raw, found := status["conditions"]
	if !found {
		return nil, nil
	}

	var conditions []api.Condition
	if err := unstructured.Into(raw, &conditions); err != nil {
		return nil, fmt.Errorf("cannot read the conditions the server reported: %w", err)
	}
	return conditions, nil
}

// describedField is one field of a spec or a status, with the text that stands
// in front of its name.
type describedField struct {
	prefix string
	name   string
	value  any
}

func (f describedField) label() string {
	return f.prefix + f.name + ":"
}

// writeGroup writes fields that stand beside each other. Their values line up
// with each other and with nothing else, so that a step into a nested value
// reads as a step rather than as another column.
func (p Printer) writeGroup(group []describedField) error {
	width := labelWidth(group)

	for _, field := range group {
		switch value := field.value.(type) {
		case map[string]any:
			if len(value) == 0 {
				if err := p.writeValue(field.label(), width, missingValue); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintln(p.Out, field.label()); err != nil {
				return err
			}
			if err := p.writeGroup(fieldsOf(field.prefix+indentStep, value, nil)); err != nil {
				return err
			}

		case []any:
			if len(value) == 0 {
				if err := p.writeValue(field.label(), width, missingValue); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintln(p.Out, field.label()); err != nil {
				return err
			}
			if err := p.writeItems(field.prefix+indentStep, value); err != nil {
				return err
			}

		default:
			if err := p.writeValue(field.label(), width, scalar(field.value)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p Printer) writeItems(indent string, items []any) error {
	for _, item := range items {
		switch value := item.(type) {
		case map[string]any:
			if len(value) == 0 {
				if _, err := fmt.Fprintln(p.Out, indent+itemMarker+missingValue); err != nil {
					return err
				}
				continue
			}
			if err := p.writeGroup(itemFieldsOf(indent, value)); err != nil {
				return err
			}

		case []any:
			if _, err := fmt.Fprintln(p.Out, indent+itemMarker); err != nil {
				return err
			}
			if err := p.writeItems(indent+indentStep, value); err != nil {
				return err
			}

		default:
			if _, err := fmt.Fprintln(p.Out, indent+itemMarker+scalar(item)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p Printer) writeValue(label string, width int, value string) error {
	_, err := fmt.Fprintf(p.Out, "%-*s%s%s\n", width, label, valueGap, value)
	return err
}

func fieldsOf(prefix string, fields map[string]any, hidden []string) []describedField {
	group := make([]describedField, 0, len(fields))
	for _, name := range sortedKeys(fields) {
		if slices.Contains(hidden, name) {
			continue
		}
		group = append(group, describedField{prefix: prefix, name: name, value: fields[name]})
	}
	return group
}

// itemFieldsOf builds the fields of one entry of a list, with the marker of the
// entry in front of the first of them.
func itemFieldsOf(indent string, fields map[string]any) []describedField {
	group := fieldsOf(indent+indentStep, fields, nil)
	if len(group) > 0 {
		group[0].prefix = indent + itemMarker
	}
	return group
}

// labelWidth is how far the values of a group stand from the left. Only a field
// that carries a value on its own line takes part in it.
func labelWidth(group []describedField) int {
	width := 0
	for _, field := range group {
		if isNested(field.value) {
			continue
		}
		if length := len(field.label()); length > width {
			width = length
		}
	}
	return width
}

func isNested(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	default:
		return false
	}
}

func sortedKeys(fields map[string]any) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func joinLabels(value any) string {
	labels, ok := value.(map[string]any)
	if !ok {
		return ""
	}

	pairs := make([]string, 0, len(labels))
	for _, key := range sortedKeys(labels) {
		pairs = append(pairs, key+"="+scalar(labels[key]))
	}
	return strings.Join(pairs, ", ")
}

// scalar writes one value of a resource on a line of its own, where a value the
// server left empty is worth saying out loud.
func scalar(value any) string {
	if value == nil {
		return missingValue
	}
	if written, ok := value.(string); ok {
		return orMissing(written)
	}
	return fmt.Sprint(value)
}

func text(value any) string {
	written, _ := value.(string)
	return written
}

// age is a cell of a table, so a condition that carries no moment leaves it
// empty rather than filling it in.
func age(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return humanDuration(time.Since(at))
}

// humanDuration writes a length of time in the largest unit that still says
// something, the way a person would read it off a screen.
func humanDuration(passed time.Duration) string {
	switch {
	case passed < time.Second:
		return "0s"
	case passed < time.Minute:
		return fmt.Sprintf("%ds", int(passed.Seconds()))
	case passed < time.Hour:
		return fmt.Sprintf("%dm", int(passed.Minutes()))
	case passed < 24*time.Hour:
		return fmt.Sprintf("%dh", int(passed.Hours()))
	case passed < 365*24*time.Hour:
		return fmt.Sprintf("%dd", int(passed.Hours()/24))
	default:
		return fmt.Sprintf("%dy", int(passed.Hours()/24/365))
	}
}
