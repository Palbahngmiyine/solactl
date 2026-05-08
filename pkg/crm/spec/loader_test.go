package spec

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoader_FetchesAndCaches(t *testing.T) {
	withTempCache(t)
	body := `{"openapi":"3.0.0","info":{"title":"crm","version":"1"},"paths":{"/crm-core/v1/records":{"get":{"summary":"list"}}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	loader := &Loader{URL: srv.URL}
	spec, err := loader.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if spec == nil || len(spec.Paths) != 1 {
		t.Fatalf("unexpected spec: %+v", spec)
	}

	// Cache should now be populated; second Load must not hit network.
	srv.Close()
	spec2, err := loader.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if spec2 == nil || len(spec2.Paths) != 1 {
		t.Fatalf("cached spec missing: %+v", spec2)
	}
}

func TestLoader_StaleFallbackOnNetworkFailure(t *testing.T) {
	withTempCache(t)
	good := `{"openapi":"3.0.0","info":{"title":"crm","version":"1"},"paths":{"/crm-core/v1/x":{"get":{"summary":"x"}}}}`
	if err := SetCache(CacheKey, json.RawMessage(good)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Drive clock past TTL so the cache is "stale".
	nowFunc = func() time.Time { return time.Now().Add(2 * time.Hour) }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	var warning string
	loader := &Loader{URL: srv.URL, StaleWarn: func(s string) { warning = s }}
	spec, err := loader.Load(context.Background(), true)
	if err != nil {
		t.Fatalf("expected stale fallback, got error: %v", err)
	}
	if spec == nil || len(spec.Paths) != 1 {
		t.Fatalf("stale spec missing: %+v", spec)
	}
	if !strings.Contains(warning, "캐시된 버전") {
		t.Errorf("StaleWarn not invoked or wrong message: %q", warning)
	}
}

func TestLoader_NetworkFailWithNoCacheReturnsError(t *testing.T) {
	withTempCache(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	loader := &Loader{URL: srv.URL}
	_, err := loader.Load(context.Background(), false)
	if err == nil {
		t.Fatal("want error when fetch fails and no cache exists")
	}
	if !strings.Contains(err.Error(), "OpenAPI spec 로딩 실패") {
		t.Errorf("unexpected err: %v", err)
	}
}

func TestLoader_RejectsSpecWithoutPaths(t *testing.T) {
	withTempCache(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"openapi":"3.0.0","info":{"title":"x","version":"1"}}`))
	}))
	t.Cleanup(srv.Close)

	loader := &Loader{URL: srv.URL}
	_, err := loader.Load(context.Background(), false)
	if err == nil || !strings.Contains(err.Error(), "paths") {
		t.Fatalf("want paths-missing error, got %v", err)
	}
}

func TestLoader_ContextCancellationStopsFetch(t *testing.T) {
	withTempCache(t)
	// Server hangs forever; ctx cancellation must propagate.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	loader := &Loader{URL: srv.URL}
	_, err := loader.Load(ctx, false)
	if err == nil {
		t.Fatal("want error when ctx cancelled")
	}
}

func TestLoader_ForceRefreshSkipsFreshCache(t *testing.T) {
	withTempCache(t)
	if err := SetCache(CacheKey, json.RawMessage(`{"openapi":"3.0.0","info":{"title":"x","version":"1"},"paths":{"/crm-core/v1/cached":{"get":{"summary":"c"}}}}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Clock is at the original "now"; cache is fresh. Force refresh must hit
	// the server even so.
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"openapi":"3.0.0","info":{"title":"x","version":"1"},"paths":{"/crm-core/v1/fresh":{"get":{"summary":"f"}}}}`))
	}))
	t.Cleanup(srv.Close)

	loader := &Loader{URL: srv.URL}
	spec, err := loader.Load(context.Background(), true)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := spec.Paths["/crm-core/v1/fresh"]; !ok {
		t.Errorf("force-refresh did not hit server, got %#v", spec.Paths)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("expected exactly 1 server hit, got %d", hits)
	}
}

// errDoer is an HTTPDoer that always fails. Drives the path where the request
// itself fails before reaching the server.
type errDoer struct{}

func (errDoer) Do(_ *http.Request) (*http.Response, error) {
	return nil, errors.New("dial error")
}

func TestLoader_DoerErrorReachesStaleFallback(t *testing.T) {
	withTempCache(t)
	good := `{"openapi":"3.0.0","info":{"title":"x","version":"1"},"paths":{"/crm-core/v1/x":{"get":{"summary":"x"}}}}`
	if err := SetCache(CacheKey, json.RawMessage(good)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	nowFunc = func() time.Time { return time.Now().Add(48 * time.Hour) }

	loader := &Loader{Client: errDoer{}, StaleWarn: func(string) {}}
	spec, err := loader.Load(context.Background(), true)
	if err != nil {
		t.Fatalf("expected stale fallback on doer err, got %v", err)
	}
	if spec == nil {
		t.Fatal("nil spec")
	}
}
