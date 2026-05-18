package output

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"testing"
)

// failingWriter는 모든 Write 호출이 실패하는 writer.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

// limitedFailingWriter는 N바이트 이후부터 실패.
type limitedFailingWriter struct {
	written int
	limit   int
}

func (w *limitedFailingWriter) Write(p []byte) (int, error) {
	remaining := w.limit - w.written
	if remaining <= 0 {
		return 0, errors.New("limit exceeded")
	}
	if len(p) <= remaining {
		w.written += len(p)
		return len(p), nil
	}
	w.written += remaining
	return remaining, errors.New("limit exceeded")
}

func TestCSVWriter_BasicWrite(t *testing.T) {
	tests := []struct {
		name string
		opts CSVOptions
		rows [][]string
		want string
	}{
		{
			name: "헤더와 2개 행",
			opts: CSVOptions{Headers: []string{"id", "name"}},
			rows: [][]string{
				{"1", "alice"},
				{"2", "bob"},
			},
			want: "id,name\n1,alice\n2,bob\n",
		},
		{
			name: "헤더만",
			opts: CSVOptions{Headers: []string{"a", "b", "c"}},
			rows: nil,
			want: "a,b,c\n",
		},
		{
			name: "헤더 없음",
			opts: CSVOptions{},
			rows: [][]string{{"x", "y"}},
			want: "x,y\n",
		},
		{
			name: "빈 cells",
			opts: CSVOptions{Headers: []string{"a", "b", "c"}},
			rows: [][]string{{"", "", ""}},
			want: "a,b,c\n,,\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cw, err := NewCSVWriter(&buf, nil, tt.opts)
			if err != nil {
				t.Fatalf("NewCSVWriter: %v", err)
			}
			for _, r := range tt.rows {
				if err := cw.WriteRow(r); err != nil {
					t.Fatalf("WriteRow: %v", err)
				}
			}
			if err := cw.Flush(); err != nil {
				t.Fatalf("Flush: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCSVWriter_Escape(t *testing.T) {
	tests := []struct {
		name string
		row  []string
		want string
	}{
		{name: "comma in cell", row: []string{"a,b", "c"}, want: "\"a,b\",c\n"},
		{name: "double quote", row: []string{"he said \"hi\"", "x"}, want: "\"he said \"\"hi\"\"\",x\n"},
		{name: "LF in cell", row: []string{"line1\nline2", "x"}, want: "\"line1\nline2\",x\n"},
		{name: "CRLF in cell", row: []string{"a\r\nb", "x"}, want: "\"a\r\nb\",x\n"},
		{name: "CR alone", row: []string{"a\rb", "x"}, want: "\"a\rb\",x\n"},
		// Go encoding/csv quotes leading-space cells (보수적 quoting).
		{name: "leading space quoted", row: []string{" hi", "x"}, want: "\" hi\",x\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cw, err := NewCSVWriter(&buf, nil, CSVOptions{})
			if err != nil {
				t.Fatalf("NewCSVWriter: %v", err)
			}
			if err := cw.WriteRow(tt.row); err != nil {
				t.Fatalf("WriteRow: %v", err)
			}
			if err := cw.Flush(); err != nil {
				t.Fatalf("Flush: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCSVWriter_StripCtrl_On(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCell string
	}{
		{name: "NUL stripped", input: "a\x00b", wantCell: "ab"},
		{name: "0x01 stripped", input: "a\x01b", wantCell: "ab"},
		{name: "0x08 stripped", input: "a\x08b", wantCell: "ab"},
		{name: "tab preserved", input: "a\tb", wantCell: "a\tb"},
		{name: "LF preserved", input: "a\nb", wantCell: "a\nb"},
		{name: "VT stripped", input: "a\x0Bb", wantCell: "ab"},
		{name: "FF stripped", input: "a\x0Cb", wantCell: "ab"},
		{name: "CR preserved", input: "a\rb", wantCell: "a\rb"},
		{name: "0x0E stripped", input: "a\x0Eb", wantCell: "ab"},
		{name: "0x1F stripped", input: "a\x1Fb", wantCell: "ab"},
		{name: "0x20 preserved", input: "a b", wantCell: "a b"},
		{name: "all-stripped", input: "\x00\x01\x02\x03", wantCell: ""},
		{name: "boundary 0x09 0x0A 0x0D preserved", input: "\t\n\r", wantCell: "\t\n\r"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cw, err := NewCSVWriter(&buf, nil, CSVOptions{StripCtrl: true})
			if err != nil {
				t.Fatalf("NewCSVWriter: %v", err)
			}
			// 두 번째 cell "END"는 marker — csv.Reader가 single-empty-cell row를 EOF로 처리하는 동작 회피.
			if err := cw.WriteRow([]string{tt.input, "END"}); err != nil {
				t.Fatalf("WriteRow: %v", err)
			}
			if err := cw.Flush(); err != nil {
				t.Fatalf("Flush: %v", err)
			}
			r := csv.NewReader(strings.NewReader(buf.String()))
			rec, err := r.Read()
			if err != nil {
				t.Fatalf("csv.Read: %v (raw output=%q)", err, buf.String())
			}
			if len(rec) != 2 || rec[0] != tt.wantCell || rec[1] != "END" {
				t.Errorf("got %q, want [%q END]", rec, tt.wantCell)
			}
		})
	}
}

func TestCSVWriter_StripCtrl_Off(t *testing.T) {
	// StripCtrl=false면 모든 control char 보존. encoding/csv가 quote 처리.
	tests := []struct {
		name  string
		input string
	}{
		{name: "NUL preserved", input: "a\x00b"},
		{name: "0x01 preserved", input: "a\x01b"},
		{name: "VT preserved", input: "a\x0Bb"},
		{name: "FF preserved", input: "a\x0Cb"},
		{name: "0x1F preserved", input: "a\x1Fb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cw, err := NewCSVWriter(&buf, nil, CSVOptions{StripCtrl: false})
			if err != nil {
				t.Fatalf("NewCSVWriter: %v", err)
			}
			if err := cw.WriteRow([]string{tt.input}); err != nil {
				t.Fatalf("WriteRow: %v", err)
			}
			if err := cw.Flush(); err != nil {
				t.Fatalf("Flush: %v", err)
			}
			r := csv.NewReader(strings.NewReader(buf.String()))
			r.LazyQuotes = true
			rec, err := r.Read()
			if err != nil {
				t.Fatalf("csv.Read: %v", err)
			}
			if len(rec) != 1 || rec[0] != tt.input {
				t.Errorf("got %q, want %q", rec, []string{tt.input})
			}
		})
	}
}

func TestCSVWriter_BOM(t *testing.T) {
	t.Run("AddBOM true non-append", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := NewCSVWriter(&buf, nil, CSVOptions{Headers: []string{"a"}, AddBOM: true})
		if err != nil {
			t.Fatalf("NewCSVWriter: %v", err)
		}
		out := buf.Bytes()
		if len(out) < 3 || out[0] != 0xEF || out[1] != 0xBB || out[2] != 0xBF {
			t.Fatalf("expected UTF-8 BOM prefix, got: % X", out[:min(3, len(out))])
		}
	})

	t.Run("AddBOM false", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := NewCSVWriter(&buf, nil, CSVOptions{Headers: []string{"a"}, AddBOM: false})
		if err != nil {
			t.Fatalf("NewCSVWriter: %v", err)
		}
		out := buf.Bytes()
		if len(out) >= 3 && out[0] == 0xEF && out[1] == 0xBB && out[2] == 0xBF {
			t.Errorf("expected no BOM, got prefix: % X", out[:3])
		}
	})

	t.Run("AddBOM true append (BOM skipped)", func(t *testing.T) {
		var buf bytes.Buffer
		existing := "a,b\n"
		_, err := NewCSVWriter(&buf, strings.NewReader(existing), CSVOptions{
			Headers: []string{"a", "b"},
			AddBOM:  true,
			Append:  true,
		})
		if err != nil {
			t.Fatalf("NewCSVWriter: %v", err)
		}
		// Append 모드는 BOM/헤더 모두 안 씀.
		if buf.Len() != 0 {
			t.Errorf("expected empty buffer in append mode, got: % X", buf.Bytes())
		}
	})
}

func TestCSVWriter_AppendHeaderMismatch(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		reader   io.Reader // nil로 명시하려면 wantErrMsg에 표시
		headers  []string
		wantErr  bool
	}{
		{
			name:     "헤더 일치",
			existing: "a,b,c\nrow1,row2,row3\n",
			headers:  []string{"a", "b", "c"},
			wantErr:  false,
		},
		{
			name:     "헤더 컬럼 다름",
			existing: "a,b,c\n",
			headers:  []string{"a", "b", "d"},
			wantErr:  true,
		},
		{
			name:     "헤더 대소문자 다름",
			existing: "A,B,C\n",
			headers:  []string{"a", "b", "c"},
			wantErr:  true,
		},
		{
			name:     "헤더 길이 다름",
			existing: "a,b\n",
			headers:  []string{"a", "b", "c"},
			wantErr:  true,
		},
		{
			name:     "빈 파일은 에러",
			existing: "",
			headers:  []string{"a", "b"},
			wantErr:  true,
		},
		{
			name:     "공백 포함 정확히 일치",
			existing: "a ,b\n",
			headers:  []string{"a ", "b"},
			wantErr:  false,
		},
		{
			name:     "공백 차이만 있어도 에러",
			existing: "a,b\n",
			headers:  []string{"a ", "b"},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			var reader io.Reader = strings.NewReader(tt.existing)
			_, err := NewCSVWriter(&buf, reader, CSVOptions{
				Headers: tt.headers,
				Append:  true,
			})
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}

	t.Run("reader nil and Append true", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := NewCSVWriter(&buf, nil, CSVOptions{
			Headers: []string{"a", "b"},
			Append:  true,
		})
		if err == nil {
			t.Error("expected error when reader is nil in Append mode, got nil")
		}
	})
}

func TestCSVWriter_AppendDoesNotRewriteHeader(t *testing.T) {
	var buf bytes.Buffer
	existing := "id,name\n1,alice\n"
	cw, err := NewCSVWriter(&buf, strings.NewReader(existing), CSVOptions{
		Headers: []string{"id", "name"},
		Append:  true,
	})
	if err != nil {
		t.Fatalf("NewCSVWriter: %v", err)
	}
	if err := cw.WriteRow([]string{"2", "bob"}); err != nil {
		t.Fatalf("WriteRow: %v", err)
	}
	if err := cw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	want := "2,bob\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q (헤더 다시 쓰면 안 됨)", got, want)
	}
}

func TestCSVWriter_KoreanAndEmoji(t *testing.T) {
	tests := []struct {
		name string
		row  []string
		want string
	}{
		{name: "한글", row: []string{"안녕하세요", "세상"}, want: "안녕하세요,세상\n"},
		{name: "이모지", row: []string{"🎉", "축하"}, want: "🎉,축하\n"},
		{name: "한글 콤마", row: []string{"안녕,세상", "x"}, want: "\"안녕,세상\",x\n"},
		{name: "혼합", row: []string{"hello 안녕 🌏", "ok"}, want: "hello 안녕 🌏,ok\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cw, err := NewCSVWriter(&buf, nil, CSVOptions{})
			if err != nil {
				t.Fatalf("NewCSVWriter: %v", err)
			}
			if err := cw.WriteRow(tt.row); err != nil {
				t.Fatalf("WriteRow: %v", err)
			}
			if err := cw.Flush(); err != nil {
				t.Fatalf("Flush: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCSVWriter_LargeCell(t *testing.T) {
	const size = 100 * 1024
	big := strings.Repeat("x", size)
	var buf bytes.Buffer
	cw, err := NewCSVWriter(&buf, nil, CSVOptions{})
	if err != nil {
		t.Fatalf("NewCSVWriter: %v", err)
	}
	if err := cw.WriteRow([]string{big, "y"}); err != nil {
		t.Fatalf("WriteRow: %v", err)
	}
	if err := cw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	// 라운드트립으로 무손실 검증.
	r := csv.NewReader(&buf)
	rec, err := r.Read()
	if err != nil {
		t.Fatalf("csv.Read: %v", err)
	}
	if len(rec) != 2 || len(rec[0]) != size || rec[1] != "y" {
		t.Errorf("large cell roundtrip failed: lens=%d/%d", len(rec[0]), len(rec[1]))
	}
}

func TestCSVWriter_LargeCellWithStrip(t *testing.T) {
	// StripCtrl 빠른 경로(needsStrip=false)도 큰 입력에서 동작 확인.
	const size = 50 * 1024
	big := strings.Repeat("x", size) + "\x00" + strings.Repeat("y", size)
	wantLen := size + size
	var buf bytes.Buffer
	cw, err := NewCSVWriter(&buf, nil, CSVOptions{StripCtrl: true})
	if err != nil {
		t.Fatalf("NewCSVWriter: %v", err)
	}
	if err := cw.WriteRow([]string{big}); err != nil {
		t.Fatalf("WriteRow: %v", err)
	}
	if err := cw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	r := csv.NewReader(&buf)
	rec, err := r.Read()
	if err != nil {
		t.Fatalf("csv.Read: %v", err)
	}
	if len(rec[0]) != wantLen {
		t.Errorf("expected length %d after stripping NUL, got %d", wantLen, len(rec[0]))
	}
}

func TestCSVWriter_FlushError(t *testing.T) {
	cw, err := NewCSVWriter(failingWriter{}, nil, CSVOptions{})
	if err != nil {
		t.Fatalf("NewCSVWriter: %v", err)
	}
	// csv.Writer는 내부 버퍼링 — Write 에러는 Flush 후 표면화.
	_ = cw.WriteRow([]string{"hello", "world"})
	if err := cw.Flush(); err == nil {
		t.Error("expected Flush error from failing writer, got nil")
	}
	if err := cw.Error(); err == nil {
		t.Error("expected Error() != nil after failed flush")
	}
}

func TestCSVWriter_BOMWriteError(t *testing.T) {
	_, err := NewCSVWriter(failingWriter{}, nil, CSVOptions{AddBOM: true})
	if err == nil {
		t.Error("expected error when BOM write fails")
	}
}

func TestCSVWriter_HeaderWriteError(t *testing.T) {
	// BOM은 통과, 헤더 쓰기에서 실패 — limit를 3으로 두면 BOM은 OK.
	w := &limitedFailingWriter{limit: 3}
	_, err := NewCSVWriter(w, nil, CSVOptions{
		Headers: []string{"a", "b"},
		AddBOM:  true,
	})
	if err == nil {
		t.Error("expected error when header write fails")
	}
}

func TestCSVWriter_NilWriter(t *testing.T) {
	_, err := NewCSVWriter(nil, nil, CSVOptions{})
	if err == nil {
		t.Error("expected error when writer is nil")
	}
}

func TestCSVWriter_AppendReaderError(t *testing.T) {
	// reader가 항상 에러를 반환.
	_, err := NewCSVWriter(&bytes.Buffer{}, &errReader{}, CSVOptions{
		Headers: []string{"a"},
		Append:  true,
	})
	if err == nil {
		t.Error("expected error when reader fails")
	}
}

func TestCSVWriter_AppendMalformedHeader(t *testing.T) {
	// 잘못된 quote 처리: bare quote in middle은 csv.Reader가 에러.
	bad := "a,\"b\nrow1\n"
	_, err := NewCSVWriter(&bytes.Buffer{}, strings.NewReader(bad), CSVOptions{
		Headers: []string{"a", "b"},
		Append:  true,
	})
	if err == nil {
		t.Error("expected error for malformed CSV header")
	}
}

func TestCSVWriter_WriteRowDoesNotMutateInput(t *testing.T) {
	original := []string{"a\x00b", "c\x01d"}
	rowCopy := []string{original[0], original[1]}

	var buf bytes.Buffer
	cw, err := NewCSVWriter(&buf, nil, CSVOptions{StripCtrl: true})
	if err != nil {
		t.Fatalf("NewCSVWriter: %v", err)
	}
	if err := cw.WriteRow(rowCopy); err != nil {
		t.Fatalf("WriteRow: %v", err)
	}
	if rowCopy[0] != original[0] || rowCopy[1] != original[1] {
		t.Errorf("WriteRow mutated caller slice: %q vs %q", rowCopy, original)
	}
}

type errReader struct{}

func (errReader) Read(p []byte) (int, error) {
	return 0, errors.New("read failed")
}

func FuzzCSVWriter_NoPanic(f *testing.F) {
	f.Add("a", "b", "c")
	f.Add("", "", "")
	f.Add("\x00\x01\x02", "\n\r\t", "안녕,세상")
	f.Add("\"quote\"", "comma,here", "")

	f.Fuzz(func(t *testing.T, a, b, c string) {
		// StripCtrl on/off 둘 다 panic 없이 동작해야 함.
		for _, strip := range []bool{false, true} {
			var buf bytes.Buffer
			cw, err := NewCSVWriter(&buf, nil, CSVOptions{StripCtrl: strip})
			if err != nil {
				t.Fatalf("NewCSVWriter: %v", err)
			}
			if err := cw.WriteRow([]string{a, b, c}); err != nil {
				t.Fatalf("WriteRow: %v", err)
			}
			if err := cw.Flush(); err != nil {
				t.Fatalf("Flush: %v", err)
			}
			// 라운드트립: 결과를 다시 파싱해도 panic 없어야 함.
			r := csv.NewReader(&buf)
			r.LazyQuotes = true
			r.FieldsPerRecord = -1
			for {
				_, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					// LazyQuotes로도 풀 수 없는 입력 (e.g. embedded NUL)은 OK — panic 아니면 통과.
					break
				}
			}
		}
	})
}

// TestCSVWriter_AppendSkipsExistingBOM은 --bom으로 생성한 파일에 --append할 때
// encoding/csv가 UTF-8 BOM(U+FEFF의 UTF-8 인코딩, 0xEF 0xBB 0xBF)을 자동 스킵하지
// 않아 첫 컬럼 헤더 비교가 항상 실패했던 버그의 회귀 가드. verifyAppendHeader는
// bufio.Reader.Peek/Discard로 BOM이 있을 때만 명시적으로 스킵해야 한다.
func TestCSVWriter_AppendSkipsExistingBOM(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		headers  []string
		wantErr  bool
	}{
		{
			name:     "BOM prefix와 정확히 일치하는 헤더",
			existing: "\xEF\xBB\xBFa,b,c\nr1,r2,r3\n",
			headers:  []string{"a", "b", "c"},
			wantErr:  false,
		},
		{
			name:     "BOM 없이 일치하는 헤더 (기존 동작 보존)",
			existing: "a,b,c\n",
			headers:  []string{"a", "b", "c"},
			wantErr:  false,
		},
		{
			name:     "BOM 후 헤더 불일치",
			existing: "\xEF\xBB\xBFa,b,c\n",
			headers:  []string{"a", "b", "d"},
			wantErr:  true,
		},
		{
			name:     "BOM만 존재하고 헤더 없음 — 빈 파일과 동일 취급",
			existing: "\xEF\xBB\xBF",
			headers:  []string{"a"},
			wantErr:  true,
		},
		{
			name:     "잘린 BOM (2바이트만) — BOM 아님으로 취급, CSV 파싱 실패",
			existing: "\xEF\xBB",
			headers:  []string{"a"},
			wantErr:  true,
		},
		{
			name:     "BOM과 한글 헤더 일치",
			existing: "\xEF\xBB\xBF날짜,계정\n",
			headers:  []string{"날짜", "계정"},
			wantErr:  false,
		},
		{
			// Persistence/recovery: 첫 export가 Flush 후 SIGKILL되어 trailing LF 없이
			// 끝났더라도 --append로 안전하게 재개되어야 한다 (csv.Reader는 EOF 직전 row 반환).
			name:     "BOM + trailing LF 없는 헤더만 (잘린 파일 회복)",
			existing: "\xEF\xBB\xBFa,b,c",
			headers:  []string{"a", "b", "c"},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			_, err := NewCSVWriter(&buf, strings.NewReader(tt.existing), CSVOptions{
				Headers: tt.headers,
				Append:  true,
			})
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestCSVWriter_AppendBOMRoundtripWithRealFile은 end-to-end 회귀:
// --bom으로 생성한 CSV에 --append하여 헤더 검증이 통과하는지 검증.
func TestCSVWriter_AppendBOMRoundtripWithRealFile(t *testing.T) {
	var first bytes.Buffer
	cw1, err := NewCSVWriter(&first, nil, CSVOptions{
		Headers: []string{"id", "name"},
		AddBOM:  true,
	})
	if err != nil {
		t.Fatalf("first NewCSVWriter: %v", err)
	}
	if err := cw1.WriteRow([]string{"1", "alice"}); err != nil {
		t.Fatalf("WriteRow: %v", err)
	}
	if err := cw1.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// 두 번째 writer가 첫 번째 출력에 append.
	var second bytes.Buffer
	reader := bytes.NewReader(first.Bytes())
	cw2, err := NewCSVWriter(&second, reader, CSVOptions{
		Headers: []string{"id", "name"},
		Append:  true,
		AddBOM:  true, // append 모드에서는 무시되어야 하지만 호출자가 동일 옵션 전달하는 시나리오.
	})
	if err != nil {
		t.Fatalf("append NewCSVWriter (BOM 스킵 회귀): %v", err)
	}
	if err := cw2.WriteRow([]string{"2", "bob"}); err != nil {
		t.Fatalf("append WriteRow: %v", err)
	}
	if err := cw2.Flush(); err != nil {
		t.Fatalf("append Flush: %v", err)
	}
	// append 모드는 BOM/헤더 모두 안 씀.
	if got := second.String(); got != "2,bob\n" {
		t.Errorf("got %q, want %q", got, "2,bob\n")
	}
}

// TestNeedsStrip_MultiByteSafe는 바이트 인덱싱 + rune 캐스트(이전 구현)가
// 멀티바이트 UTF-8 continuation byte(0x80-0xBF)를 단독 rune으로 잘못 분류하던
// 잠재 버그의 회귀 가드. for-range over string으로 전환하여 한/중/일/이모지가
// false positive로 strip 대상이 되지 않고, 진짜 제어문자만 true를 반환해야 한다.
// invalid UTF-8 시퀀스는 utf8.RuneError(U+FFFD)로 치환되어 보존되는지도 검증.
func TestNeedsStrip_MultiByteSafe(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "ASCII 평문", in: "hello world", want: false},
		{name: "한글", in: "안녕하세요", want: false},
		{name: "중국어", in: "你好世界", want: false},
		{name: "일본어", in: "こんにちは", want: false},
		{name: "이모지", in: "🎉🌏", want: false},
		{name: "한글 + 콤마 + 따옴표", in: "안녕\"세상\",ok", want: false},
		{name: "tab/LF/CR은 보존 (false)", in: "a\tb\nc\rd", want: false},
		{name: "NUL", in: "x\x00y", want: true},
		{name: "한글 + NUL", in: "안녕\x00세상", want: true},
		{name: "ESC (0x1B)", in: "a\x1Bb", want: true},
		{name: "VT (0x0B)", in: "a\x0Bb", want: true},
		{name: "FF (0x0C)", in: "a\x0Cb", want: true},
		{name: "BS (0x08)", in: "a\x08b", want: true},
		{name: "빈 문자열", in: "", want: false},
		{name: "이모지 다음에 NUL", in: "🌏\x00", want: true},
		// invalid UTF-8 byte: for-range는 U+FFFD로 치환하므로 strip 대상 아님 (보존).
		{name: "invalid UTF-8 byte 0xFF", in: "a\xFFb", want: false},
		{name: "lone continuation byte 0xBF", in: "a\xBFb", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsStrip(tt.in); got != tt.want {
				t.Errorf("needsStrip(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestStripControlChars_PreservesMultiByte는 멀티바이트 문자가 제거되지 않고
// 비표시 제어문자만 제거되는지 검증.
func TestStripControlChars_PreservesMultiByte(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "한글 보존", in: "안녕하세요", want: "안녕하세요"},
		{name: "한글 + NUL 제거", in: "안\x00녕", want: "안녕"},
		{name: "이모지 보존", in: "🎉ok", want: "🎉ok"},
		{name: "이모지 + 제어문자", in: "🎉\x01ok", want: "🎉ok"},
		{name: "tab/LF/CR 보존", in: "a\tb\nc\rd", want: "a\tb\nc\rd"},
		{name: "혼합", in: "한\x08글\x1Btest", want: "한글test"},
		// invalid UTF-8 byte는 needsStrip이 false를 반환해 빠른 경로(원본 그대로 반환)
		// 진입 — stripControlChars는 제어문자만 제거하고 사용자 데이터를 임의 변환하지
		// 않는다. (느린 경로에 들어가면 for-range가 U+FFFD로 정규화하므로 두 경로의
		// 결과가 달라지지만, 현재 의도는 invalid byte도 원본 보존.)
		{name: "invalid byte는 원본 그대로 보존 (빠른 경로)", in: "a\xFFb", want: "a\xFFb"},
		// 느린 경로 (제어문자 있음 + invalid byte) — for-range가 U+FFFD로 정규화.
		{name: "느린 경로 진입 시 invalid byte는 U+FFFD로 치환", in: "a\xFFb\x00", want: "a�b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripControlChars(tt.in); got != tt.want {
				t.Errorf("stripControlChars(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func FuzzCSVWriter_StripRoundtrip(f *testing.F) {
	// StripCtrl=true 적용 후 라운드트립으로 동일한 sanitized 값이 나와야 함.
	f.Add("normal text")
	f.Add("with\x00null")
	f.Add("한글 텍스트")
	f.Add("with\nnewline")

	f.Fuzz(func(t *testing.T, s string) {
		sanitized := stripControlChars(s)
		// csv.Reader는 quoted field 안의 \r\n을 \n으로 정규화 (RFC 4180 §2.6에 따른 표준 동작).
		// 라운드트립 비교 시 동일한 정규화 적용.
		expected := strings.ReplaceAll(sanitized, "\r\n", "\n")

		var buf bytes.Buffer
		cw, err := NewCSVWriter(&buf, nil, CSVOptions{StripCtrl: true})
		if err != nil {
			t.Fatalf("NewCSVWriter: %v", err)
		}
		// marker cell "M"으로 single-empty-cell row의 EOF 해석 회피.
		if err := cw.WriteRow([]string{s, "M"}); err != nil {
			t.Fatalf("WriteRow: %v", err)
		}
		if err := cw.Flush(); err != nil {
			t.Fatalf("Flush: %v", err)
		}
		r := csv.NewReader(&buf)
		r.LazyQuotes = true
		rec, err := r.Read()
		if err != nil {
			t.Fatalf("csv.Read: %v (input=%q sanitized=%q output=%q)", err, s, sanitized, buf.String())
		}
		if len(rec) != 2 || rec[0] != expected || rec[1] != "M" {
			t.Errorf("roundtrip mismatch: got %q, want [%q M] (orig=%q sanitized=%q)", rec, expected, s, sanitized)
		}
	})
}

