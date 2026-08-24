package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func jsonIndent(out io.Writer, raw []byte) error {
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, raw, "", "  "); err != nil {
		return fmt.Errorf("cannot read the server response as JSON: %w", err)
	}
	formatted.WriteByte('\n')
	_, err := out.Write(formatted.Bytes())
	return err
}
