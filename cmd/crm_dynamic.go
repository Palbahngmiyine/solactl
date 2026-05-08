package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/pkg/client"
	"github.com/solapi/solactl/pkg/crm/output"
	"github.com/solapi/solactl/pkg/crm/spec"
)

// crmLoaderOverride lets tests inject a deterministic Loader. Production
// leaves it nil; the loader uses the upstream URL.
var crmLoaderOverride *spec.Loader

// RegisterDynamicCRM resolves the OpenAPI spec, then mounts dynamic resource
// subcommands under `solactl crm`. Failures here must NEVER block the rest of
// the CLI:
//
//   - OpenAPI fetch failure with no cache → skip + stderr warning
//   - OpenAPI fetch failure with stale cache → register from stale + warning
//
// Credentials are intentionally not loaded here: this runs before cobra parses
// persistent flags, so `--profile`, `--api-key`, and `--api-secret` are only
// reliable during command execution in newClient().
func RegisterDynamicCRM(ctx context.Context) {
	loader := crmLoaderOverride
	if loader == nil {
		loader = &spec.Loader{
			StaleWarn: func(msg string) { _, _ = fmt.Fprintln(errOut(), msg) },
		}
	}

	apiSpec, err := loader.Load(ctx, false)
	if err != nil {
		_, _ = fmt.Fprintf(errOut(), "⚠ CRM OpenAPI spec 로딩 실패 — 동적 명령이 등록되지 않습니다: %v\n", err)
		return
	}

	commands := spec.MapSpec(apiSpec)
	for _, resource := range spec.Resources(commands) {
		resourceCmd := &cobra.Command{
			Use:   resource,
			Short: fmt.Sprintf("%s 리소스 관리", resource),
		}
		for _, mc := range spec.CommandsForResource(commands, resource) {
			resourceCmd.AddCommand(buildDynamicSubcommand(mc))
		}
		crmCmd.AddCommand(resourceCmd)
	}
}

// dynamicFlags holds per-command flag storage. cobra writes flag values into
// these slots during parse; the RunE closure reads them afterwards.
type dynamicFlags struct {
	query    map[string]*string
	data     string
	dataFile string
	format   string
}

func buildDynamicSubcommand(mc spec.MappedCommand) *cobra.Command {
	var useBuilder strings.Builder
	useBuilder.WriteString(mc.Action)
	for _, p := range mc.PathParams {
		useBuilder.WriteString(" <")
		useBuilder.WriteString(p.Name)
		useBuilder.WriteByte('>')
	}

	short := strings.TrimSpace(mc.Summary)
	if short == "" {
		short = fmt.Sprintf("%s %s", mc.Method, mc.Path)
	}

	sub := &cobra.Command{
		Use:   useBuilder.String(),
		Short: short,
		Args:  cobra.ExactArgs(len(mc.PathParams)),
	}

	flags := &dynamicFlags{query: make(map[string]*string, len(mc.QueryParams))}
	for _, q := range mc.QueryParams {
		v := new(string)
		flags.query[q.Name] = v
		desc := q.Description
		if desc == "" {
			desc = q.Name
		}
		sub.Flags().StringVar(v, q.Name, "", desc)
		if q.Required {
			_ = sub.MarkFlagRequired(q.Name)
		}
	}
	if mc.HasBody {
		sub.Flags().StringVar(&flags.data, "data", "", "요청 본문 (JSON 문자열)")
		sub.Flags().StringVar(&flags.dataFile, "data-file", "", "요청 본문 파일 경로")
	}
	sub.Flags().StringVar(&flags.format, "format", "", "출력 형식 (json/table/csv, 기본 table; --json이 켜져 있으면 json)")

	sub.RunE = func(_ *cobra.Command, args []string) error {
		return runDynamicCommand(mc, args, flags)
	}
	return sub
}

func runDynamicCommand(mc spec.MappedCommand, args []string, flags *dynamicFlags) error {
	format, err := output.NormalizeFormat(flags.format)
	if err != nil {
		return err
	}
	if flags.format == "" && flagJSON {
		format = output.FormatJSON
	}

	path := mc.Path
	for i, p := range mc.PathParams {
		encoded := encodePathArg(args[i])
		path = strings.ReplaceAll(path, "{"+p.Name+"}", encoded)
		path = strings.ReplaceAll(path, ":"+p.Name, encoded)
	}
	path = strings.TrimPrefix(path, "/")

	q := url.Values{}
	for _, p := range mc.QueryParams {
		if v := flags.query[p.Name]; v != nil && *v != "" {
			q.Set(p.Name, *v)
		}
	}

	var body any
	if mc.HasBody {
		body, err = readRequestBody(flags.data, flags.dataFile, mc.BodyRequired)
		if err != nil {
			return err
		}
	}

	c, err := newClient()
	if err != nil {
		return err
	}

	raw, err := dispatch(ctx(), c, mc.Method, path, q, body)
	if err != nil {
		return fmt.Errorf("%s %s 호출 실패: %w", mc.Method, mc.Path, err)
	}

	rendered, err := output.FormatBytes([]byte(raw), format)
	if err != nil {
		return err
	}
	if rendered != "" {
		_, _ = fmt.Fprintln(out(), rendered)
	}
	return nil
}

func readRequestBody(dataFlag, dataFileFlag string, required bool) (any, error) {
	if dataFileFlag != "" {
		raw, err := os.ReadFile(dataFileFlag)
		if err != nil {
			return nil, fmt.Errorf("--data-file 읽기 실패: %w", err)
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("--data-file JSON 파싱 실패: %w", err)
		}
		return v, nil
	}
	if dataFlag != "" {
		var v any
		if err := json.Unmarshal([]byte(dataFlag), &v); err != nil {
			return nil, fmt.Errorf("--data JSON 파싱 실패: %w", err)
		}
		return v, nil
	}
	if required {
		return nil, errors.New("--data 또는 --data-file 로 요청 본문을 지정해야 합니다")
	}
	return nil, nil
}

// dispatch routes the method to the right client helper. mapper.MapSpec
// already uppercases mc.Method, so no normalisation is needed here.
func dispatch(ctx context.Context, c *client.Client, method, path string, q url.Values, body any) (json.RawMessage, error) {
	switch method {
	case http.MethodGet:
		return c.Get(ctx, path, q)
	case http.MethodPost:
		return c.Post(ctx, withQuery(path, q), body)
	case http.MethodPut:
		return c.Put(ctx, withQuery(path, q), body)
	case http.MethodPatch:
		return c.Patch(ctx, withQuery(path, q), body)
	case http.MethodDelete:
		return c.Delete(ctx, withQuery(path, q))
	}
	return nil, fmt.Errorf("지원하지 않는 HTTP 메서드: %s", method)
}

// encodePathArg matches JS `encodeURIComponent` semantics so path arguments
// containing `/` cannot break out of their segment. url.PathEscape is not a
// substitute — it leaves `/` un-encoded.
func encodePathArg(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(s))
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z',
			c >= 'a' && c <= 'z',
			c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0F])
		}
	}
	return b.String()
}

func withQuery(path string, q url.Values) string {
	if len(q) == 0 {
		return path
	}
	if strings.Contains(path, "?") {
		return path + "&" + q.Encode()
	}
	return path + "?" + q.Encode()
}
