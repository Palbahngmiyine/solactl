package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/solapi/solactl/internal/version"
)

func resetUpgradeState(t *testing.T) {
	t.Helper()
	resetFlags()
	t.Cleanup(func() {
		upgradeHTTPClient = nil
		executablePathFunc = os.Executable
		githubBaseURL = "https://api.github.com"
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
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

// setupUpgradeServer creates a test server that dynamically rewrites asset download
// URLs to point at itself. Callers should set asset BrowserDownloadURL to just
// the path (e.g., "/download/solactl_1.0.0_darwin_arm64.tar.gz").
func setupUpgradeServer(t *testing.T, release githubRelease, assetData []byte) *httptest.Server {
	t.Helper()
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			// Rewrite asset URLs to point at this test server
			rel := release
			for i := range rel.Assets {
				if strings.HasPrefix(rel.Assets[i].BrowserDownloadURL, "/") {
					rel.Assets[i].BrowserDownloadURL = ts.URL + rel.Assets[i].BrowserDownloadURL
				}
			}
			data, _ := json.Marshal(rel)
			w.WriteHeader(200)
			_, _ = w.Write(data)
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

	// Create a fake current binary in a read-only directory situation
	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "solactl")
	if err := os.WriteFile(fakeBinary, []byte("original"), 0755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}

	// Make executablePathFunc return a path in a non-writable subdirectory
	readonlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(readonlyDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	readonlyBinary := filepath.Join(readonlyDir, "solactl")
	if err := os.WriteFile(readonlyBinary, []byte("original"), 0755); err != nil {
		t.Fatalf("create binary: %v", err)
	}

	executablePathFunc = func() (string, error) { return readonlyBinary, nil }

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

	// Make directory read-only after backup to trigger copy failure
	os.Rename(readonlyBinary, readonlyBinary+".old")
	os.Chmod(readonlyDir, 0444)
	t.Cleanup(func() { os.Chmod(readonlyDir, 0755) })

	captureBuf(t)

	rootCmd.SetArgs([]string{"upgrade"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when copy fails")
	}

	// Restore permissions to check
	os.Chmod(readonlyDir, 0755)

	// Verify rollback happened
	content, err := os.ReadFile(readonlyBinary)
	if err != nil {
		// Rollback should have restored the file
		content, _ = os.ReadFile(readonlyBinary + ".old")
	}
	if string(content) != "original" {
		t.Errorf("original binary should be preserved after rollback")
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

// testOSArch returns the current os_arch string for asset naming.
func testOSArch() string {
	return runtime.GOOS + "_" + runtime.GOARCH
}
