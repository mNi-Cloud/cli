package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/unstructured"
)

const (
	// FormatTable is the default, human readable format.
	FormatTable = "table"
	// FormatJSON writes the object as the server sent it.
	FormatJSON = "json"
	// FormatYAML writes the object as a manifest.
	FormatYAML = "yaml"
	// JSONPathPrefix starts a format that picks one value out of the object.
	JSONPathPrefix = "jsonpath="
	// missingValue fills a table cell the resource holds no value for.
	missingValue = "<none>"
)

// Encodable is a resource, or a list of them, that can be written out.
type Encodable interface {
	EncodeJSON() (string, error)
	EncodeYAML() (string, error)
	GetValueByJSONPath(path string) (string, error)
}

// Printer writes resources to one output stream.
type Printer struct {
	Out io.Writer
}

// NewPrinter builds a printer over an output stream.
func NewPrinter(out io.Writer) Printer {
	return Printer{Out: out}
}

// Print writes a resource in one of the machine readable formats.
func (p Printer) Print(data Encodable, format string) error {
	switch {
	case format == FormatJSON:
		encoded, err := data.EncodeJSON()
		if err != nil {
			return fmt.Errorf("cannot encode the resource as JSON: %w", err)
		}
		_, err = fmt.Fprintln(p.Out, encoded)
		return err

	case format == FormatYAML:
		encoded, err := data.EncodeYAML()
		if err != nil {
			return fmt.Errorf("cannot encode the resource as YAML: %w", err)
		}
		_, err = fmt.Fprint(p.Out, encoded)
		return err

	case strings.HasPrefix(format, JSONPathPrefix):
		// A path typed on the command line that matches nothing is a mistake of
		// the caller, so it is reported instead of writing an empty line.
		value, err := data.GetValueByJSONPath(strings.TrimPrefix(format, JSONPathPrefix))
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(p.Out, value)
		return err

	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

// PrintTable writes the columns api-gateway publishes for a resource.
func (p Printer) PrintTable(resource api.APIResource, objects []unstructured.Unstructured) error {
	headers := []string{"Name"}
	for _, column := range resource.AdditionalPrinterColumns {
		headers = append(headers, column.Name)
	}

	table := NewWriter(p.Out)
	table.WriteHeader(headers...)
	for _, object := range objects {
		row := []string{object.Name()}
		for _, column := range resource.AdditionalPrinterColumns {
			row = append(row, cell(object, column.JSONPath))
		}
		table.WriteRow(row...)
	}

	return table.Flush()
}

// PrintTenants writes the tenants the caller takes part in.
func (p Printer) PrintTenants(tenants []api.Tenant) error {
	table := NewWriter(p.Out)
	table.WriteHeader("Name", "Display Name", "Phase", "Role")
	for _, tenant := range tenants {
		table.WriteRow(tenant.Name, tenant.DisplayName, tenant.Phase, tenant.Role)
	}
	return table.Flush()
}

// PrintMembers writes the members of one tenant.
func (p Printer) PrintMembers(members []api.Member) error {
	table := NewWriter(p.Out)
	table.WriteHeader("User", "Roles")
	for _, member := range members {
		table.WriteRow(member.User, strings.Join(member.Roles, ","))
	}
	return table.Flush()
}

// PrintDependencies writes one side of the dependency graph of a resource: the
// resources it needs, or the ones that need it.
func (p Printer) PrintDependencies(dependencies []api.Dependency) error {
	table := NewWriter(p.Out)
	table.WriteHeader("API Version", "Kind", "Name")
	for _, dependency := range dependencies {
		table.WriteRow(dependency.APIVersion, dependency.Kind, dependency.Name)
	}
	return table.Flush()
}

// cell reads one column of a row. A resource that holds no value for a column
// is not a failure, so the cell is left empty instead of stopping the table.
// A row of a table stands beside its neighbours, which already say what the
// column is about, so an empty cell needs nothing written into it.
func cell(object unstructured.Unstructured, path string) string {
	value, err := object.GetValueByJSONPath(path)
	if err != nil {
		return ""
	}
	return value
}

// orMissing fills in a value that stands on a line of its own, where an empty
// line would read as a value rather than as the lack of one.
func orMissing(value string) string {
	if value == "" {
		return missingValue
	}
	return value
}
