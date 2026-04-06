package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestPrintJSON_PrettyPrint(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Writer: &buf, JSONMode: true}

	data := json.RawMessage(`{"id":"123","name":"test"}`)
	if err := p.PrintJSON(data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "\"id\": \"123\"") {
		t.Errorf("expected pretty-printed JSON, got: %s", output)
	}
}

func TestPrintJSON_InvalidJSON(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Writer: &buf, JSONMode: true}

	data := json.RawMessage(`not json`)
	if err := p.PrintJSON(data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back to raw output
	if !strings.Contains(buf.String(), "not json") {
		t.Errorf("expected raw fallback, got: %s", buf.String())
	}
}

func TestPrintResult_JSONMode(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Writer: &buf, JSONMode: true}

	data := json.RawMessage(`{"key":"value"}`)
	if err := p.PrintResult(data, "text output"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "\"key\"") {
		t.Errorf("expected JSON output, got: %s", buf.String())
	}
}

func TestPrintResult_TextMode(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Writer: &buf, JSONMode: false}

	data := json.RawMessage(`{"key":"value"}`)
	if err := p.PrintResult(data, "text output"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "text output" {
		t.Errorf("expected text output, got: %q", buf.String())
	}
}

func TestFormatTable(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Writer: &buf}

	headers := []string{"ID", "STATUS", "TITLE"}
	rows := [][]string{
		{"MSG001", "SENDING", "테스트 메시지"},
		{"MSG002", "COMPLETE", "완료된 메시지"},
	}
	p.FormatTable(headers, rows)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 4 { // header + separator + 2 rows
		t.Errorf("expected 4 lines, got %d: %s", len(lines), output)
	}
	if !strings.Contains(lines[0], "ID") {
		t.Errorf("header missing ID: %s", lines[0])
	}
	if !strings.Contains(lines[1], "---") {
		t.Errorf("separator missing: %s", lines[1])
	}
}

func TestFormatTable_EmptyRows(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Writer: &buf}

	p.FormatTable([]string{"A", "B"}, nil)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 { // header + separator
		t.Errorf("expected 2 lines for empty table, got %d", len(lines))
	}
}

func TestPrintKeyValue(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Writer: &buf}

	p.PrintKeyValue("ID", "MSG001", "Title", "테스트")
	output := buf.String()
	if !strings.Contains(output, "MSG001") {
		t.Errorf("missing value in output: %s", output)
	}
}

func TestNew_CreatesWithStdout(t *testing.T) {
	p := New(true)
	p2 := New(false)

	if p.Writer != os.Stdout {
		t.Error("New(true).Writer should be os.Stdout")
	}
	if p.JSONMode != true {
		t.Error("New(true).JSONMode should be true")
	}
	if p2.JSONMode != false {
		t.Error("New(false).JSONMode should be false")
	}
}

func TestPrintError_WritesToStderr(t *testing.T) {
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	PrintError("test %s", "msg")
	_ = w.Close()
	out, _ := io.ReadAll(r)

	if !strings.Contains(string(out), "Error: test msg") {
		t.Errorf("expected stderr to contain 'Error: test msg', got: %q", string(out))
	}
}

func TestPrintKeyValue_OddPairsReturnsEarly(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Writer: &buf}

	p.PrintKeyValue("key1", "val1", "orphan")
	if buf.Len() != 0 {
		t.Errorf("expected no output for odd pairs, got: %q", buf.String())
	}
}

func TestFormatTable_MismatchedRowLength(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Writer: &buf}

	headers := []string{"A", "B", "C"}
	rows := [][]string{{"x"}}
	p.FormatTable(headers, rows)

	output := buf.String()
	if !strings.Contains(output, "x") {
		t.Errorf("expected output to contain 'x', got: %q", output)
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("write failed") }

func TestPrintJSON_WriteError(t *testing.T) {
	p := &Printer{Writer: errWriter{}, JSONMode: true}

	// Use invalid JSON to trigger the fallback path (fmt.Fprintln to errWriter)
	data := json.RawMessage(`not valid json`)
	err := p.PrintJSON(data)
	if err == nil {
		t.Fatal("expected error from errWriter, got nil")
	}
}

func TestFormatTable_Concurrent(t *testing.T) {
	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			var buf bytes.Buffer
			p := &Printer{Writer: &buf}
			headers := []string{"ID", "NAME"}
			rows := [][]string{
				{fmt.Sprintf("id-%d", n), fmt.Sprintf("name-%d", n)},
			}
			p.FormatTable(headers, rows)

			out := buf.String()
			if !strings.Contains(out, fmt.Sprintf("id-%d", n)) {
				t.Errorf("goroutine %d: expected id-%d in output, got: %q", n, n, out)
			}
		}(i)
	}

	wg.Wait()
}

func TestFormatTable_ZeroColumns(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Writer: &buf}

	p.FormatTable([]string{}, [][]string{})

	output := buf.String()
	// With zero headers, the output should be two newlines (empty header line + empty separator line)
	// and no row lines. The content is effectively empty aside from newlines from fmt.Fprintln.
	if strings.TrimSpace(output) != "" {
		t.Errorf("expected empty trimmed output for zero columns, got: %q", output)
	}
}

func TestFormatTable_RowsShorterThanHeaders(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Writer: &buf}

	headers := []string{"A", "B", "C"}
	rows := [][]string{{"1"}}
	p.FormatTable(headers, rows)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 { // header + separator + 1 row
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), output)
	}
	// The row line should contain "1" and the missing cells should be padded
	if !strings.Contains(lines[2], "1") {
		t.Errorf("expected row to contain '1', got: %q", lines[2])
	}
	// Verify the header has all three columns
	if !strings.Contains(lines[0], "A") || !strings.Contains(lines[0], "B") || !strings.Contains(lines[0], "C") {
		t.Errorf("expected header to contain A, B, C, got: %q", lines[0])
	}
}

func TestPrintKeyValue_ZeroPairs(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Writer: &buf}

	p.PrintKeyValue()

	if buf.Len() != 0 {
		t.Errorf("expected empty output for zero pairs, got: %q", buf.String())
	}
}

func TestPrintJSON_NilMessage(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Writer: &buf, JSONMode: true}

	data := json.RawMessage(nil)
	// json.Indent on nil data returns an error, so it should fall back to raw output
	err := p.PrintJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Fallback prints string(nil RawMessage) which is empty, plus a newline
	// Just verify no panic occurred and something was written
}

func FuzzFormatTable(f *testing.F) {
	f.Add("H1", "H2", "R1C1", "R1C2")
	f.Add("", "", "", "")
	f.Add("A", "B", "x", "y")

	f.Fuzz(func(t *testing.T, h1, h2, r1c1, r1c2 string) {
		var buf bytes.Buffer
		p := &Printer{Writer: &buf}
		headers := []string{h1, h2}
		rows := [][]string{{r1c1, r1c2}}
		// Must not panic
		p.FormatTable(headers, rows)
	})
}
