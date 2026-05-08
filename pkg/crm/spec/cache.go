package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// CacheTTL is the default TTL for cached OpenAPI specs (1 hour). Mirrors
// solapi-crm-cli sdk/cli/src/spec/cache.ts:6.
const CacheTTL = 60 * time.Minute

// CacheDirEnv lets callers override the cache directory location, primarily
// for tests. Production code should leave this empty.
const CacheDirEnv = "SOLACTL_CACHE_DIR"

// nowFunc is overridable so tests can drive TTL boundaries without sleeping.
var nowFunc = time.Now

// safeKeyRe matches characters that must be replaced for safe filenames.
var safeKeyRe = regexp.MustCompile(`[^a-zA-Z0-9\-_]`)

// cacheMu serialises read/write operations against the cache directory so
// concurrent CLI invocations on the same machine cannot tear writes.
var cacheMu sync.Mutex

// cacheEntry is the on-disk envelope.
type cacheEntry struct {
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"` // unix milliseconds
}

// cacheDir resolves the cache directory, honouring SOLACTL_CACHE_DIR for
// tests. Falls back to ~/.solactl/cache.
func cacheDir() (string, error) {
	if v := os.Getenv(CacheDirEnv); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("홈 디렉토리를 찾을 수 없습니다: %w", err)
	}
	return filepath.Join(home, ".solactl", "cache"), nil
}

func cachePath(key string) (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	safe := safeKeyRe.ReplaceAllString(key, "_")
	return filepath.Join(dir, safe+".json"), nil
}

func ensureCacheDir() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("캐시 디렉토리 생성 실패: %w", err)
	}
	return dir, nil
}

// GetCached returns the cached value for `key`. It returns (nil, nil) if the
// entry is missing, malformed, or expired (unless ignoreTTL is true).
//
// Callers receive a *fresh* json.RawMessage so they can decode into any type.
func GetCached(key string, ignoreTTL bool) (json.RawMessage, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	return getCachedLocked(key, ignoreTTL)
}

func getCachedLocked(key string, ignoreTTL bool) (json.RawMessage, error) {
	path, err := cachePath(key)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, nil // Treat read errors as miss; the loader will refetch.
	}
	var entry cacheEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, nil // Corrupted → miss.
	}
	if !ignoreTTL {
		ageMs := nowFunc().UnixMilli() - entry.Timestamp
		if ageMs > int64(CacheTTL/time.Millisecond) {
			return nil, nil // Expired.
		}
	}
	return entry.Data, nil
}

// SetCache writes the value to disk under `key`. The write is atomic
// (tmp + rename) to avoid torn reads under concurrent invocations.
func SetCache(key string, data json.RawMessage) error {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	dir, err := ensureCacheDir()
	if err != nil {
		return err
	}
	path, err := cachePath(key)
	if err != nil {
		return err
	}

	entry := cacheEntry{Data: data, Timestamp: nowFunc().UnixMilli()}
	body, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("캐시 직렬화 실패: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".cache-*.tmp")
	if err != nil {
		return fmt.Errorf("캐시 임시 파일 생성 실패: %w", err)
	}
	tmpPath := tmp.Name()
	if _, writeErr := tmp.Write(body); writeErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("캐시 쓰기 실패: %w", writeErr)
	}
	if chmodErr := tmp.Chmod(0o600); chmodErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return chmodErr
	}
	if closeErr := tmp.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		return closeErr
	}
	if renameErr := os.Rename(tmpPath, path); renameErr != nil {
		_ = os.Remove(tmpPath)
		return renameErr
	}
	return nil
}

// ClearCache removes every file under the cache directory. Idempotent: a
// missing directory is treated as success (mirrors clear-cache no-op).
func ClearCache() error {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	dir, err := cacheDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("캐시 디렉토리 읽기 실패: %w", err)
	}
	for _, e := range entries {
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return fmt.Errorf("캐시 파일 삭제 실패: %w", err)
		}
	}
	return nil
}

// CacheDirPath returns the resolved cache directory for diagnostics.
func CacheDirPath() (string, error) {
	return cacheDir()
}
