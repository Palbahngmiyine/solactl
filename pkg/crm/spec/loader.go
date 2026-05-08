package spec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultSpecURL is the upstream OpenAPI spec endpoint. Mirrors
// solapi-crm-cli sdk/cli/src/utils/env.ts:6.
const DefaultSpecURL = "https://api.solapi.com/crm-core/v1/public/openapi/json"

// CacheKey is the on-disk cache key for the (single) environment.
const CacheKey = "openapi-spec-solapi"

// HTTPDoer is the subset of *http.Client used by the loader. Tests pass a
// custom doer to drive failure scenarios.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Loader fetches the CRM OpenAPI spec and persists it on disk.
type Loader struct {
	URL    string   // empty → DefaultSpecURL
	Client HTTPDoer // nil → http.DefaultClient
	// StaleWarn receives a single-line warning when stale fallback kicks in.
	// Production wires this to stderr; tests inspect the captured value.
	StaleWarn func(string)
}

// Load returns a parsed OpenAPI spec, preferring the on-disk cache. If the
// cache is missing or expired, the loader fetches over HTTP. On HTTP failure
// it falls back to a stale cache (if any) with a warning to StaleWarn.
//
// `forceRefresh` skips the fresh cache lookup but still allows stale fallback
// — matches loader.ts:49-79 semantics.
func (l *Loader) Load(ctx context.Context, forceRefresh bool) (*OpenApiSpec, error) {
	if !forceRefresh {
		if raw, _ := GetCached(CacheKey, false); raw != nil {
			spec, err := parseSpec(raw)
			if err == nil {
				return spec, nil
			}
			// Cached blob looked like JSON but didn't satisfy the spec shape.
			// Fall through to fetch — do not surface the stale entry.
		}
	}

	specURL := l.URL
	if specURL == "" {
		specURL = DefaultSpecURL
	}
	doer := l.Client
	if doer == nil {
		doer = http.DefaultClient
	}

	raw, fetchErr := doFetch(ctx, doer, specURL)
	if fetchErr == nil {
		spec, err := parseSpec(raw)
		if err != nil {
			return l.tryStale(err)
		}
		// Persist the *raw* bytes so subsequent loads do not re-encode.
		if err := SetCache(CacheKey, raw); err != nil {
			// Cache write failures must not block command execution.
			if l.StaleWarn != nil {
				l.StaleWarn("⚠ OpenAPI spec 캐시 저장 실패: " + err.Error())
			}
		}
		return spec, nil
	}
	return l.tryStale(fetchErr)
}

func (l *Loader) tryStale(cause error) (*OpenApiSpec, error) {
	stale, _ := GetCached(CacheKey, true)
	if stale != nil {
		spec, err := parseSpec(stale)
		if err == nil {
			if l.StaleWarn != nil {
				l.StaleWarn("⚠ OpenAPI spec을 갱신할 수 없어 캐시된 버전을 사용합니다.")
			}
			return spec, nil
		}
	}
	return nil, cause
}

func doFetch(ctx context.Context, doer HTTPDoer, url string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("OpenAPI spec 요청 생성 실패: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	// Apply a soft per-request timeout so that a hung server cannot block the
	// CLI indefinitely. Caller-supplied ctx still wins if it's tighter.
	ctxFetch, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()
	req = req.WithContext(ctxFetch)

	resp, err := doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAPI spec 로딩 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenAPI spec 로딩 실패 (%d): %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("OpenAPI spec 응답 읽기 실패: %w", err)
	}
	return body, nil
}

func parseSpec(raw json.RawMessage) (*OpenApiSpec, error) {
	var spec OpenApiSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("OpenAPI spec 파싱 실패: %w", err)
	}
	if spec.Paths == nil {
		return nil, errors.New("유효하지 않은 OpenAPI spec: paths 필드가 없습니다")
	}
	return &spec, nil
}
