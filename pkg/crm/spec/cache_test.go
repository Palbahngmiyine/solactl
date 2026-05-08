package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

// withTempCache redirects the cache to a t.TempDir and resets nowFunc.
// Returns a function that drives the clock to a fixed instant.
func withTempCache(t *testing.T) (dir string, setNow func(time.Time)) {
	t.Helper()
	dir = t.TempDir()
	t.Setenv(CacheDirEnv, dir)
	original := nowFunc
	t.Cleanup(func() { nowFunc = original })
	setNow = func(at time.Time) { nowFunc = func() time.Time { return at } }
	setNow(time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC))
	return dir, setNow
}

func TestCache_RoundTripWithinTTL(t *testing.T) {
	_, setNow := withTempCache(t)
	payload := json.RawMessage(`{"openapi":"3.0.0","paths":{}}`)
	if err := SetCache("openapi-spec-solapi", payload); err != nil {
		t.Fatalf("SetCache: %v", err)
	}
	setNow(time.Date(2026, 5, 1, 12, 30, 0, 0, time.UTC)) // +30 min, within TTL
	got, err := GetCached("openapi-spec-solapi", false)
	if err != nil {
		t.Fatalf("GetCached: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("payload mismatch: %s", string(got))
	}
}

func TestCache_ExpiredReturnsNil(t *testing.T) {
	_, setNow := withTempCache(t)
	if err := SetCache("openapi-spec-solapi", json.RawMessage(`{"x":1}`)); err != nil {
		t.Fatalf("SetCache: %v", err)
	}
	setNow(time.Date(2026, 5, 1, 14, 0, 1, 0, time.UTC)) // +2h
	got, err := GetCached("openapi-spec-solapi", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != nil {
		t.Errorf("want nil for expired entry, got %s", string(got))
	}
}

func TestCache_IgnoreTTLReturnsStale(t *testing.T) {
	_, setNow := withTempCache(t)
	payload := json.RawMessage(`{"x":2}`)
	if err := SetCache("openapi-spec-solapi", payload); err != nil {
		t.Fatalf("SetCache: %v", err)
	}
	setNow(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) // way past TTL
	got, err := GetCached("openapi-spec-solapi", true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("ignoreTTL must return stale value, got %s", string(got))
	}
}

func TestCache_MissingFileIsNotAnError(t *testing.T) {
	withTempCache(t)
	got, err := GetCached("nonexistent-key", false)
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if got != nil {
		t.Errorf("want nil for missing key, got %s", string(got))
	}
}

func TestCache_CorruptedFileTreatedAsMiss(t *testing.T) {
	dir, _ := withTempCache(t)
	// Write a file directly that fails JSON parse.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "openapi-spec-solapi.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := GetCached("openapi-spec-solapi", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != nil {
		t.Errorf("corrupted entry must be miss, got %s", string(got))
	}
}

func TestCache_KeySanitization(t *testing.T) {
	dir, _ := withTempCache(t)
	if err := SetCache("openapi/spec:weird key", json.RawMessage(`{"k":1}`)); err != nil {
		t.Fatalf("SetCache: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 file, got %d", len(entries))
	}
	if name := entries[0].Name(); name != "openapi_spec_weird_key.json" {
		t.Errorf("unexpected sanitized filename: %q", name)
	}
}

func TestCache_FilePermissionsRestricted(t *testing.T) {
	dir, _ := withTempCache(t)
	if err := SetCache("k", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("SetCache: %v", err)
	}
	st, err := os.Stat(filepath.Join(dir, "k.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := st.Mode().Perm(); mode != 0o600 {
		t.Errorf("want perm 0o600, got %v", mode)
	}
}

func TestCache_ClearIsIdempotent(t *testing.T) {
	withTempCache(t)
	// Empty (no dir created yet) must succeed.
	if err := ClearCache(); err != nil {
		t.Fatalf("first clear: %v", err)
	}
	// After write + clear, second clear is also fine.
	if err := SetCache("k", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("SetCache: %v", err)
	}
	if err := ClearCache(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if err := ClearCache(); err != nil {
		t.Fatalf("second clear: %v", err)
	}
}

func TestCache_ClearRemovesAllFiles(t *testing.T) {
	dir, _ := withTempCache(t)
	for _, k := range []string{"a", "b", "c"} {
		if err := SetCache(k, json.RawMessage(`{}`)); err != nil {
			t.Fatalf("SetCache(%q): %v", k, err)
		}
	}
	if err := ClearCache(); err != nil {
		t.Fatalf("ClearCache: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want empty dir, got %d entries", len(entries))
	}
}

func TestCache_ConcurrentSetsDoNotCorrupt(t *testing.T) {
	withTempCache(t)
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			payload := json.RawMessage(`{"i":` + strconv.Itoa(i) + `}`)
			if err := SetCache("concurrent", payload); err != nil {
				t.Errorf("SetCache: %v", err)
			}
		})
	}
	wg.Wait()
	// Final read must yield a *valid* JSON envelope (no torn write).
	got, err := GetCached("concurrent", false)
	if err != nil {
		t.Fatalf("GetCached: %v", err)
	}
	if got == nil {
		t.Fatal("got nil after concurrent writes")
	}
	var probe map[string]int
	if err := json.Unmarshal(got, &probe); err != nil {
		t.Fatalf("decoded payload not parseable: %v (%s)", err, string(got))
	}
	if _, ok := probe["i"]; !ok {
		t.Errorf("want key 'i' in payload, got %s", string(got))
	}
}
