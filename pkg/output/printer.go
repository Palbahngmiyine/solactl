package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Printer handles output formatting.
type Printer struct {
	Writer   io.Writer
	JSONMode bool
}

// New creates a Printer writing to stdout.
func New(jsonMode bool) *Printer {
	return &Printer{Writer: os.Stdout, JSONMode: jsonMode}
}

// PrintJSON outputs raw JSON data (pretty-printed).
func (p *Printer) PrintJSON(data json.RawMessage) error {
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, data, "", "  "); err != nil {
		// Fallback: print raw
		_, err = fmt.Fprintln(p.Writer, string(data))
		return err
	}
	_, err := fmt.Fprintln(p.Writer, formatted.String())
	return err
}

// PrintResult outputs data in JSON or text mode.
// In JSON mode, data is pretty-printed JSON.
// In text mode, v is formatted using its String() method or default formatting.
func (p *Printer) PrintResult(data json.RawMessage, v any) error {
	if p.JSONMode {
		return p.PrintJSON(data)
	}
	_, err := fmt.Fprintln(p.Writer, v)
	return err
}

// PrintError outputs an error message to stderr.
func PrintError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

// PrintKeyValue outputs key-value pairs in aligned format.
func (p *Printer) PrintKeyValue(pairs ...string) {
	if len(pairs)%2 != 0 {
		return
	}
	maxKeyLen := 0
	for i := 0; i < len(pairs); i += 2 {
		if len(pairs[i]) > maxKeyLen {
			maxKeyLen = len(pairs[i])
		}
	}
	for i := 0; i < len(pairs); i += 2 {
		_, _ = fmt.Fprintf(p.Writer, "%-*s  %s\n", maxKeyLen, pairs[i], pairs[i+1])
	}
}

// FormatTable outputs data in a simple table format.
func (p *Printer) FormatTable(headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Header
	parts := make([]string, len(headers))
	for i, h := range headers {
		parts[i] = fmt.Sprintf("%-*s", widths[i], h)
	}
	_, _ = fmt.Fprintln(p.Writer, strings.Join(parts, "  "))

	// Separator
	sepParts := make([]string, len(headers))
	for i := range headers {
		sepParts[i] = strings.Repeat("-", widths[i])
	}
	_, _ = fmt.Fprintln(p.Writer, strings.Join(sepParts, "  "))

	// Rows
	for _, row := range rows {
		rowParts := make([]string, len(headers))
		for i := range headers {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			rowParts[i] = fmt.Sprintf("%-*s", widths[i], cell)
		}
		_, _ = fmt.Fprintln(p.Writer, strings.Join(rowParts, "  "))
	}
}
