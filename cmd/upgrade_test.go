package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/solapi/solactl/internal/version"
)

func resetUpgradeState(t *testing.T) {
	t.Helper()
	resetFlags()
	t.Cleanup(func() {
		upgradeHTTPClient = nil
		executablePathFunc = os.Executable
		githubBaseURL = "https://api.github.com"
		copyFileFunc = copyFile
		verifyChecksumFunc = verifyChecksum
		validateAssetURLFunc = validateAssetURL
		allowedDownloadHosts = []string{"github.com", "objects.githubusercontent.com"}
		flagTimeout = 30 * time.Second
		clientOverride = nil
		outWriter = nil
		resetFlags()
	})
}

// buildTarGz creates a tar.gz archive containing a fake binary.
func buildTarGz(t *testing.T, binaryName string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("#!/bin/sh\necho upgraded")
	hdr := &tar.Header{
		Name: binaryName,
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write tar content: %v", err)
	}
	_ = tw.Close()
	_ = gw.Close()
	return buf.Bytes()
}

// computeSHA256 returns the hex-encoded SHA256 hash of data.
func computeSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// setupUpgradeServer creates a test server that dynamically rewrites asset download
// URLs to point at itself. Callers should set asset BrowserDownloadURL to just
// the path (e.g., "/download/solactl_1.0.0_darwin_arm64.tar.gz").
// It also injects a checksums.txt asset and adds the test server host to
// allowedDownloadHosts so URL validation passes.
func setupUpgradeServer(t *testing.T, release githubRelease, assetData []byte) *httptest.Server {
	t.Helper()
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			// Build checksums.txt content for all assets
			var checksumLines []string
			if assetData != nil {
				for _, a := range release.Assets {
					if a.Name != "checksums.txt" {
						checksumLines = append(checksumLines, computeSHA256(assetData)+"  "+a.Name)
					}
				}
			}
			checksumContent := strings.Join(checksumLines, "\n") + "\n"

			// Rewrite asset URLs and inject checksums.txt
			rel := release
			for i := range rel.Assets {
				if strings.HasPrefix(rel.Assets[i].BrowserDownloadURL, "/") {
					rel.Assets[i].BrowserDownloadURL = ts.URL + rel.Assets[i].BrowserDownloadURL
				}
			}
			rel.Assets = append(rel.Assets, githubAsset{
				Name:               "checksums.txt",
				BrowserDownloadURL: ts.URL + "/download/checksums.txt",
			})

			// Store the checksums content for serving
			data, _ := json.Marshal(rel)
			w.Header().Set("X-Checksums", checksumContent)
			w.WriteHeader(200)
			_, _ = w.Write(data)
		case r.URL.Path == "/download/checksums.txt":
			// Serve dynamically generated checksums
			if assetData == nil {
				w.WriteHeader(404)
				return
			}
			var lines []string
			for _, a := range release.Assets {
				if a.Name != "checksums.txt" {
					lines = append(lines, computeSHA256(assetData)+"  "+a.Name)
				}
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(strings.Join(lines, "\n") + "\n"))
		case strings.Contains(r.URL.Path, "/download/"):
			if assetData == nil {
				w.WriteHeader(404)
				return
			}
			w.WriteHeader(200)
			_, _ = w.Write(assetData)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(ts.Close)

	// Allow test server URLs to pass URL validation (test servers use http)
	tsHost, _ := url.Parse(ts.URL)
	validateAssetURLFunc = func(rawURL string) error {
		u, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		if u.Host == tsHost.Host {
			return nil
		}
		return validateAssetURL(rawURL)
	}

	return ts
}

func TestUpgrade_AlreadyLatest(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v1.0.0"
	t.Cleanup(func() { version.Version = origVersion })

	release := githubRelease{TagName: "v1.0.0"}
	ts := setupUpgradeServer(t, release, nil)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	buf := captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "이미 최신 버전입니다") {
		t.Errorf("should say already latest, got: %s", output)
	}
}

func TestUpgrade_CurrentGreaterThanLatest(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v2.0.0"
	t.Cleanup(func() { version.Version = origVersion })

	release := githubRelease{TagName: "v1.5.0"}
	ts := setupUpgradeServer(t, release, nil)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	buf := captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "이미 최신 버전입니다") {
		t.Errorf("should say already latest when current > latest")
	}
}

func TestUpgrade_SuccessfulUpgrade(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v1.0.0"
	t.Cleanup(func() { version.Version = origVersion })

	// Create a fake current binary
	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "solactl")
	if err := os.WriteFile(fakeBinary, []byte("old binary"), 0755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}
	executablePathFunc = func() (string, error) { return fakeBinary, nil }

	assetData := buildTarGz(t, "solactl")
	assetName := "solactl_1.1.0_" + testOSArch() + ".tar.gz"
	release := githubRelease{
		TagName: "v1.1.0",
		Assets: []githubAsset{
			{Name: assetName, BrowserDownloadURL: "/download/" + assetName},
		},
	}

	ts := setupUpgradeServer(t, release, assetData)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	buf := captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "업그레이드 완료") {
		t.Errorf("should say upgrade complete, got: %s", output)
	}

	// Verify new binary was written
	newContent, err := os.ReadFile(fakeBinary)
	if err != nil {
		t.Fatalf("read new binary: %v", err)
	}
	if string(newContent) == "old binary" {
		t.Error("binary should have been replaced")
	}

	// Verify backup was cleaned up
	if _, err := os.Stat(fakeBinary + ".old"); !os.IsNotExist(err) {
		t.Error("backup .old file should have been removed")
	}
}

func TestUpgrade_RollbackOnReplaceFail(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v1.0.0"
	t.Cleanup(func() { version.Version = origVersion })

	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "solactl")
	if err := os.WriteFile(fakeBinary, []byte("original"), 0755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}
	executablePathFunc = func() (string, error) { return fakeBinary, nil }

	// Inject copy failure via test seam to exercise the rollback path
	copyErr := fmt.Errorf("injected copy failure")
	copyFileFunc = func(src, dst string, perm os.FileMode) error {
		return copyErr
	}

	assetData := buildTarGz(t, "solactl")
	assetName := "solactl_1.1.0_" + testOSArch() + ".tar.gz"
	release := githubRelease{
		TagName: "v1.1.0",
		Assets: []githubAsset{
			{Name: assetName, BrowserDownloadURL: "/download/" + assetName},
		},
	}
	ts := setupUpgradeServer(t, release, assetData)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when copy fails")
	}
	if !strings.Contains(err.Error(), "롤백 완료") {
		t.Errorf("error should mention rollback completed, got: %v", err)
	}

	// Verify rollback restored the original binary
	content, err := os.ReadFile(fakeBinary)
	if err != nil {
		t.Fatalf("original binary should be restored after rollback: %v", err)
	}
	if string(content) != "original" {
		t.Errorf("binary content should be 'original' after rollback, got: %s", content)
	}

	// Verify .old backup was cleaned up by rollback
	if _, err := os.Stat(fakeBinary + ".old"); !os.IsNotExist(err) {
		t.Error("backup .old file should not exist after rollback restores it")
	}
}

func TestUpgrade_GitHubAPIError(t *testing.T) {
	resetUpgradeState(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	t.Cleanup(ts.Close)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for GitHub API 500")
	}
	if !strings.Contains(err.Error(), "GitHub API 오류") {
		t.Errorf("error should mention GitHub API: %v", err)
	}
}

func TestUpgrade_AssetNotFound(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v1.0.0"
	t.Cleanup(func() { version.Version = origVersion })

	// Release with no matching asset
	release := githubRelease{
		TagName: "v1.1.0",
		Assets: []githubAsset{
			{Name: "solactl_1.1.0_freebsd_mips.tar.gz", BrowserDownloadURL: "http://x"},
		},
	}
	ts := setupUpgradeServer(t, release, nil)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing asset")
	}
	if !strings.Contains(err.Error(), "릴리스를 찾을 수 없습니다") {
		t.Errorf("error should mention missing release: %v", err)
	}
}

func TestUpgrade_InvalidArchive(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v1.0.0"
	t.Cleanup(func() { version.Version = origVersion })

	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "solactl")
	_ = os.WriteFile(fakeBinary, []byte("old"), 0755)
	executablePathFunc = func() (string, error) { return fakeBinary, nil }

	assetName := "solactl_1.1.0_" + testOSArch() + ".tar.gz"
	release := githubRelease{
		TagName: "v1.1.0",
		Assets: []githubAsset{
			{Name: assetName, BrowserDownloadURL: "/download/" + assetName},
		},
	}

	// Invalid tar.gz data
	invalidData := []byte("this is not a valid archive")
	ts := setupUpgradeServer(t, release, invalidData)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid archive")
	}
	if !strings.Contains(err.Error(), "아카이브 추출 실패") {
		t.Errorf("error should mention extraction failure: %v", err)
	}
}

func TestUpgrade_PrereleaseToStable(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v1.0.0-rc1"
	t.Cleanup(func() { version.Version = origVersion })

	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "solactl")
	_ = os.WriteFile(fakeBinary, []byte("old"), 0755)
	executablePathFunc = func() (string, error) { return fakeBinary, nil }

	assetData := buildTarGz(t, "solactl")
	assetName := "solactl_1.0.0_" + testOSArch() + ".tar.gz"
	release := githubRelease{
		TagName: "v1.0.0",
		Assets: []githubAsset{
			{Name: assetName, BrowserDownloadURL: "/download/" + assetName},
		},
	}
	ts := setupUpgradeServer(t, release, assetData)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	buf := captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "업그레이드 완료") {
		t.Error("should upgrade from prerelease to stable")
	}
}

func TestUpgrade_DownloadFail(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v1.0.0"
	t.Cleanup(func() { version.Version = origVersion })

	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "solactl")
	_ = os.WriteFile(fakeBinary, []byte("old"), 0755)
	executablePathFunc = func() (string, error) { return fakeBinary, nil }

	// Bypass URL and checksum validation — this test targets download failure
	validateAssetURLFunc = func(string) error { return nil }
	verifyChecksumFunc = func(context.Context, *http.Client, githubRelease, string, string) error { return nil }

	assetName := "solactl_1.1.0_" + testOSArch() + ".tar.gz"
	release := githubRelease{
		TagName: "v1.1.0",
		Assets: []githubAsset{
			{Name: assetName, BrowserDownloadURL: "PLACEHOLDER"},
		},
	}
	// Server returns 404 for download
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			release.Assets[0].BrowserDownloadURL = "http://127.0.0.1:1/broken"
			data, _ := json.Marshal(release)
			w.WriteHeader(200)
			_, _ = w.Write(data)
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(ts.Close)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for download failure")
	}
	if !strings.Contains(err.Error(), "다운로드 실패") {
		t.Errorf("error should mention download failure: %v", err)
	}
}

func TestUpgrade_ContextCancellation(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v1.0.0"
	t.Cleanup(func() { version.Version = origVersion })

	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "solactl")
	_ = os.WriteFile(fakeBinary, []byte("original"), 0755)
	executablePathFunc = func() (string, error) { return fakeBinary, nil }

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			assetName := "solactl_1.1.0_" + testOSArch() + ".tar.gz"
			release := githubRelease{
				TagName: "v1.1.0",
				Assets: []githubAsset{
					{Name: assetName, BrowserDownloadURL: ts.URL + "/download/" + assetName},
				},
			}
			data, _ := json.Marshal(release)
			w.WriteHeader(200)
			_, _ = w.Write(data)
			return
		}
		// Block until request context is cancelled by timeout
		<-r.Context().Done()
	}))
	t.Cleanup(ts.Close)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	captureBuf(t)

	// Use a short timeout — generous enough for the local release-fetch round trip
	// but short enough that the blocking download handler triggers cancellation.
	rootCmd.SetArgs([]string{"upgrade", "--timeout", "2s"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when context is cancelled during download")
	}

	// Verify original binary is intact
	content, _ := os.ReadFile(fakeBinary)
	if string(content) != "original" {
		t.Error("original binary should remain intact after context cancellation")
	}
}

func TestUpgrade_PathTraversalRejection(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v1.0.0"
	t.Cleanup(func() { version.Version = origVersion })

	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "solactl")
	_ = os.WriteFile(fakeBinary, []byte("old"), 0755)
	executablePathFunc = func() (string, error) { return fakeBinary, nil }

	// Build a tar.gz with a path-traversal entry and NO legitimate binary
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("malicious")
	hdr := &tar.Header{
		Name: "../../../etc/solactl",
		Mode: 0755,
		Size: int64(len(content)),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write(content)
	_ = tw.Close()
	_ = gw.Close()

	assetName := "solactl_1.1.0_" + testOSArch() + ".tar.gz"
	release := githubRelease{
		TagName: "v1.1.0",
		Assets: []githubAsset{
			{Name: assetName, BrowserDownloadURL: "/download/" + assetName},
		},
	}
	ts := setupUpgradeServer(t, release, buf.Bytes())

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when archive only contains traversal paths")
	}
	if !strings.Contains(err.Error(), "찾을 수 없습니다") {
		t.Errorf("error should mention binary not found, got: %v", err)
	}
}

func TestUpgrade_BinaryNotFoundInValidArchive(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v1.0.0"
	t.Cleanup(func() { version.Version = origVersion })

	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "solactl")
	_ = os.WriteFile(fakeBinary, []byte("old"), 0755)
	executablePathFunc = func() (string, error) { return fakeBinary, nil }

	// Build a valid tar.gz that does NOT contain "solactl"
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("some other file")
	hdr := &tar.Header{
		Name: "README.md",
		Mode: 0644,
		Size: int64(len(content)),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write(content)
	_ = tw.Close()
	_ = gw.Close()

	assetName := "solactl_1.1.0_" + testOSArch() + ".tar.gz"
	release := githubRelease{
		TagName: "v1.1.0",
		Assets: []githubAsset{
			{Name: assetName, BrowserDownloadURL: "/download/" + assetName},
		},
	}
	ts := setupUpgradeServer(t, release, buf.Bytes())

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when binary not in archive")
	}
	if !strings.Contains(err.Error(), "찾을 수 없습니다") {
		t.Errorf("error should say binary not found: %v", err)
	}
}

func TestUpgrade_DownloadSizeLimitExceeded(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v1.0.0"
	t.Cleanup(func() { version.Version = origVersion })

	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "solactl")
	_ = os.WriteFile(fakeBinary, []byte("old"), 0755)
	executablePathFunc = func() (string, error) { return fakeBinary, nil }

	// Set a very small size limit for testing
	origMaxSize := maxExtractSize
	maxExtractSize = 10 // 10 bytes
	t.Cleanup(func() { maxExtractSize = origMaxSize })

	// Build a valid tar.gz that exceeds the limit
	assetData := buildTarGz(t, "solactl")

	assetName := "solactl_1.1.0_" + testOSArch() + ".tar.gz"
	release := githubRelease{
		TagName: "v1.1.0",
		Assets: []githubAsset{
			{Name: assetName, BrowserDownloadURL: "/download/" + assetName},
		},
	}
	ts := setupUpgradeServer(t, release, assetData)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when download exceeds size limit")
	}
	if !strings.Contains(err.Error(), "한계") {
		t.Errorf("error should mention size limit, got: %v", err)
	}
}

func TestUpgrade_MalformedCurrentVersion(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "not-a-version"
	t.Cleanup(func() { version.Version = origVersion })

	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "solactl")
	_ = os.WriteFile(fakeBinary, []byte("old"), 0755)
	executablePathFunc = func() (string, error) { return fakeBinary, nil }

	assetData := buildTarGz(t, "solactl")
	assetName := "solactl_1.1.0_" + testOSArch() + ".tar.gz"
	release := githubRelease{
		TagName: "v1.1.0",
		Assets: []githubAsset{
			{Name: assetName, BrowserDownloadURL: "/download/" + assetName},
		},
	}
	ts := setupUpgradeServer(t, release, assetData)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	buf := captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("should still upgrade with malformed version: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "경고") {
		t.Error("should print warning about unparseable version")
	}
	if !strings.Contains(output, "업그레이드 완료") {
		t.Error("should complete upgrade despite malformed version")
	}
}

func TestDownloadFile_SizeLimitCleanup(t *testing.T) {
	resetUpgradeState(t)

	origMaxSize := maxExtractSize
	maxExtractSize = 10
	t.Cleanup(func() { maxExtractSize = origMaxSize })

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write(bytes.Repeat([]byte("x"), 100))
	}))
	t.Cleanup(ts.Close)

	destPath := filepath.Join(t.TempDir(), "download")
	err := downloadFile(context.Background(), ts.Client(), ts.URL, destPath)
	if err == nil {
		t.Fatal("expected error for oversized download")
	}
	if !strings.Contains(err.Error(), "한계") {
		t.Errorf("error should mention size limit, got: %v", err)
	}
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Error("oversized download file should have been cleaned up")
	}
}

func TestValidateAssetURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{
			name: "valid github.com URL",
			url:  "https://github.com/solapi/solactl/releases/download/v1.0.0/solactl.tar.gz",
		},
		{
			name: "valid objects.githubusercontent.com URL",
			url:  "https://objects.githubusercontent.com/some/path",
		},
		{
			name:    "http scheme rejected",
			url:     "http://github.com/solapi/solactl/releases/download/v1.0.0/solactl.tar.gz",
			wantErr: "안전하지 않은 다운로드 URL 스킴",
		},
		{
			name:    "untrusted host rejected",
			url:     "https://evil.com/solactl.tar.gz",
			wantErr: "신뢰할 수 없는 다운로드 호스트",
		},
		{
			name:    "empty URL rejected",
			url:     "",
			wantErr: "안전하지 않은 다운로드 URL 스킴",
		},
		{
			name:    "ftp scheme rejected",
			url:     "ftp://github.com/solactl.tar.gz",
			wantErr: "안전하지 않은 다운로드 URL 스킴",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAssetURL(tt.url)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestVerifyChecksum_Valid(t *testing.T) {
	resetUpgradeState(t)

	assetData := buildTarGz(t, "solactl")
	assetName := "solactl_1.0.0_linux_amd64.tar.gz"
	expectedHash := computeSHA256(assetData)
	checksumBody := expectedHash + "  " + assetName + "\n"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(checksumBody))
	}))
	t.Cleanup(ts.Close)

	// Write asset to temp file
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, assetName)
	if err := os.WriteFile(archivePath, assetData, 0644); err != nil {
		t.Fatalf("write test archive: %v", err)
	}

	release := githubRelease{
		Assets: []githubAsset{
			{Name: "checksums.txt", BrowserDownloadURL: ts.URL + "/checksums.txt"},
		},
	}

	err := verifyChecksum(context.Background(), ts.Client(), release, archivePath, assetName)
	if err != nil {
		t.Fatalf("expected valid checksum, got error: %v", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	resetUpgradeState(t)

	assetData := buildTarGz(t, "solactl")
	assetName := "solactl_1.0.0_linux_amd64.tar.gz"
	fakeHash := "0000000000000000000000000000000000000000000000000000000000000000"
	checksumBody := fakeHash + "  " + assetName + "\n"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(checksumBody))
	}))
	t.Cleanup(ts.Close)

	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, assetName)
	if err := os.WriteFile(archivePath, assetData, 0644); err != nil {
		t.Fatalf("write test archive: %v", err)
	}

	release := githubRelease{
		Assets: []githubAsset{
			{Name: "checksums.txt", BrowserDownloadURL: ts.URL + "/checksums.txt"},
		},
	}

	err := verifyChecksum(context.Background(), ts.Client(), release, archivePath, assetName)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "체크섬 불일치") {
		t.Errorf("error should mention checksum mismatch, got: %v", err)
	}
}

func TestVerifyChecksum_MissingChecksumsTxt(t *testing.T) {
	resetUpgradeState(t)

	release := githubRelease{
		Assets: []githubAsset{
			{Name: "solactl_1.0.0_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/archive"},
		},
	}

	err := verifyChecksum(context.Background(), http.DefaultClient, release, "/nonexistent", "solactl_1.0.0_linux_amd64.tar.gz")
	if err == nil {
		t.Fatal("expected error when checksums.txt missing from release")
	}
	if !strings.Contains(err.Error(), "checksums.txt") {
		t.Errorf("error should mention checksums.txt, got: %v", err)
	}
}

func TestVerifyChecksum_AssetNotInChecksums(t *testing.T) {
	resetUpgradeState(t)

	checksumBody := "abc123  other_file.tar.gz\n"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(checksumBody))
	}))
	t.Cleanup(ts.Close)

	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "solactl.tar.gz")
	if err := os.WriteFile(archivePath, []byte("data"), 0644); err != nil {
		t.Fatalf("write test archive: %v", err)
	}

	release := githubRelease{
		Assets: []githubAsset{
			{Name: "checksums.txt", BrowserDownloadURL: ts.URL + "/checksums.txt"},
		},
	}

	err := verifyChecksum(context.Background(), ts.Client(), release, archivePath, "solactl.tar.gz")
	if err == nil {
		t.Fatal("expected error when asset not found in checksums.txt")
	}
	if !strings.Contains(err.Error(), "해시를 찾을 수 없습니다") {
		t.Errorf("error should mention hash not found, got: %v", err)
	}
}

func TestUpgrade_UntrustedURLRejected(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v1.0.0"
	t.Cleanup(func() { version.Version = origVersion })

	assetName := "solactl_1.1.0_" + testOSArch() + ".tar.gz"
	release := githubRelease{
		TagName: "v1.1.0",
		Assets: []githubAsset{
			{Name: assetName, BrowserDownloadURL: "https://evil.com/download/" + assetName},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := json.Marshal(release)
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}))
	t.Cleanup(ts.Close)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	// Use the real validateAssetURL (don't override)
	validateAssetURLFunc = validateAssetURL
	captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for untrusted download URL")
	}
	if !strings.Contains(err.Error(), "다운로드 URL 검증 실패") {
		t.Errorf("error should mention URL validation failure, got: %v", err)
	}
}

func TestUpgrade_ChecksumMismatchRejected(t *testing.T) {
	resetUpgradeState(t)

	origVersion := version.Version
	version.Version = "v1.0.0"
	t.Cleanup(func() { version.Version = origVersion })

	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "solactl")
	_ = os.WriteFile(fakeBinary, []byte("old"), 0755)
	executablePathFunc = func() (string, error) { return fakeBinary, nil }

	assetData := buildTarGz(t, "solactl")
	assetName := "solactl_1.1.0_" + testOSArch() + ".tar.gz"
	release := githubRelease{
		TagName: "v1.1.0",
		Assets: []githubAsset{
			{Name: assetName, BrowserDownloadURL: "/download/" + assetName},
		},
	}

	// Set up server that returns wrong checksums
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			rel := release
			for i := range rel.Assets {
				if strings.HasPrefix(rel.Assets[i].BrowserDownloadURL, "/") {
					rel.Assets[i].BrowserDownloadURL = ts.URL + rel.Assets[i].BrowserDownloadURL
				}
			}
			rel.Assets = append(rel.Assets, githubAsset{
				Name:               "checksums.txt",
				BrowserDownloadURL: ts.URL + "/download/checksums.txt",
			})
			data, _ := json.Marshal(rel)
			w.WriteHeader(200)
			_, _ = w.Write(data)
		case r.URL.Path == "/download/checksums.txt":
			// Return wrong checksum
			fakeHash := "0000000000000000000000000000000000000000000000000000000000000000"
			w.WriteHeader(200)
			_, _ = w.Write([]byte(fakeHash + "  " + assetName + "\n"))
		case strings.Contains(r.URL.Path, "/download/"):
			w.WriteHeader(200)
			_, _ = w.Write(assetData)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(ts.Close)

	githubBaseURL = ts.URL
	upgradeHTTPClient = ts.Client()
	// Allow test server URL
	tsHost, _ := url.Parse(ts.URL)
	validateAssetURLFunc = func(rawURL string) error {
		u, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		if u.Host == tsHost.Host {
			return nil
		}
		return validateAssetURL(rawURL)
	}
	captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for checksum mismatch")
	}
	if !strings.Contains(err.Error(), "체크섬 검증 실패") {
		t.Errorf("error should mention checksum verification failure, got: %v", err)
	}
}

// testOSArch returns the current os_arch string for asset naming.
func testOSArch() string {
	return runtime.GOOS + "_" + runtime.GOARCH
}
