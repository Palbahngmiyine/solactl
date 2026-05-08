// Package output formats CRM API responses for terminal display.
//
// Mirrors @solapi/crm-cli sdk/cli/src/output/formatter.ts so that solactl's
// CRM commands accept the same --format json|table|csv contract.
package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Format is one of the three supported output modes. An empty value is
// treated as the default (table) so that callers can pass the user-supplied
// flag without normalising first.
type Format string

const (
	FormatJSON  Format = "json"
	FormatTable Format = "table"
	FormatCSV   Format = "csv"

	// arrayCellMaxRunes is the per-cell rune budget when rendering arrays
	// of objects as a table. Mirrors formatter.ts:58.
	arrayCellMaxRunes = 50
	// objectCellMaxRunes is the per-cell rune budget for single-object
	// rendering. Mirrors formatter.ts:78.
	objectCellMaxRunes = 80
)

// ErrInvalidFormat is returned when a format outside the known set is given.
var ErrInvalidFormat = errors.New("출력 형식은 json, table, csv 중 하나여야 합니다")

// NormalizeFormat returns the canonical format value or ErrInvalidFormat.
// Empty strings default to table.
func NormalizeFormat(s string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(s))) {
	case "":
		return FormatTable, nil
	case FormatJSON:
		return FormatJSON, nil
	case FormatTable:
		return FormatTable, nil
	case FormatCSV:
		return FormatCSV, nil
	}
	return "", ErrInvalidFormat
}

// Format renders raw JSON bytes (the API response) in the requested format.
// `data` may be any valid JSON document; non-JSON input falls back to a
// JSON-decode error so callers can surface a clear message.
func FormatBytes(raw []byte, format Format) (string, error) {
	switch format {
	case FormatJSON, "":
		return prettyJSON(raw)
	case FormatTable:
		return formatTable(raw)
	case FormatCSV:
		return formatCSV(raw)
	}
	return "", ErrInvalidFormat
}

func prettyJSON(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return "", fmt.Errorf("응답 JSON 파싱 실패: %w", err)
	}
	return buf.String(), nil
}

func formatTable(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", fmt.Errorf("응답 JSON 파싱 실패: %w", err)
	}
	switch tv := v.(type) {
	case []any:
		if len(tv) == 0 {
			return "(결과 없음)", nil
		}
		return arrayAsTable(tv), nil
	case map[string]any:
		if list, ok := tv["data"].([]any); ok {
			meta := metaLine(tv)
			body := arrayAsTable(list)
			if meta == "" {
				return body, nil
			}
			return body + "\n" + meta, nil
		}
		return objectAsTable(tv), nil
	}
	return fmt.Sprintf("%v", v), nil
}

// arrayAsTable renders []any (each item assumed to be a map) as an ASCII
// table. Columns are taken from the first item's primitive (non-object)
// fields, mirroring the upstream CLI behaviour.
func arrayAsTable(items []any) string {
	if len(items) == 0 {
		return "(결과 없음)"
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		// Mixed array of primitives: fall back to a single-column "value" table.
		rows := make([][]string, 0, len(items))
		for _, it := range items {
			rows = append(rows, []string{toCell(it, arrayCellMaxRunes)})
		}
		return renderTable([]string{"value"}, rows)
	}

	// Preserve insertion-friendly column ordering: sort keys alphabetically
	// for determinism (Go map iteration order is randomised).
	keys := make([]string, 0, len(first))
	for k, v := range first {
		if isPrimitiveOrNil(v) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	rows := make([][]string, 0, len(items))
	for _, it := range items {
		row := make([]string, len(keys))
		obj, _ := it.(map[string]any)
		for i, k := range keys {
			row[i] = toCell(obj[k], arrayCellMaxRunes)
		}
		rows = append(rows, row)
	}
	return renderTable(keys, rows)
}

// objectAsTable renders a single object as a two-column key→value table.
func objectAsTable(obj map[string]any) string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([][]string, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, []string{k, toCell(obj[k], objectCellMaxRunes)})
	}
	return renderTable([]string{"key", "value"}, rows)
}

// metaLine builds the pagination footer (totalCount/startKey/hasMore).
// Returns "" if no pagination keys were found.
func metaLine(m map[string]any) string {
	parts := make([]string, 0, 3)
	if v, ok := m["totalCount"]; ok {
		parts = append(parts, fmt.Sprintf("총 %v건", v))
	}
	if v, ok := m["startKey"]; ok && v != nil && v != "" {
		parts = append(parts, fmt.Sprintf("다음 키: %v", v))
	}
	if v, ok := m["hasMore"]; ok {
		if b, isBool := v.(bool); isBool {
			if b {
				parts = append(parts, "다음 페이지 있음")
			} else {
				parts = append(parts, "마지막 페이지")
			}
		}
	}
	return strings.Join(parts, " | ")
}

func formatCSV(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", fmt.Errorf("응답 JSON 파싱 실패: %w", err)
	}
	items := coerceItems(v)
	if len(items) == 0 {
		return "", nil
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		// Single-column dump for primitive arrays.
		var b strings.Builder
		b.WriteString("value\n")
		for i, it := range items {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(escapeCSV(stringify(it)))
		}
		return b.String(), nil
	}
	keys := make([]string, 0, len(first))
	for k := range first {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(escapeCSV(k))
	}
	for _, item := range items {
		obj, _ := item.(map[string]any)
		b.WriteByte('\n')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(escapeCSV(stringify(obj[k])))
		}
	}
	return b.String(), nil
}

func coerceItems(v any) []any {
	switch tv := v.(type) {
	case []any:
		return tv
	case map[string]any:
		if list, ok := tv["data"].([]any); ok {
			return list
		}
		return []any{tv}
	default:
		return []any{tv}
	}
}

func escapeCSV(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	switch tv := v.(type) {
	case string:
		return tv
	case bool:
		if tv {
			return "true"
		}
		return "false"
	case float64:
		// JSON numbers decode as float64. Preserve integer printing where
		// the value is an integer.
		if tv == float64(int64(tv)) {
			return fmt.Sprintf("%d", int64(tv))
		}
		return fmt.Sprintf("%g", tv)
	case map[string]any, []any:
		body, err := json.Marshal(tv)
		if err != nil {
			return fmt.Sprintf("%v", tv)
		}
		return string(body)
	default:
		return fmt.Sprintf("%v", tv)
	}
}

// toCell stringifies a value for table display, applying length truncation
// at the rune boundary so multi-byte UTF-8 (Korean) does not get cut.
func toCell(v any, maxRunes int) string {
	if v == nil {
		return ""
	}
	s := stringify(v)
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func isPrimitiveOrNil(v any) bool {
	switch v.(type) {
	case nil, bool, string, float64:
		return true
	}
	return false
}

// renderTable renders headers + rows as a fixed-width ASCII table. The width
// of each column is sized to the widest cell (rune count) so multi-byte
// characters do not visually misalign.
func renderTable(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = displayWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				if w := displayWidth(cell); w > widths[i] {
					widths[i] = w
				}
			}
		}
	}

	var b strings.Builder
	writeRow := func(cells []string) {
		for i, c := range cells {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(c)
			pad := widths[i] - displayWidth(c)
			if pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
		}
		b.WriteByte('\n')
	}
	writeRow(headers)
	sep := make([]string, len(headers))
	for i, w := range widths {
		sep[i] = strings.Repeat("-", w)
	}
	writeRow(sep)
	for _, row := range rows {
		fitted := make([]string, len(headers))
		for i := range headers {
			if i < len(row) {
				fitted[i] = row[i]
			}
		}
		writeRow(fitted)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// displayWidth returns a best-effort visual width using rune count. Wide
// CJK characters render double-wide in most terminals; the upstream CLI
// does not compensate either, so we keep parity.
func displayWidth(s string) int {
	return len([]rune(s))
}
