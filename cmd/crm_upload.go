package cmd

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/solapi/solactl/pkg/client"
	"github.com/solapi/solactl/pkg/crm/output"
)

const (
	crmUploadMax10MB = 10 * 1024 * 1024
	crmUploadMax20MB = 20 * 1024 * 1024
	crmUploadMax5MB  = 5 * 1024 * 1024
	crmUploadMax1MB  = 1 * 1024 * 1024
)

var (
	crmImageUploadExt      = extSet(".jpg", ".jpeg", ".png", ".gif", ".webp")
	crmExcelUploadExt      = extSet(".xls", ".xlsx")
	crmFileUploadExt       = extSet(".jpg", ".jpeg", ".png", ".gif", ".webp", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".hwp", ".txt", ".csv", ".mp4", ".mov", ".mp3")
	crmDocUploadExt        = extSet(".jpg", ".jpeg", ".png", ".gif", ".webp", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".hwp", ".txt")
	crmAgentUploadExt      = extSet(".jpg", ".jpeg", ".png", ".gif", ".webp", ".pdf", ".csv", ".xlsx", ".xls", ".txt")
	crmDocUploadMaxByExt   = extMaxSet(crmUploadMax10MB, ".jpg", ".jpeg", ".png", ".gif", ".webp")
	crmAgentUploadMaxByExt = mergeExtMaxSets(
		extMaxSet(crmUploadMax5MB, ".jpg", ".jpeg", ".png", ".gif", ".webp"),
		extMaxSet(crmUploadMax20MB, ".pdf"),
		extMaxSet(crmUploadMax10MB, ".csv", ".xlsx", ".xls"),
		extMaxSet(crmUploadMax1MB, ".txt"),
	)
)

type crmUploadConstraint struct {
	label         string
	maxBytes      int64
	maxBytesByExt map[string]int64
	extensions    map[string]struct{}
}

type crmUploadRequest struct {
	path   string
	query  url.Values
	fields []client.MultipartField
}

func init() {
	records := ensureStaticCRMResourceCommand("records", "CRM 레코드 리소스 관리")
	records.AddCommand(newCRMUploadCommand("extract-excel-columns", "Excel 파일 컬럼을 추출합니다", "/crm-core/v1/records/import/excel/extract-columns", nil, crmUploadConstraint{"Excel 파일", crmUploadMax10MB, nil, crmExcelUploadExt}, addExcelCommonFlags, buildExtractExcelColumnsUpload))
	records.AddCommand(newCRMUploadCommand("preview-excel-import", "Excel 가져오기 미리보기를 생성합니다", "/crm-core/v1/records/import/excel/preview", nil, crmUploadConstraint{"Excel 파일", crmUploadMax10MB, nil, crmExcelUploadExt}, addExcelPreviewFlags, buildPreviewExcelUpload))
	records.AddCommand(newCRMUploadCommand("import-excel", "Excel 파일로 레코드를 일괄 가져옵니다", "/crm-core/v1/records/import/excel", nil, crmUploadConstraint{"Excel 파일", crmUploadMax10MB, nil, crmExcelUploadExt}, addExcelImportFlags, buildImportExcelUpload))
	records.AddCommand(newCRMUploadCommand("upload-profile-image <recordId>", "레코드 프로필 이미지를 업로드합니다", "/crm-core/v1/records/{recordId}/profile-image", []string{"recordId"}, crmUploadConstraint{"이미지 파일", crmUploadMax10MB, nil, crmImageUploadExt}, nil, buildSimpleUpload))
	records.AddCommand(newCRMUploadCommand("upload-image <recordId>", "레코드 이미지를 업로드합니다", "/crm-core/v1/records/{recordId}/images", []string{"recordId"}, crmUploadConstraint{"이미지 파일", crmUploadMax10MB, nil, crmImageUploadExt}, nil, buildSimpleUpload))
	records.AddCommand(newCRMUploadCommand("upload-attachment <recordId>", "레코드 첨부파일을 업로드합니다", "/crm-core/v1/records/{recordId}/attachments", []string{"recordId"}, crmUploadConstraint{"첨부파일", crmUploadMax10MB, nil, crmFileUploadExt}, addRecordAttachmentFlags, buildRecordAttachmentUpload))

	agent := ensureStaticCRMResourceCommand("agent", "CRM AI 에이전트 리소스 관리")
	agent.AddCommand(newCRMUploadCommand("upload-file", "AI 에이전트 파일을 업로드합니다", "/crm-core/v1/agent/files", nil, crmUploadConstraint{"에이전트 파일", crmUploadMax20MB, crmAgentUploadMaxByExt, crmAgentUploadExt}, nil, buildSimpleUpload))

	documentTemplates := ensureStaticCRMResourceCommand("document-templates", "CRM 문서 템플릿 리소스 관리")
	documentTemplates.AddCommand(newCRMUploadCommand("upload-version-attachment <templateId> <versionId>", "문서 템플릿 버전에 파일을 첨부합니다", "/crm-core/v1/document-templates/{templateId}/versions/{versionId}/attachments", []string{"templateId", "versionId"}, crmUploadConstraint{"첨부파일", 0, nil, nil}, nil, buildSimpleUpload))

	documents := ensureStaticCRMResourceCommand("documents", "CRM 문서 리소스 관리")
	documents.AddCommand(newCRMUploadCommand("upload-attachment <documentId>", "문서 첨부파일을 업로드합니다", "/crm-core/v1/documents/{documentId}/attachments", []string{"documentId"}, crmUploadConstraint{"문서 첨부파일", crmUploadMax20MB, crmDocUploadMaxByExt, crmDocUploadExt}, nil, buildSimpleUpload))

	messageTemplates := ensureStaticCRMResourceCommand("message-templates", "CRM 메시지 템플릿 리소스 관리")
	messageTemplates.AddCommand(newCRMUploadCommand("upload-image <messageTemplateId>", "메시지 템플릿 이미지를 업로드합니다", "/crm-core/v1/message-templates/{messageTemplateId}/image", []string{"messageTemplateId"}, crmUploadConstraint{"이미지 파일", crmUploadMax10MB, nil, crmImageUploadExt}, nil, buildSimpleUpload))

	forms := ensureStaticCRMResourceCommand("forms", "CRM 폼 리소스 관리")
	forms.AddCommand(newCRMUploadCommand("upload-image <formId>", "폼 이미지를 업로드합니다", "/crm-core/v1/forms/{formId}/images", []string{"formId"}, crmUploadConstraint{"이미지 파일", crmUploadMax10MB, nil, crmImageUploadExt}, addFormImageFlags, buildFormImageUpload))
	forms.AddCommand(newPublicCRMUploadCommand("upload-public-file <publicToken>", "공개 폼 파일 첨부를 업로드합니다", "/crm-core/v1/sdk/forms/{publicToken}/upload", []string{"publicToken"}, crmUploadConstraint{"공개 폼 첨부파일", crmUploadMax10MB, nil, crmFileUploadExt}, nil, buildSimpleUpload))

	contents := ensureStaticCRMResourceCommand("contents", "CRM 콘텐츠 리소스 관리")
	contents.AddCommand(newCRMUploadCommand("upload-image <contentId>", "콘텐츠 이미지를 업로드합니다", "/crm-core/v1/contents/{contentId}/images", []string{"contentId"}, crmUploadConstraint{"이미지 파일", crmUploadMax10MB, nil, crmImageUploadExt}, nil, buildSimpleUpload))
}

func newCRMUploadCommand(use, short, path string, pathParams []string, constraint crmUploadConstraint, configure func(*cobra.Command), build func(*cobra.Command, string) (crmUploadRequest, error)) *cobra.Command {
	return newCRMUploadCommandWithAuth(use, short, path, pathParams, constraint, configure, build, true)
}

func newPublicCRMUploadCommand(use, short, path string, pathParams []string, constraint crmUploadConstraint, configure func(*cobra.Command), build func(*cobra.Command, string) (crmUploadRequest, error)) *cobra.Command {
	return newCRMUploadCommandWithAuth(use, short, path, pathParams, constraint, configure, build, false)
}

func newCRMUploadCommandWithAuth(use, short, path string, pathParams []string, constraint crmUploadConstraint, configure func(*cobra.Command), build func(*cobra.Command, string) (crmUploadRequest, error), authRequired bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:          use,
		Short:        short,
		Args:         cobra.ExactArgs(len(pathParams)),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer resetFlagSet(cmd.Flags())

			format, err := uploadOutputFormat(cmd)
			if err != nil {
				return err
			}
			filePath, err := requiredUploadFilePath(cmd)
			if err != nil {
				return err
			}
			if err := validateCRMUploadFile(filePath, constraint); err != nil {
				return err
			}

			resolvedPath := path
			for i, name := range pathParams {
				resolvedPath = strings.ReplaceAll(resolvedPath, "{"+name+"}", encodePathArg(args[i]))
			}
			resolvedPath = strings.TrimPrefix(resolvedPath, "/")

			req, err := build(cmd, resolvedPath)
			if err != nil {
				return err
			}
			if req.query != nil && len(req.query) > 0 {
				req.path = withQuery(req.path, req.query)
			}

			c, err := newCRMUploadClient(authRequired)
			if err != nil {
				return err
			}
			raw, err := c.PostMultipart(ctx(), req.path, req.fields, client.MultipartFile{
				FieldName:   "file",
				Path:        filePath,
				FileName:    filepath.Base(filePath),
				ContentType: crmUploadContentType(filePath),
			})
			if err != nil {
				return fmt.Errorf("%s 업로드 실패: %w", constraint.label, err)
			}

			rendered, err := output.FormatBytes([]byte(raw), format)
			if err != nil {
				return err
			}
			if rendered != "" {
				_, _ = fmt.Fprintln(out(), rendered)
			}
			return nil
		},
	}
	cmd.Flags().String("file", "", "업로드할 파일 경로")
	cmd.Flags().String("format", "", "출력 형식 (json/table/csv, 기본 table; --json이 켜져 있으면 json)")
	if configure != nil {
		configure(cmd)
	}
	return cmd
}

func newCRMUploadClient(authRequired bool) (*client.Client, error) {
	if authRequired {
		return newClient()
	}
	if clientOverride != nil {
		return clientOverride, nil
	}
	c := client.New("", "")
	c.SkipAuthorization = true
	return c, nil
}

func addExcelCommonFlags(cmd *cobra.Command) {
	cmd.Flags().String("sheet-name", "", "대상 Excel 시트 이름")
	cmd.Flags().Bool("has-header", true, "첫 행을 헤더로 처리할지 여부")
}

func addExcelPreviewFlags(cmd *cobra.Command) {
	cmd.Flags().String("entity-id", "", "가져올 대상 개체 ID")
	addExcelCommonFlags(cmd)
}

func addExcelImportFlags(cmd *cobra.Command) {
	cmd.Flags().String("entity-id", "", "가져올 대상 개체 ID")
	cmd.Flags().String("column-mappings", "", "컬럼 매핑 JSON 문자열")
	cmd.Flags().String("link-configs", "", "연결 설정 JSON 문자열")
	cmd.Flags().Bool("skip-automation", false, "가져오기 후 자동화 실행을 건너뜁니다")
	addExcelCommonFlags(cmd)
}

func addRecordAttachmentFlags(cmd *cobra.Command) {
	cmd.Flags().String("title", "", "첨부파일 제목 (최대 200자)")
	cmd.Flags().String("description", "", "첨부파일 설명 (최대 1000자)")
}

func addFormImageFlags(cmd *cobra.Command) {
	cmd.Flags().String("purpose", "", "이미지 용도 (예: cover, question)")
}

func buildSimpleUpload(_ *cobra.Command, path string) (crmUploadRequest, error) {
	return crmUploadRequest{path: path}, nil
}

func buildExtractExcelColumnsUpload(cmd *cobra.Command, path string) (crmUploadRequest, error) {
	fields := make([]client.MultipartField, 0, 2)
	addOptionalStringField(cmd, &fields, "sheet-name", "sheetName")
	addOptionalBoolField(cmd, &fields, "has-header", "hasHeader")
	return crmUploadRequest{path: path, fields: fields}, nil
}

func buildPreviewExcelUpload(cmd *cobra.Command, path string) (crmUploadRequest, error) {
	entityID, err := requiredStringFlag(cmd, "entity-id", "--entity-id 로 가져올 대상 개체 ID를 지정해야 합니다")
	if err != nil {
		return crmUploadRequest{}, err
	}
	fields := []client.MultipartField{{Name: "entityId", Value: entityID}}
	addOptionalStringField(cmd, &fields, "sheet-name", "sheetName")
	addOptionalBoolField(cmd, &fields, "has-header", "hasHeader")
	return crmUploadRequest{path: path, fields: fields}, nil
}

func buildImportExcelUpload(cmd *cobra.Command, path string) (crmUploadRequest, error) {
	entityID, err := requiredStringFlag(cmd, "entity-id", "--entity-id 로 가져올 대상 개체 ID를 지정해야 합니다")
	if err != nil {
		return crmUploadRequest{}, err
	}
	fields := []client.MultipartField{{Name: "entityId", Value: entityID}}
	addOptionalStringField(cmd, &fields, "sheet-name", "sheetName")
	addOptionalBoolField(cmd, &fields, "has-header", "hasHeader")
	if err := addOptionalJSONField(cmd, &fields, "column-mappings", "columnMappings"); err != nil {
		return crmUploadRequest{}, err
	}
	if err := addOptionalJSONField(cmd, &fields, "link-configs", "linkConfigs"); err != nil {
		return crmUploadRequest{}, err
	}
	query := url.Values{}
	if flagChanged(cmd, "skip-automation") {
		v, _ := cmd.Flags().GetBool("skip-automation")
		query.Set("skipAutomation", strconv.FormatBool(v))
	}
	return crmUploadRequest{path: path, fields: fields, query: query}, nil
}

func buildRecordAttachmentUpload(cmd *cobra.Command, path string) (crmUploadRequest, error) {
	fields := make([]client.MultipartField, 0, 2)
	if err := addLimitedOptionalStringField(cmd, &fields, "title", "title", 200); err != nil {
		return crmUploadRequest{}, err
	}
	if err := addLimitedOptionalStringField(cmd, &fields, "description", "description", 1000); err != nil {
		return crmUploadRequest{}, err
	}
	return crmUploadRequest{path: path, fields: fields}, nil
}

func buildFormImageUpload(cmd *cobra.Command, path string) (crmUploadRequest, error) {
	query := url.Values{}
	if flagChanged(cmd, "purpose") {
		v, _ := cmd.Flags().GetString("purpose")
		query.Set("purpose", strings.TrimSpace(v))
	}
	return crmUploadRequest{path: path, query: query}, nil
}

func uploadOutputFormat(cmd *cobra.Command) (output.Format, error) {
	raw, _ := cmd.Flags().GetString("format")
	format, err := output.NormalizeFormat(raw)
	if err != nil {
		return "", err
	}
	if raw == "" && flagJSON {
		return output.FormatJSON, nil
	}
	return format, nil
}

func requiredUploadFilePath(cmd *cobra.Command) (string, error) {
	filePath, _ := cmd.Flags().GetString("file")
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("--file 로 업로드할 파일 경로를 지정해야 합니다")
	}
	return filePath, nil
}

func requiredStringFlag(cmd *cobra.Command, name, message string) (string, error) {
	v, _ := cmd.Flags().GetString(name)
	v = strings.TrimSpace(v)
	if v == "" {
		return "", fmt.Errorf("%s", message)
	}
	return v, nil
}

func addOptionalStringField(cmd *cobra.Command, fields *[]client.MultipartField, flagName, fieldName string) {
	if !flagChanged(cmd, flagName) {
		return
	}
	v, _ := cmd.Flags().GetString(flagName)
	*fields = append(*fields, client.MultipartField{Name: fieldName, Value: v})
}

func addOptionalBoolField(cmd *cobra.Command, fields *[]client.MultipartField, flagName, fieldName string) {
	if !flagChanged(cmd, flagName) {
		return
	}
	v, _ := cmd.Flags().GetBool(flagName)
	*fields = append(*fields, client.MultipartField{Name: fieldName, Value: strconv.FormatBool(v)})
}

func addOptionalJSONField(cmd *cobra.Command, fields *[]client.MultipartField, flagName, fieldName string) error {
	if !flagChanged(cmd, flagName) {
		return nil
	}
	v, _ := cmd.Flags().GetString(flagName)
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("--%s 값은 비어 있을 수 없습니다", flagName)
	}
	var parsed any
	if err := json.Unmarshal([]byte(v), &parsed); err != nil {
		return fmt.Errorf("--%s 값은 JSON 형식이어야 합니다: %w", flagName, err)
	}
	*fields = append(*fields, client.MultipartField{Name: fieldName, Value: v})
	return nil
}

func addLimitedOptionalStringField(cmd *cobra.Command, fields *[]client.MultipartField, flagName, fieldName string, maxRunes int) error {
	if !flagChanged(cmd, flagName) {
		return nil
	}
	v, _ := cmd.Flags().GetString(flagName)
	if len([]rune(v)) > maxRunes {
		return fmt.Errorf("--%s 값은 %d자 이하여야 합니다", flagName, maxRunes)
	}
	*fields = append(*fields, client.MultipartField{Name: fieldName, Value: v})
	return nil
}

func validateCRMUploadFile(path string, constraint crmUploadConstraint) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s을(를) 열 수 없습니다: %w", constraint.label, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s 경로가 디렉터리입니다: %s", constraint.label, path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s은(는) 일반 파일이어야 합니다: %s", constraint.label, path)
	}
	if info.Size() == 0 {
		return fmt.Errorf("%s이(가) 비어 있습니다", constraint.label)
	}
	maxBytes := maxBytesForUpload(path, constraint)
	if maxBytes > 0 && info.Size() > maxBytes {
		return fmt.Errorf("%s 크기는 최대 %s까지 허용됩니다 (현재 %s)", constraint.label, formatBytes(maxBytes), formatBytes(info.Size()))
	}
	ext := strings.ToLower(filepath.Ext(path))
	if len(constraint.extensions) > 0 {
		if _, ok := constraint.extensions[ext]; !ok {
			return fmt.Errorf("%s 형식은 지원하지 않습니다: %s", constraint.label, firstNonEmptyString(ext, "(확장자 없음)"))
		}
	}
	return nil
}

func crmUploadContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	known := map[string]string{
		".csv":  "text/csv",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".hwp":  "application/x-hwp",
		".mp3":  "audio/mpeg",
		".mp4":  "video/mp4",
		".mov":  "video/quicktime",
		".pdf":  "application/pdf",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".txt":  "text/plain",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}
	if ct, ok := known[ext]; ok {
		return ct
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		if i := strings.IndexByte(ct, ';'); i >= 0 {
			return ct[:i]
		}
		return ct
	}
	return "application/octet-stream"
}

func extSet(exts ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(exts))
	for _, ext := range exts {
		out[ext] = struct{}{}
	}
	return out
}

func extMaxSet(maxBytes int64, exts ...string) map[string]int64 {
	out := make(map[string]int64, len(exts))
	for _, ext := range exts {
		out[ext] = maxBytes
	}
	return out
}

func mergeExtMaxSets(sets ...map[string]int64) map[string]int64 {
	out := map[string]int64{}
	for _, set := range sets {
		for ext, maxBytes := range set {
			out[ext] = maxBytes
		}
	}
	return out
}

func maxBytesForUpload(path string, constraint crmUploadConstraint) int64 {
	ext := strings.ToLower(filepath.Ext(path))
	if constraint.maxBytesByExt != nil {
		if maxBytes, ok := constraint.maxBytesByExt[ext]; ok {
			return maxBytes
		}
	}
	return constraint.maxBytes
}

func flagChanged(cmd *cobra.Command, name string) bool {
	f := cmd.Flags().Lookup(name)
	return f != nil && f.Changed
}

func resetFlagSet(flags *pflag.FlagSet) {
	flags.VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		_ = f.Value.Set(f.DefValue)
	})
}

func formatBytes(n int64) string {
	const mb = 1024 * 1024
	if n%mb == 0 {
		return fmt.Sprintf("%dMB", n/mb)
	}
	return fmt.Sprintf("%d bytes", n)
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
