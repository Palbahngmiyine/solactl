package output

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

// utf8BOM은 Excel 등에서 한글 인코딩 자동 인식을 돕는 마커.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// CSVOptions controls CSV 출력 동작.
type CSVOptions struct {
	Headers   []string
	AddBOM    bool
	Append    bool
	StripCtrl bool
}

// CSVWriter wraps encoding/csv with BOM, control-char stripping, append header validation.
type CSVWriter struct {
	w         *csv.Writer
	stripCtrl bool
}

// NewCSVWriter creates a CSV writer.
// w: target writer.
// reader: 기존 파일을 읽기 위한 reader (Append=true일 때만 필수).
// Append=true이고 reader가 nil이거나 헤더가 일치하지 않으면 error.
func NewCSVWriter(w io.Writer, reader io.Reader, opts CSVOptions) (*CSVWriter, error) {
	if w == nil {
		return nil, errors.New("CSVWriter: writer가 nil")
	}

	if opts.Append {
		if reader == nil {
			return nil, errors.New("CSVWriter: Append 모드에서 reader가 nil (호출자는 빈 파일을 일반 모드로 분기해야 함)")
		}
		if err := verifyAppendHeader(reader, opts.Headers); err != nil {
			return nil, err
		}
		// Append 모드: 헤더 다시 쓰지 않음, BOM도 쓰지 않음 (기존 파일에 이미 있다고 가정).
		cw := &CSVWriter{w: csv.NewWriter(w), stripCtrl: opts.StripCtrl}
		return cw, nil
	}

	if opts.AddBOM {
		if _, err := w.Write(utf8BOM); err != nil {
			return nil, fmt.Errorf("CSVWriter: BOM 쓰기 실패: %w", err)
		}
	}

	cw := &CSVWriter{w: csv.NewWriter(w), stripCtrl: opts.StripCtrl}

	if len(opts.Headers) > 0 {
		if err := cw.WriteRow(opts.Headers); err != nil {
			return nil, fmt.Errorf("CSVWriter: 헤더 쓰기 실패: %w", err)
		}
		// csv.Writer가 내부 버퍼링하므로 underlying writer 에러는 Flush 후에야 표면화.
		// 헤더 쓰기 실패는 생성 시점에 알려야 호출자가 빠르게 분기 가능.
		if err := cw.Flush(); err != nil {
			return nil, fmt.Errorf("CSVWriter: 헤더 flush 실패: %w", err)
		}
	}
	return cw, nil
}

// WriteRow writes a single row, applying StripCtrl when enabled.
func (cw *CSVWriter) WriteRow(row []string) error {
	if cw.stripCtrl {
		// 새 슬라이스로 복사: 호출자의 row를 변경하지 않음.
		cleaned := make([]string, len(row))
		for i, cell := range row {
			cleaned[i] = stripControlChars(cell)
		}
		row = cleaned
	}
	return cw.w.Write(row)
}

// Flush flushes the underlying csv.Writer and returns the first error encountered.
func (cw *CSVWriter) Flush() error {
	cw.w.Flush()
	return cw.w.Error()
}

// Error returns the last error from the underlying csv.Writer.
func (cw *CSVWriter) Error() error {
	return cw.w.Error()
}

// verifyAppendHeader는 reader의 첫 줄을 CSV로 파싱하여 headers와 strict 비교.
// 기존 파일이 UTF-8 BOM으로 시작하면(예: --bom으로 생성된 파일) 비교 전에 BOM을
// 건너뛴다. encoding/csv는 BOM을 자동 처리하지 않으므로 그대로 두면 첫 컬럼 헤더에
// BOM이 prefix되어 항상 불일치한다 (Go 공식 문서 권장 패턴).
func verifyAppendHeader(reader io.Reader, headers []string) error {
	br := bufio.NewReader(reader)

	// BOM 스킵: 존재할 때만 Discard. Peek/Discard는 buffer 내 무손실.
	if b, err := br.Peek(len(utf8BOM)); err == nil && bytes.Equal(b, utf8BOM) {
		if _, derr := br.Discard(len(utf8BOM)); derr != nil {
			return fmt.Errorf("CSV BOM 스킵 실패: %w", derr)
		}
	}

	// 빈 파일(또는 BOM만 존재)도 명시적으로 거부: 호출자가 별도 분기하도록 시그널링.
	if _, err := br.Peek(1); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("CSV 헤더 불일치: 기존 파일이 비어 있음 (Append 대신 일반 모드 사용 필요)")
		}
		return fmt.Errorf("CSV 헤더 읽기 실패: %w", err)
	}
	r := csv.NewReader(br)
	r.FieldsPerRecord = -1
	existing, err := r.Read()
	if err != nil {
		return fmt.Errorf("CSV 헤더 파싱 실패: %w", err)
	}
	if len(existing) != len(headers) {
		return fmt.Errorf("CSV 헤더 불일치: 기존=%v, 신규=%v", existing, headers)
	}
	for i := range existing {
		if existing[i] != headers[i] {
			return fmt.Errorf("CSV 헤더 불일치: 기존=%v, 신규=%v", existing, headers)
		}
	}
	return nil
}

// stripControlChars는 NUL과 비표시 제어문자를 제거하되 tab/LF/CR은 보존.
func stripControlChars(s string) string {
	// 빠른 경로: 제거 대상이 없으면 원본 반환 (alloc 회피).
	if !needsStrip(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isStripTarget(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// needsStrip은 string을 rune 단위로 순회하여 제거 대상 코드 포인트를 탐지한다.
// Go 공식 가이드("Strings, bytes, runes and characters") 권장 idiom으로
// 멀티바이트 UTF-8 문자의 continuation 바이트를 잘못 검사하지 않는다.
func needsStrip(s string) bool {
	for _, r := range s {
		if isStripTarget(r) {
			return true
		}
	}
	return false
}

func isStripTarget(r rune) bool {
	// 보존: 0x09 (tab), 0x0A (LF), 0x0D (CR).
	// 제거: 0x00-0x08, 0x0B, 0x0C, 0x0E-0x1F.
	switch {
	case r >= 0x00 && r <= 0x08:
		return true
	case r == 0x0B, r == 0x0C:
		return true
	case r >= 0x0E && r <= 0x1F:
		return true
	}
	return false
}
