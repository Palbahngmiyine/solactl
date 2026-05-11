package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/client"
	"github.com/solapi/solactl/pkg/clock"
	"github.com/solapi/solactl/pkg/exporter"
	"github.com/solapi/solactl/pkg/output"
	"github.com/solapi/solactl/pkg/progress"
)

// 상한과 기본값. statistics/daily 엔드포인트는 messages보다 보수적으로 설정.
const (
	statisticsExportPageSizeMax     = 100
	statisticsExportPageSizeDefault = 30
	statisticsExportThrottleDefault = 500 * time.Millisecond
)

var (
	statsExportFlagOutput      string
	statsExportFlagFormat      string
	statsExportFlagThrottle    time.Duration
	statsExportFlagPageSize    int
	statsExportFlagMaxPages    int
	statsExportFlagAppend      bool
	statsExportFlagBOM         bool
	statsExportFlagProgress    string
	statsExportFlagNoProgress  bool
	statsExportFlagResumeToken string

	statsExportFlagStartDate string
	statsExportFlagEndDate   string
	statsExportFlagPrepaid   string
	statsExportFlagOffset    int
)

var statisticsExportDailyCmd = &cobra.Command{
	Use:   "export-daily",
	Short: "일별 발송 통계를 CSV/JSON/JSONL로 export합니다",
	Long: `일별 발송 통계를 파일로 export합니다.

7일을 초과하는 기간은 자동으로 1일 단위 윈도우로 분할되어 streaming됩니다.
페이지 사이 --throttle 만큼 sleep을 두어 통계 서버 부하를 줄입니다.
6개월(180일) 이전 데이터는 자동 삭제되므로 조회할 수 없습니다.

CSV는 count.* 키의 합집합을 헤더로 사용합니다(union-header).
모든 윈도우/페이지의 record를 수집한 뒤 단일 헤더로 일괄 출력합니다.
Ctrl+C로 중단하면 stderr에 재개용 --resume-token이 출력됩니다.`,
	RunE: runStatisticsExportDaily,
}

func init() {
	f := statisticsExportDailyCmd.Flags()
	f.StringVar(&statsExportFlagOutput, "output", "", "출력 파일 경로 (필수, '-'는 stdout)")
	f.StringVar(&statsExportFlagFormat, "format", "csv", "출력 포맷 (csv|json|jsonl)")
	f.DurationVar(&statsExportFlagThrottle, "throttle", statisticsExportThrottleDefault, "페이지/윈도우 호출 사이 sleep (최소 100ms)")
	f.IntVar(&statsExportFlagPageSize, "page-size", statisticsExportPageSizeDefault, fmt.Sprintf("페이지당 건수 (최대 %d)", statisticsExportPageSizeMax))
	f.IntVar(&statsExportFlagMaxPages, "max-pages", 0, "전체 페이지 상한 (0=무제한)")
	f.BoolVar(&statsExportFlagAppend, "append", false, "기존 파일에 이어쓰기 (헤더 검증)")
	f.BoolVar(&statsExportFlagBOM, "bom", false, "UTF-8 BOM 추가 (Windows Excel 한글)")
	f.StringVar(&statsExportFlagProgress, "progress", "auto", "진행률 표시 모드 (auto|on|off)")
	f.BoolVar(&statsExportFlagNoProgress, "no-progress", false, "--progress=off와 동일")
	f.StringVar(&statsExportFlagResumeToken, "resume-token", "", "중단된 export 재개 토큰")
	f.StringVar(&statsExportFlagStartDate, "start-date", "", "시작 날짜 (ISO 8601, 6개월 이내, 필수)")
	f.StringVar(&statsExportFlagEndDate, "end-date", "", "종료 날짜 (ISO 8601, 필수)")
	f.StringVar(&statsExportFlagPrepaid, "prepaid", "", "선불/후불 필터 (true|false). 미지정 시 전체")
	f.IntVar(&statsExportFlagOffset, "offset", 0, "단일 윈도우 내 재개용 offset (resume-token 권장)")
	_ = statisticsExportDailyCmd.MarkFlagRequired("output")
	statisticsCmd.AddCommand(statisticsExportDailyCmd)
}

// statisticsCSVFixedHeaders는 모든 응답에서 보장되는 고정 prefix 컬럼.
// count.* 키들은 union-header로 동적 suffix로 추가된다.
var statisticsCSVFixedHeaders = []string{
	"date", "accountId", "prepaid", "balance", "point", "profit", "refundBalance", "refundPoint",
}

// dailyStatRecord는 statistics/daily 응답 한 항목의 부분 디코딩.
type dailyStatRecord struct {
	Date      string         `json:"date"`
	AccountID string         `json:"accountId"`
	Prepaid   *bool          `json:"prepaid,omitempty"`
	Balance   float64        `json:"balance"`
	Point     float64        `json:"point"`
	Profit    float64        `json:"profit"`
	Count     map[string]any `json:"count"`
	Refund    *struct {
		Balance float64 `json:"balance"`
		Point   float64 `json:"point"`
	} `json:"refund,omitempty"`
}

func (r *dailyStatRecord) fixedRow() []string {
	prepaid := ""
	if r.Prepaid != nil {
		if *r.Prepaid {
			prepaid = "true"
		} else {
			prepaid = "false"
		}
	}
	refBal, refPt := "", ""
	if r.Refund != nil {
		refBal = strconv.FormatFloat(r.Refund.Balance, 'f', -1, 64)
		refPt = strconv.FormatFloat(r.Refund.Point, 'f', -1, 64)
	}
	return []string{
		r.Date, r.AccountID, prepaid,
		strconv.FormatFloat(r.Balance, 'f', -1, 64),
		strconv.FormatFloat(r.Point, 'f', -1, 64),
		strconv.FormatFloat(r.Profit, 'f', -1, 64),
		refBal, refPt,
	}
}

func runStatisticsExportDaily(_ *cobra.Command, _ []string) error {
	// 1. progress 모드.
	mode, err := resolveProgressMode(statsExportFlagProgress, statsExportFlagNoProgress)
	if err != nil {
		return err
	}

	// 2. start/end 필수 검증 (messages와 다름 — 기본값 없음).
	if statsExportFlagStartDate == "" {
		return errors.New("--start-date는 필수입니다")
	}
	if statsExportFlagEndDate == "" {
		return errors.New("--end-date는 필수입니다")
	}

	// 3. prepaid 검증.
	if err := validatePrepaidFlag(statsExportFlagPrepaid); err != nil {
		return err
	}

	// 4. format 검증.
	format := statsExportFlagFormat
	if format != "csv" && format != "json" && format != "jsonl" {
		return fmt.Errorf("잘못된 --format 값: %s (csv|json|jsonl)", format)
	}
	if format == "json" && statsExportFlagAppend {
		return errors.New("--format json은 --append와 함께 사용할 수 없습니다 (json array는 append-safe 불가)")
	}

	// 5. resume-token / offset 결정.
	startWindowDate, initialState, err := resolveStatisticsResumeState(statsExportFlagResumeToken, statsExportFlagOffset)
	if err != nil {
		return err
	}

	// 6. 날짜 파싱.
	startDate, err := parseExportDate(statsExportFlagStartDate)
	if err != nil {
		return fmt.Errorf("--start-date 파싱 실패: %w", err)
	}
	endDate, err := parseExportDate(statsExportFlagEndDate)
	if err != nil {
		return fmt.Errorf("--end-date 파싱 실패: %w", err)
	}

	now := time.Now().UTC()

	// 7. page-size / throttle / 날짜 범위 사전 검증.
	if err := exporter.ValidatePageSize("statistics export-daily --page-size", statsExportFlagPageSize, statisticsExportPageSizeMax); err != nil {
		return err
	}
	if err := exporter.ValidateThrottle(statsExportFlagThrottle, exporter.MinThrottle); err != nil {
		return err
	}
	if _, err := exporter.ValidateDateRange(startDate, endDate, now, exporter.DefaultMaxLookbackDays); err != nil {
		return err
	}

	// 8. 클라이언트 준비.
	c, err := newClient()
	if err != nil {
		return err
	}

	// 9. 출력 파일/스트림 오픈. (validation 통과 후에만)
	w, closer, err := openExportOutput(statsExportFlagOutput, statsExportFlagAppend)
	if err != nil {
		return err
	}
	defer func() { _ = closer.Close() }()

	// 10. RowWriter 생성 (format별).
	rw, err := newStatisticsRowWriter(format, w, statsExportFlagOutput, statsExportFlagAppend, statsExportFlagBOM)
	if err != nil {
		return err
	}

	// 11. progress reporter (항상 stderr).
	rep := progress.New(progress.Options{Writer: errOut(), Mode: mode})

	// 12. fetcher 구성.
	fetcher := newStatisticsFetcher(c, statsExportFlagPageSize, statsExportFlagPrepaid)

	// 13. exporter.Run.
	opts := exporter.Options{
		Now:             now,
		StartDate:       startDate,
		EndDate:         endDate,
		PageSize:        statsExportFlagPageSize,
		MaxPages:        statsExportFlagMaxPages,
		Throttle:        statsExportFlagThrottle,
		Fetcher:         fetcher,
		Writer:          rw,
		Reporter:        rep,
		Clock:           clock.Real(),
		StartWindowDate: startWindowDate,
		InitialState:    initialState,
	}
	result, runErr := exporter.Run(ctx(), opts)

	// 14. format별 종결 처리.
	//   - CSV: union-header batch write를 위해 FinalizeWrite 호출 (runErr 여부 무관)
	//   - JSON: array close
	var finalizeErr error
	switch v := rw.(type) {
	case *statisticsCSVRowWriter:
		finalizeErr = v.FinalizeWrite()
	case *output.JSONArrayWriter:
		finalizeErr = v.Close()
	}
	if runErr == nil && finalizeErr != nil {
		runErr = finalizeErr
	}

	// 15. 부분 결과 + 재개 안내.
	if result.ResumeToken != "" {
		_, _ = fmt.Fprintf(errOut(), "\n중단됨. 누적 %d건 처리.\n", result.RecordsWritten)
		_, _ = fmt.Fprintf(errOut(), "재개:\n  solactl statistics export-daily --output %s --append --resume-token %s\n",
			statsExportFlagOutput, result.ResumeToken)
	}
	return runErr
}

// resolveStatisticsResumeState: resume-token 우선, 다음으로 offset (단일 윈도우 내).
func resolveStatisticsResumeState(token string, offset int) (string, exporter.PageState, error) {
	if token != "" {
		tok, err := exporter.DecodeToken(token)
		if err != nil {
			return "", nil, fmt.Errorf("--resume-token 디코드 실패: %w", err)
		}
		return tok.Window, tok.State, nil
	}
	if offset > 0 {
		b, err := json.Marshal(map[string]int{"offset": offset})
		if err != nil {
			return "", nil, err
		}
		return "", b, nil
	}
	return "", nil, nil
}

// validatePrepaidFlag: 빈 문자열 또는 "true"/"false"만 허용.
func validatePrepaidFlag(v string) error {
	switch v {
	case "", "true", "false":
		return nil
	}
	return fmt.Errorf("잘못된 --prepaid 값: %s (true|false 또는 미지정)", v)
}

// newStatisticsFetcher: GET /messages/v4/statistics/daily PageFetcher.
// offset/limit 페이지네이션. len(arr) < pageSize면 윈도우 종료로 간주.
func newStatisticsFetcher(c *client.Client, pageSize int, prepaid string) exporter.PageFetcher {
	return func(ctx context.Context, w exporter.Window, state exporter.PageState) (exporter.Page, error) {
		params := url.Values{}
		params.Set("limit", strconv.Itoa(pageSize))
		params.Set("startDate", w.Start.UTC().Format(time.RFC3339))
		params.Set("endDate", w.End.UTC().Format(time.RFC3339))
		if prepaid != "" {
			params.Set("prepaid", prepaid)
		}
		offset := 0
		if len(state) > 0 {
			var st struct {
				Offset int `json:"offset"`
			}
			if err := json.Unmarshal(state, &st); err == nil {
				offset = st.Offset
			}
		}
		if offset > 0 {
			params.Set("offset", strconv.Itoa(offset))
		}

		raw, err := c.Get(ctx, "messages/v4/statistics/daily", params)
		if err != nil {
			return exporter.Page{}, err
		}

		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			return exporter.Page{}, fmt.Errorf("응답 파싱 실패: %w", err)
		}

		// 종료 신호: 빈 배열 또는 len < pageSize.
		// len == pageSize일 때만 다음 페이지를 가져온다 — 한 번의 추가 빈 호출 회피.
		var next exporter.PageState
		if len(arr) >= pageSize {
			nextOffset := offset + len(arr)
			nb, mErr := json.Marshal(map[string]int{"offset": nextOffset})
			if mErr != nil {
				return exporter.Page{}, mErr
			}
			next = nb
		}
		return exporter.Page{Records: arr, Next: next}, nil
	}
}

// --- RowWriter 구현 ---

// statisticsCSVRowWriter는 union-header 전략으로 모든 record를 누적 후 FinalizeWrite에서
// 단일 헤더와 함께 일괄 출력한다. 스트리밍 모드는 헤더가 미정이라 불가능.
type statisticsCSVRowWriter struct {
	w            io.Writer
	appendMode   bool
	bom          bool
	appendReader io.ReadCloser

	records   []*dailyStatRecord
	countKeys map[string]struct{}
}

func (s *statisticsCSVRowWriter) WriteRecord(rec json.RawMessage) error {
	var r dailyStatRecord
	if err := json.Unmarshal(rec, &r); err != nil {
		return fmt.Errorf("통계 레코드 디코드 실패: %w", err)
	}
	s.records = append(s.records, &r)
	for k := range r.Count {
		s.countKeys[k] = struct{}{}
	}
	return nil
}

// Flush is intentionally a no-op: CSV는 FinalizeWrite에서 일괄 출력.
func (s *statisticsCSVRowWriter) Flush() error { return nil }

// FinalizeWrite는 누적된 record를 union header로 한 번에 write한다.
// runErr 발생 후에도 호출되어 부분 결과를 보존한다.
func (s *statisticsCSVRowWriter) FinalizeWrite() error {
	if s.appendReader != nil {
		defer func() { _ = s.appendReader.Close() }()
	}
	countKeys := make([]string, 0, len(s.countKeys))
	for k := range s.countKeys {
		countKeys = append(countKeys, k)
	}
	sort.Strings(countKeys)
	headers := make([]string, 0, len(statisticsCSVFixedHeaders)+len(countKeys))
	headers = append(headers, statisticsCSVFixedHeaders...)
	for _, k := range countKeys {
		headers = append(headers, "count_"+k)
	}

	cw, err := output.NewCSVWriter(s.w, s.appendReader, output.CSVOptions{
		Headers:   headers,
		AddBOM:    s.bom,
		Append:    s.appendMode,
		StripCtrl: true,
	})
	if err != nil {
		return err
	}

	for _, r := range s.records {
		row := append(r.fixedRow(), countCellValues(r.Count, countKeys)...)
		if err := cw.WriteRow(row); err != nil {
			return err
		}
	}
	return cw.Flush()
}

// countCellValues returns the cell text for each key from r.Count.
// 누락 키는 빈 셀.
func countCellValues(count map[string]any, keys []string) []string {
	cells := make([]string, len(keys))
	for i, k := range keys {
		if v, ok := count[k]; ok {
			cells[i] = formatNumericCell(v)
		}
	}
	return cells
}

// formatNumericCell: count 셀의 다양한 JSON 숫자/문자 표현을 string으로 정규화.
func formatNumericCell(v any) string {
	switch n := v.(type) {
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64)
	case int:
		return strconv.Itoa(n)
	case int64:
		return strconv.FormatInt(n, 10)
	case json.Number:
		return n.String()
	case string:
		return n
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func newStatisticsRowWriter(format string, w io.Writer, path string, appendMode, bom bool) (exporter.RowWriter, error) {
	switch format {
	case "csv":
		rw := &statisticsCSVRowWriter{
			w:          w,
			appendMode: appendMode,
			bom:        bom,
			countKeys:  make(map[string]struct{}),
		}
		if appendMode && path != "-" {
			// Append 모드: 기존 헤더는 FinalizeWrite 시점에 검증한다 — union 결정 후에만 비교 가능.
			r, err := os.Open(path)
			if err != nil {
				return nil, fmt.Errorf("--append: 기존 파일 열기 실패: %w", err)
			}
			rw.appendReader = r
		}
		return rw, nil
	case "jsonl":
		return output.NewJSONLWriter(w), nil
	case "json":
		return output.NewJSONArrayWriter(w), nil
	}
	return nil, fmt.Errorf("지원되지 않는 format: %s", format)
}
