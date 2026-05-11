package output

import (
	"bufio"
	"encoding/json"
	"io"
)

// JSONLWriter writes each record as a single JSON line.
type JSONLWriter struct{ bw *bufio.Writer }

// NewJSONLWriter wraps w in a buffered JSONL writer.
func NewJSONLWriter(w io.Writer) *JSONLWriter {
	return &JSONLWriter{bw: bufio.NewWriter(w)}
}

func (j *JSONLWriter) WriteRecord(rec json.RawMessage) error {
	if _, err := j.bw.Write(rec); err != nil {
		return err
	}
	return j.bw.WriteByte('\n')
}

func (j *JSONLWriter) Flush() error { return j.bw.Flush() }

// JSONArrayWriter writes records as a single JSON array. Close() must be called to
// emit the closing bracket. The array is opened lazily on first WriteRecord so an
// empty run produces "[]".
type JSONArrayWriter struct {
	bw     *bufio.Writer
	first  bool
	opened bool
}

// NewJSONArrayWriter wraps w in a buffered JSON-array writer.
func NewJSONArrayWriter(w io.Writer) *JSONArrayWriter {
	return &JSONArrayWriter{bw: bufio.NewWriter(w)}
}

func (j *JSONArrayWriter) WriteRecord(rec json.RawMessage) error {
	if !j.opened {
		if _, err := j.bw.WriteString("["); err != nil {
			return err
		}
		j.opened = true
		j.first = true
	}
	if !j.first {
		if _, err := j.bw.WriteString(","); err != nil {
			return err
		}
	}
	if _, err := j.bw.Write(rec); err != nil {
		return err
	}
	j.first = false
	return nil
}

func (j *JSONArrayWriter) Flush() error { return j.bw.Flush() }

// Close emits the closing "]" (or "[]" if no record was written) and flushes.
func (j *JSONArrayWriter) Close() error {
	if !j.opened {
		if _, err := j.bw.WriteString("[]"); err != nil {
			return err
		}
	} else {
		if _, err := j.bw.WriteString("]"); err != nil {
			return err
		}
	}
	return j.bw.Flush()
}
