package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
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

// 상한과 기본값. messages-v4 부하 보호를 위해 보수적으로 설정.
const (
	messagesExportPageSizeMax     = 200
	messagesExportPageSizeDefault = 50
	messagesExportThrottleDefault = 500 * time.Millisecond
)

var (
	msgExportFlagOutput      string
	msgExportFlagFormat      string // csv|json|jsonl
	msgExportFlagThrottle    time.Duration
	msgExportFlagPageSize    int
	msgExportFlagMaxPages    int
	msgExportFlagAppend      bool
	msgExportFlagBOM         bool
	msgExportFlagProgress    string // auto|on|off
	msgExportFlagNoProgress  bool
	msgExportFlagResumeToken string

	// messages 전용 필터.
	msgExportFlagStartDate string
	msgExportFlagEndDate   string
	msgExportFlagType      string
	msgExportFlagStatus    string
	msgExportFlagTo        string
	msgExportFlagFrom      string
	msgExportFlagGroupID   string
	msgExportFlagStartKey  string
)

var messagesExportCmd = &cobra.Command{
	Use:   "export",
	Short: "발송 내역을 CSV/JSON/JSONL로 export합니다",
	Long: `발송 내역을 파일로 export합니다.

7일을 초과하는 기간은 자동으로 1일 단위 윈도우로 분할되어 streaming됩니다.
페이지 사이 --throttle 만큼 sleep을 두어 messages-v4 부하를 줄입니다.
6개월(180일) 이전 데이터는 자동 삭제되므로 조회할 수 없습니다.
Ctrl+C로 중단하면 stderr에 재개용 --resume-token이 출력됩니다.`,
	RunE: runMessagesExport,
}

func init() {
	f := messagesExportCmd.Flags()
	f.StringVar(&msgExportFlagOutput, "output", "", "출력 파일 경로 (필수, '-'는 stdout)")
	f.StringVar(&msgExportFlagFormat, "format", "csv", "출력 포맷 (csv|json|jsonl)")
	f.DurationVar(&msgExportFlagThrottle, "throttle", messagesExportThrottleDefault, "페이지/윈도우 호출 사이 sleep (최소 100ms)")
	f.IntVar(&msgExportFlagPageSize, "page-size", messagesExportPageSizeDefault, fmt.Sprintf("페이지당 건수 (최대 %d)", messagesExportPageSizeMax))
	f.IntVar(&msgExportFlagMaxPages, "max-pages", 0, "전체 페이지 상한 (0=무제한)")
	f.BoolVar(&msgExportFlagAppend, "append", false, "기존 파일에 이어쓰기 (헤더 검증)")
	f.BoolVar(&msgExportFlagBOM, "bom", false, "UTF-8 BOM 추가 (Windows Excel 한글)")
	f.StringVar(&msgExportFlagProgress, "progress", "auto", "진행률 표시 모드 (auto|on|off)")
	f.BoolVar(&msgExportFlagNoProgress, "no-progress", false, "--progress=off와 동일")
	f.StringVar(&msgExportFlagResumeToken, "resume-token", "", "중단된 export 재개 토큰")
	f.StringVar(&msgExportFlagStartDate, "start-date", "", "시작 날짜 (ISO 8601, 6개월 이내)")
	f.StringVar(&msgExportFlagEndDate, "end-date", "", "종료 날짜 (ISO 8601)")
	f.StringVar(&msgExportFlagType, "type", "", "메시지 타입 (SMS, LMS, MMS, ATA, ...)")
	f.StringVar(&msgExportFlagStatus, "status-code", "", "상태 코드")
	f.StringVar(&msgExportFlagTo, "to", "", "수신 번호")
	f.StringVar(&msgExportFlagFrom, "from", "", "발신 번호")
	f.StringVar(&msgExportFlagGroupID, "group-id", "", "그룹 ID")
	f.StringVar(&msgExportFlagStartKey, "start-key", "", "단일 윈도우 내 재개용 (resume-token 권장)")
	_ = messagesExportCmd.MarkFlagRequired("output")
	messagesCmd.AddCommand(messagesExportCmd)
}

// 14개 고정 컬럼. customFields는 JSON 문자열로 한 컬럼에 직렬화.
var messagesCSVHeaders = []string{
	"messageId", "type", "status", "statusCode", "to", "from", "country",
	"subject", "dateCreated", "dateUpdated", "groupId", "accountId", "text", "customFields",
}

// messageRecord는 CSV row 구성을 위한 부분 디코딩. 누락 필드는 빈 문자열.
type messageRecord struct {
	MessageID    string         `json:"messageId"`
	Type         string         `json:"type"`
	Status       string         `json:"status"`
	StatusCode   string         `json:"statusCode"`
	To           string         `json:"to"`
	From         string         `json:"from"`
	Country      string         `json:"country"`
	Subject      string         `json:"subject"`
	DateCreated  string         `json:"dateCreated"`
	DateUpdated  string         `json:"dateUpdated"`
	GroupID      string         `json:"groupId"`
	AccountID    string         `json:"accountId"`
	Text         string         `json:"text"`
	CustomFields map[string]any `json:"customFields,omitempty"`
}

func (m *messageRecord) row() []string {
	customFields := ""
	if len(m.CustomFields) > 0 {
		if b, err := json.Marshal(m.CustomFields); err == nil {
			customFields = string(b)
		}
	}
	return []string{
		m.MessageID, m.Type, m.Status, m.StatusCode, m.To, m.From, m.Country,
		m.Subject, m.DateCreated, m.DateUpdated, m.GroupID, m.AccountID, m.Text, customFields,
	}
}

func runMessagesExport(_ *cobra.Command, _ []string) error {
	// 1. progress 모드.
	mode, err := resolveProgressMode(msgExportFlagProgress, msgExportFlagNoProgress)
	if err != nil {
		return err
	}

	// 2. resume-token / start-key 결정.
	startWindowDate, initialState, err := resolveResumeState(msgExportFlagResumeToken, msgExportFlagStartKey)
	if err != nil {
		return err
	}

	// 3. 날짜 파싱 (기본값 [now-7d, now]).
	now := time.Now().UTC()
	startDate, endDate, err := resolveDateRange(msgExportFlagStartDate, msgExportFlagEndDate, now)
	if err != nil {
		return err
	}

	// 4. format 검증.
	format := msgExportFlagFormat
	if format != "csv" && format != "json" && format != "jsonl" {
		return fmt.Errorf("잘못된 --format 값: %s (csv|json|jsonl)", format)
	}
	if format == "json" && msgExportFlagAppend {
		return errors.New("--format json은 --append와 함께 사용할 수 없습니다 (json array는 append-safe 불가)")
	}

	// 5. page-size / throttle / 날짜 범위 사전 검증.
	//    exporter.Run도 다시 검증하지만, 파일을 열기 전에 거부해야 zero-byte 파일을 남기지 않는다.
	if err := exporter.ValidatePageSize("messages export --page-size", msgExportFlagPageSize, messagesExportPageSizeMax); err != nil {
		return err
	}
	if err := exporter.ValidateThrottle(msgExportFlagThrottle, exporter.MinThrottle); err != nil {
		return err
	}
	if _, err := exporter.ValidateDateRange(startDate, endDate, now, exporter.DefaultMaxLookbackDays); err != nil {
		return err
	}

	// 6. 클라이언트 준비.
	c, err := newClient()
	if err != nil {
		return err
	}

	// 7. 출력 파일/스트림 오픈. (validation 통과 후에만 — 실패 시 빈 파일 남기지 않음)
	w, closer, err := openExportOutput(msgExportFlagOutput, msgExportFlagAppend)
	if err != nil {
		return err
	}
	defer func() { _ = closer.Close() }()

	// 8. RowWriter 생성 (format별).
	rw, err := newMessagesRowWriter(format, w, msgExportFlagOutput, msgExportFlagAppend, msgExportFlagBOM)
	if err != nil {
		return err
	}

	// 9. progress reporter (항상 stderr).
	rep := progress.New(progress.Options{Writer: errOut(), Mode: mode})

	// 10. fetcher 구성.
	fetcher := newMessagesFetcher(c, msgExportFlagPageSize, buildMessagesFilters())

	// 11. exporter.Run.
	opts := exporter.Options{
		Now:             now,
		StartDate:       startDate,
		EndDate:         endDate,
		PageSize:        msgExportFlagPageSize,
		MaxPages:        msgExportFlagMaxPages,
		Throttle:        msgExportFlagThrottle,
		Fetcher:         fetcher,
		Writer:          rw,
		Reporter:        rep,
		Clock:           clock.Real(),
		StartWindowDate: startWindowDate,
		InitialState:    initialState,
	}
	result, runErr := exporter.Run(ctx(), opts)

	// 12. JSON array 종결. 부분 결과여도 [] 또는 [..]는 닫아 valid JSON 유지.
	if jw, ok := rw.(*output.JSONArrayWriter); ok {
		if closeErr := jw.Close(); closeErr != nil && runErr == nil {
			runErr = closeErr
		}
	}

	// 13. 부분 결과 + 재개 안내. 토큰은 stderr로.
	if result.ResumeToken != "" {
		_, _ = fmt.Fprintf(errOut(), "\n중단됨. 누적 %d건 처리.\n", result.RecordsWritten)
		_, _ = fmt.Fprintf(errOut(), "재개:\n  solactl messages export --output %s --append --resume-token %s\n", msgExportFlagOutput, result.ResumeToken)
	}
	return runErr
}

func resolveResumeState(token, startKey string) (string, exporter.PageState, error) {
	if token != "" {
		tok, err := exporter.DecodeToken(token)
		if err != nil {
			return "", nil, fmt.Errorf("--resume-token 디코드 실패: %w", err)
		}
		return tok.Window, tok.State, nil
	}
	if startKey != "" {
		b, err := json.Marshal(map[string]string{"startKey": startKey})
		if err != nil {
			return "", nil, err
		}
		return "", b, nil
	}
	return "", nil, nil
}

func resolveDateRange(startStr, endStr string, now time.Time) (time.Time, time.Time, error) {
	var start, end time.Time
	if startStr == "" {
		start = now.AddDate(0, 0, -7)
	} else {
		s, err := parseExportDate(startStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--start-date 파싱 실패: %w", err)
		}
		start = s
	}
	if endStr == "" {
		end = now
	} else {
		e, err := parseExportDate(endStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--end-date 파싱 실패: %w", err)
		}
		end = e
	}
	return start, end, nil
}


// buildMessagesFilters는 flag → query 파라미터 매핑. limit/날짜/startKey는 fetcher에서 윈도우별로 채움.
func buildMessagesFilters() url.Values {
	p := url.Values{}
	if msgExportFlagType != "" {
		p.Set("type", msgExportFlagType)
	}
	if msgExportFlagStatus != "" {
		p.Set("statusCode", msgExportFlagStatus)
	}
	if msgExportFlagTo != "" {
		p.Set("to", msgExportFlagTo)
	}
	if msgExportFlagFrom != "" {
		p.Set("from", msgExportFlagFrom)
	}
	if msgExportFlagGroupID != "" {
		p.Set("groupId", msgExportFlagGroupID)
	}
	return p
}

// newMessagesFetcher는 GET /messages/v4/list 호출 PageFetcher를 만든다.
// messages-v4 응답 포맷은 두 가지: {messageList:{id:rec,...}} 또는 {messages:[...]}.
// 양쪽을 모두 처리하되, map 응답의 경우 키를 messageId로 백필한다.
func newMessagesFetcher(c *client.Client, pageSize int, baseFilters url.Values) exporter.PageFetcher {
	return func(ctx context.Context, w exporter.Window, state exporter.PageState) (exporter.Page, error) {
		params := url.Values{}
		maps.Copy(params, baseFilters)
		params.Set("limit", strconv.Itoa(pageSize))
		params.Set("startDate", w.Start.UTC().Format(time.RFC3339))
		params.Set("endDate", w.End.UTC().Format(time.RFC3339))
		if len(state) > 0 {
			var st struct {
				StartKey string `json:"startKey"`
			}
			if err := json.Unmarshal(state, &st); err == nil && st.StartKey != "" {
				params.Set("startKey", st.StartKey)
			}
		}
		raw, err := c.Get(ctx, "messages/v4/list", params)
		if err != nil {
			return exporter.Page{}, err
		}

		var page struct {
			MessageList map[string]json.RawMessage `json:"messageList"`
			Messages    []json.RawMessage          `json:"messages"`
			NextKey     string                     `json:"nextKey"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return exporter.Page{}, fmt.Errorf("응답 파싱 실패: %w", err)
		}

		var records []json.RawMessage
		switch {
		case len(page.Messages) > 0:
			records = page.Messages
		case len(page.MessageList) > 0:
			keys := make([]string, 0, len(page.MessageList))
			for k := range page.MessageList {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			records = make([]json.RawMessage, 0, len(keys))
			for _, k := range keys {
				rec := page.MessageList[k]
				rec = ensureMessageID(rec, k)
				records = append(records, rec)
			}
		}
		var next exporter.PageState
		if page.NextKey != "" {
			nb, err := json.Marshal(map[string]string{"startKey": page.NextKey})
			if err != nil {
				return exporter.Page{}, err
			}
			next = nb
		}
		return exporter.Page{Records: records, Next: next}, nil
	}
}

// ensureMessageID는 record에 messageId 키가 없으면 map key로 백필한다.
// 파싱 실패 시 원본을 그대로 반환 (응답이 비정형이어도 export는 계속 진행).
func ensureMessageID(rec json.RawMessage, id string) json.RawMessage {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(rec, &probe); err != nil {
		return rec
	}
	if _, ok := probe["messageId"]; ok {
		return rec
	}
	idJSON, _ := json.Marshal(id)
	probe["messageId"] = idJSON
	b, err := json.Marshal(probe)
	if err != nil {
		return rec
	}
	return b
}

// --- RowWriter 구현 ---

type messagesCSVRowWriter struct{ cw *output.CSVWriter }

func (m *messagesCSVRowWriter) WriteRecord(rec json.RawMessage) error {
	var mr messageRecord
	if err := json.Unmarshal(rec, &mr); err != nil {
		return fmt.Errorf("메시지 레코드 디코드 실패: %w", err)
	}
	return m.cw.WriteRow(mr.row())
}
func (m *messagesCSVRowWriter) Flush() error { return m.cw.Flush() }

func newMessagesRowWriter(format string, w io.Writer, path string, appendMode, bom bool) (exporter.RowWriter, error) {
	switch format {
	case "csv":
		var reader io.Reader
		if appendMode && path != "-" {
			// Append 모드 헤더 검증을 위해 별도 reader 핸들로 open.
			// write 핸들과 분리 (writer는 O_APPEND, reader는 read-only).
			r, err := os.Open(path)
			if err != nil {
				// 빈/없는 파일에 대한 append는 CSVWriter.verifyAppendHeader에서 명확히 거부.
				return nil, fmt.Errorf("--append: 기존 파일 열기 실패: %w", err)
			}
			defer func() { _ = r.Close() }()
			cw, err := output.NewCSVWriter(w, r, output.CSVOptions{
				Headers:   messagesCSVHeaders,
				AddBOM:    bom,
				Append:    true,
				StripCtrl: true,
			})
			if err != nil {
				return nil, err
			}
			return &messagesCSVRowWriter{cw: cw}, nil
		}
		cw, err := output.NewCSVWriter(w, reader, output.CSVOptions{
			Headers:   messagesCSVHeaders,
			AddBOM:    bom,
			Append:    false,
			StripCtrl: true,
		})
		if err != nil {
			return nil, err
		}
		return &messagesCSVRowWriter{cw: cw}, nil
	case "jsonl":
		return output.NewJSONLWriter(w), nil
	case "json":
		return output.NewJSONArrayWriter(w), nil
	}
	return nil, fmt.Errorf("지원되지 않는 format: %s", format)
}
