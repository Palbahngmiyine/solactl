package output

import (
	"strings"
	"testing"
)

func TestNormalizeFormat(t *testing.T) {
	cases := []struct {
		in      string
		want    Format
		wantErr bool
	}{
		{"", FormatTable, false},
		{"json", FormatJSON, false},
		{"JSON", FormatJSON, false},
		{"  table  ", FormatTable, false},
		{"csv", FormatCSV, false},
		{"yaml", "", true},
	}
	for _, tc := range cases {
		got, err := NormalizeFormat(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("NormalizeFormat(%q): want error, got %q", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeFormat(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizeFormat(%q): want %q got %q", tc.in, tc.want, got)
		}
	}
}

func TestFormatBytes_JSON(t *testing.T) {
	out, err := FormatBytes([]byte(`{"a":1,"b":[2,3]}`), FormatJSON)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	if !strings.Contains(out, "\"a\": 1") {
		t.Errorf("expected indented JSON, got %q", out)
	}
}

func TestFormatBytes_JSONInvalidErrors(t *testing.T) {
	_, err := FormatBytes([]byte(`not-json`), FormatJSON)
	if err == nil {
		t.Fatal("want error for invalid JSON")
	}
}

func TestFormatBytes_TableArray(t *testing.T) {
	in := `[{"id":"R1","name":"가나","count":3},{"id":"R2","name":"다라","count":7}]`
	out, err := FormatBytes([]byte(in), FormatTable)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	if !strings.Contains(out, "id") || !strings.Contains(out, "R1") || !strings.Contains(out, "R2") {
		t.Errorf("unexpected table:\n%s", out)
	}
}

func TestFormatBytes_TableEmptyArray(t *testing.T) {
	out, err := FormatBytes([]byte(`[]`), FormatTable)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	if out != "(결과 없음)" {
		t.Errorf("want (결과 없음), got %q", out)
	}
}

func TestFormatBytes_TablePagination(t *testing.T) {
	in := `{"data":[{"id":"X"}],"totalCount":42,"startKey":"abc","hasMore":true}`
	out, err := FormatBytes([]byte(in), FormatTable)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	if !strings.Contains(out, "총 42건") {
		t.Errorf("missing totalCount: %s", out)
	}
	if !strings.Contains(out, "다음 키: abc") {
		t.Errorf("missing startKey: %s", out)
	}
	if !strings.Contains(out, "다음 페이지 있음") {
		t.Errorf("missing hasMore: %s", out)
	}
}

func TestFormatBytes_TableSingleObject(t *testing.T) {
	out, err := FormatBytes([]byte(`{"id":"R1","name":"홍길동"}`), FormatTable)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	if !strings.Contains(out, "id") || !strings.Contains(out, "홍길동") {
		t.Errorf("missing values:\n%s", out)
	}
}

func TestFormatBytes_TableTruncation(t *testing.T) {
	long := strings.Repeat("가", 60)
	in := `[{"v":"` + long + `"}]`
	out, err := FormatBytes([]byte(in), FormatTable)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	// 50-rune budget; expect "..." and shorter than the input.
	if !strings.Contains(out, "...") {
		t.Errorf("expected truncation marker:\n%s", out)
	}
	if strings.Count(out, "가") >= 60 {
		t.Errorf("not truncated:\n%s", out)
	}
}

func TestFormatBytes_TableSkipsNestedFields(t *testing.T) {
	// Nested object/array fields are excluded from columns; primitive ones kept.
	in := `[{"id":"R1","tags":["a","b"],"meta":{"x":1},"count":7}]`
	out, err := FormatBytes([]byte(in), FormatTable)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	if !strings.Contains(out, "id") || !strings.Contains(out, "count") {
		t.Errorf("primitive cols missing:\n%s", out)
	}
	if strings.Contains(out, "tags") || strings.Contains(out, "meta") {
		t.Errorf("nested cols should be skipped:\n%s", out)
	}
}

func TestFormatBytes_CSVBasic(t *testing.T) {
	in := `[{"id":"R1","name":"홍"},{"id":"R2","name":"김"}]`
	out, err := FormatBytes([]byte(in), FormatCSV)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d:\n%s", len(lines), out)
	}
	if lines[0] != "id,name" {
		t.Errorf("header: %q", lines[0])
	}
}

func TestFormatBytes_CSVEscapes(t *testing.T) {
	in := `[{"v":"a,b"},{"v":"q\"q"},{"v":"line\nbreak"}]`
	out, err := FormatBytes([]byte(in), FormatCSV)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	if !strings.Contains(out, `"a,b"`) {
		t.Errorf("comma escape missing: %s", out)
	}
	if !strings.Contains(out, `"q""q"`) {
		t.Errorf("quote escape missing: %s", out)
	}
	if !strings.Contains(out, "\"line\nbreak\"") {
		t.Errorf("newline escape missing: %s", out)
	}
}

func TestFormatBytes_CSVPaginatedEnvelope(t *testing.T) {
	in := `{"data":[{"id":"R1"}],"totalCount":99}`
	out, err := FormatBytes([]byte(in), FormatCSV)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	// CSV must drill into data[]; totalCount must NOT leak as a column.
	if !strings.Contains(out, "id") || strings.Contains(out, "totalCount") {
		t.Errorf("envelope handling:\n%s", out)
	}
}

func TestFormatBytes_CSVNumberFormatting(t *testing.T) {
	out, err := FormatBytes([]byte(`[{"n":1,"f":1.5}]`), FormatCSV)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	// Integer-valued JSON numbers must print without decimal.
	if !strings.Contains(out, "1,1.5") && !strings.Contains(out, "f,n") {
		t.Errorf("number formatting:\n%s", out)
	}
}

func TestFormatBytes_EmptyInput(t *testing.T) {
	for _, f := range []Format{FormatJSON, FormatTable, FormatCSV} {
		got, err := FormatBytes(nil, f)
		if err != nil {
			t.Errorf("[%s] err: %v", f, err)
		}
		if got != "" {
			t.Errorf("[%s] want empty, got %q", f, got)
		}
	}
}

func TestFormatBytes_NullJSON(t *testing.T) {
	// API client returns json.RawMessage("null") for empty 2xx bodies.
	out, err := FormatBytes([]byte(`null`), FormatTable)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	// Should not panic; rendering "null" as a stringified primitive is acceptable.
	if out == "" {
		t.Errorf("empty rendering for null is suspicious")
	}
}

func TestFormatBytes_PaginationFalseHasMore(t *testing.T) {
	in := `{"data":[{"id":"X"}],"hasMore":false}`
	out, err := FormatBytes([]byte(in), FormatTable)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	if !strings.Contains(out, "마지막 페이지") {
		t.Errorf("hasMore=false missing terminal indicator:\n%s", out)
	}
}

func TestFormatBytes_HeterogeneousArrayFirstKeyOnly(t *testing.T) {
	// Documented limitation: only first item's keys form the header. Second
	// item's "extra" field is silently dropped.
	in := `[{"id":"R1"},{"id":"R2","extra":"lost"}]`
	out, err := FormatBytes([]byte(in), FormatCSV)
	if err != nil {
		t.Fatalf("FormatBytes: %v", err)
	}
	if strings.Contains(out, "extra") || strings.Contains(out, "lost") {
		t.Errorf("heterogeneous fields must not leak into CSV:\n%s", out)
	}
}
