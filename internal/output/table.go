// Package output writes resources to the terminal in the format the user asked
// for.
package output

import (
	"io"
	"strings"
	"text/tabwriter"
)

// Writer lays rows out in aligned columns.
type Writer interface {
	WriteHeader(headers ...string)
	WriteRow(values ...string)
	// WriteLine writes a line that stands on its own, such as the heading of a
	// block. It takes part in no column, so it aligns nothing.
	WriteLine(text string)
	Flush() error
}

type writer struct {
	tabwriter.Writer
}

// NewWriter builds a table writer over an output stream.
func NewWriter(out io.Writer) Writer {
	return &writer{
		Writer: *tabwriter.NewWriter(out, 0, 0, 3, ' ', 0),
	}
}

func (w *writer) WriteHeader(headers ...string) {
	upper := make([]string, 0, len(headers))
	for _, header := range headers {
		upper = append(upper, strings.ToUpper(header))
	}
	w.WriteRow(upper...)
}

// WriteRow writes the cells of one row. The last cell closes the line without
// a tab behind it, so that a row carries no trailing blanks.
func (w *writer) WriteRow(values ...string) {
	w.WriteLine(strings.Join(values, "\t"))
}

func (w *writer) WriteLine(text string) {
	w.Write([]byte(text + "\n"))
}

func (w *writer) Flush() error {
	return w.Writer.Flush()
}
