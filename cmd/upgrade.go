package cmd

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/internal/version"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "최신 버전으로 업그레이드합니다",
	RunE:  runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

// isAllowedDownloadHost reports whether host is a trusted download origin.
func isAllowedDownloadHost(host string) bool {
	switch host {
	case "github.com", "objects.githubusercontent.com":
		return true
	default:
		return false
	}
}

// Test seams
var (
	upgradeHTTPClient      *http.Client
	executablePathFunc     = os.Executable
	githubBaseURL          = "https://api.github.com"
	maxExtractSize   int64 = 100 * 1024 * 1024 // 100MB
	copyFileFunc           = copyFile
	verifyChecksumFunc     = verifyChecksum
	validateAssetURLFunc   = validateAssetURL
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// newTrustedHTTPClient returns an HTTP client that rejects redirects to
// untrusted hosts. This prevents a trusted initial URL from being redirected
// to an attacker-controlled server.
func newTrustedHTTPClient(base *http.Client) *http.Client {
	c := *base // shallow copy
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("너무 많은 리다이렉트")
		}
		if req.URL.Scheme != "https" {
			return fmt.Errorf("리다이렉트 대상이 신뢰할 수 없음: %s", req.URL.Host)
		}
		if port := req.URL.Port(); port != "" && port != "443" {
			return fmt.Errorf("리다이렉트 대상이 비표준 포트 사용: %s", req.URL.Host)
		}
		if !isAllowedDownloadHost(req.URL.Hostname()) {
			return fmt.Errorf("리다이렉트 대상이 신뢰할 수 없음: %s", req.URL.Host)
		}
		return nil
	}
	return &c
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	w := out()
	_, _ = fmt.Fprintln(w, "최신 버전 확인 중...")

	httpC := upgradeHTTPClient
	if httpC == nil {
		httpC = http.DefaultClient
	}

	// 1. Fetch latest release
	releaseURL := githubBaseURL + "/repos/solapi/solactl/releases/latest"
	req, err := http.NewRequestWithContext(ctx(), http.MethodGet, releaseURL, nil)
	if err != nil {
		return fmt.Errorf("GitHub API 요청 생성 실패: %w", err)
	}
	resp, err := httpC.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub API 요청 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("GitHub API 오류 (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("릴리스 정보 파싱 실패: %w", err)
	}

	// 2. Compare versions
	latest, err := version.ParseSemver(release.TagName)
	if err != nil {
		return fmt.Errorf("최신 버전 파싱 실패: %w", err)
	}

	current, err := version.ParseSemver(version.Version)
	if err != nil {
		if version.Version != "dev" && version.Version != "" {
			_, _ = fmt.Fprintf(w, "경고: 현재 버전(%s) 파싱 실패, 업그레이드를 계속합니다\n", version.Version)
		}
		current = version.Semver{}
	}

	if version.CompareSemver(current, latest) >= 0 {
		_, _ = fmt.Fprintf(w, "이미 최신 버전입니다 (%s)\n", version.Version)
		return nil
	}

	_, _ = fmt.Fprintf(w, "%s → %s 업그레이드를 시작합니다\n", version.Version, release.TagName)

	// 3. Find matching asset
	ver := strings.TrimPrefix(release.TagName, "v")
	osName := runtime.GOOS
	archName := runtime.GOARCH
	ext := "tar.gz"
	if osName == "windows" {
		ext = "zip"
	}
	assetName := fmt.Sprintf("solactl_%s_%s_%s.%s", ver, osName, archName, ext)

	var assetURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			assetURL = a.BrowserDownloadURL
			break
		}
	}
	if assetURL == "" {
		return fmt.Errorf("현재 플랫폼(%s/%s)에 맞는 릴리스를 찾을 수 없습니다: %s", osName, archName, assetName)
	}

	// 3a. Validate download URL points to a trusted host
	if err := validateAssetURLFunc(assetURL); err != nil {
		return fmt.Errorf("다운로드 URL 검증 실패: %w", err)
	}

	_, _ = fmt.Fprintf(w, "다운로드 중... %s\n", assetName)

	// 4. Download to temp directory
	tmpDir, err := os.MkdirTemp("", "solactl-upgrade-*")
	if err != nil {
		return fmt.Errorf("임시 디렉토리 생성 실패: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Use a redirect-validating client for file downloads
	trustedC := newTrustedHTTPClient(httpC)

	archivePath := filepath.Join(tmpDir, assetName)
	if err := downloadFile(ctx(), trustedC, assetURL, archivePath); err != nil {
		return fmt.Errorf("다운로드 실패: %w", err)
	}

	// 4a. Verify checksum
	_, _ = fmt.Fprintln(w, "체크섬 검증 중...")
	if err := verifyChecksumFunc(ctx(), trustedC, release, archivePath, assetName); err != nil {
		return fmt.Errorf("체크섬 검증 실패: %w", err)
	}

	// 5. Extract binary
	_, _ = fmt.Fprintln(w, "아카이브 추출 중...")
	binaryName := "solactl"
	if osName == "windows" {
		binaryName = "solactl.exe"
	}

	extractedPath, err := extractBinary(archivePath, tmpDir, binaryName, ext)
	if err != nil {
		return fmt.Errorf("아카이브 추출 실패: %w", err)
	}

	// 6. Replace binary
	_, _ = fmt.Fprintln(w, "바이너리 교체 중...")
	execPath, err := executablePathFunc()
	if err != nil {
		return fmt.Errorf("현재 바이너리 경로 확인 실패: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("심볼릭 링크 해석 실패: %w", err)
	}

	backupPath := execPath + ".old"

	// Remove stale backup from a previous failed upgrade
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("이전 백업 파일 삭제 실패 (%s): %w", backupPath, err)
	}

	// Backup current binary (on Windows, rename of a running .exe is allowed)
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("기존 바이너리 백업 실패: %w", err)
	}

	// Preserve existing binary's permissions
	backupInfo, err := os.Stat(backupPath)
	filePerm := os.FileMode(0755)
	if err == nil {
		filePerm = backupInfo.Mode().Perm()
	}

	// Copy new binary to the original path
	if err := copyFileFunc(extractedPath, execPath, filePerm); err != nil {
		// Remove partial file before rollback (required on Windows where
		// os.Rename cannot replace an existing destination)
		_ = os.Remove(execPath)
		if rollbackErr := os.Rename(backupPath, execPath); rollbackErr != nil {
			return fmt.Errorf("새 바이너리 설치 실패: %w (롤백도 실패: %v — 수동 복구 필요: mv %s %s)", err, rollbackErr, backupPath, execPath)
		}
		return fmt.Errorf("새 바이너리 설치 실패 (롤백 완료): %w", err)
	}

	// On Windows the .old file may still be locked; removal failure is not fatal
	_ = os.Remove(backupPath)

	_, _ = fmt.Fprintf(w, "업그레이드 완료! solactl %s\n", release.TagName)
	return nil
}

// validateAssetURL ensures the download URL uses HTTPS and points to a trusted host
// on the standard port (443). Non-standard ports are rejected to prevent bypass.
func validateAssetURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("다운로드 URL 파싱 실패: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("안전하지 않은 다운로드 URL 스킴: %s (https 필수)", u.Scheme)
	}
	if port := u.Port(); port != "" && port != "443" {
		return fmt.Errorf("비표준 포트 사용 불가: %s", port)
	}
	if !isAllowedDownloadHost(u.Hostname()) {
		return fmt.Errorf("신뢰할 수 없는 다운로드 호스트: %s", u.Hostname())
	}
	return nil
}

// verifyChecksum downloads checksums.txt from the GitHub release assets and
// verifies the SHA256 hash of the downloaded archive. Returns an error if
// checksums.txt is missing from the release (verification is mandatory).
// Expected format: "<hex-sha256>  <filename>\n" per line (whitespace-separated).
func verifyChecksum(reqCtx context.Context, httpC *http.Client, release githubRelease, archivePath, assetName string) error {
	// Find checksums.txt asset
	var checksumURL string
	for _, a := range release.Assets {
		if a.Name == "checksums.txt" {
			checksumURL = a.BrowserDownloadURL
			break
		}
	}
	if checksumURL == "" {
		return fmt.Errorf("릴리스에 checksums.txt가 없습니다 (무결성 검증 불가)")
	}

	// Validate checksums.txt URL against the same trusted-host allowlist
	if err := validateAssetURLFunc(checksumURL); err != nil {
		return fmt.Errorf("체크섬 다운로드 URL 검증 실패: %w", err)
	}

	// Download checksums.txt
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return fmt.Errorf("체크섬 요청 생성 실패: %w", err)
	}
	resp, err := httpC.Do(req)
	if err != nil {
		return fmt.Errorf("체크섬 다운로드 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("체크섬 다운로드 실패 (HTTP %d)", resp.StatusCode)
	}

	// Parse checksums.txt: "<sha256>  <filename>\n"
	var expectedHash string
	scanner := bufio.NewScanner(io.LimitReader(resp.Body, 64*1024))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			expectedHash = parts[0]
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("체크섬 파일 읽기 실패: %w", err)
	}
	if expectedHash == "" {
		return fmt.Errorf("checksums.txt에서 %s의 해시를 찾을 수 없습니다", assetName)
	}

	// Validate hash format (must be 64 hex characters for SHA256)
	if len(expectedHash) != 64 {
		return fmt.Errorf("checksums.txt의 해시 길이가 잘못되었습니다: %d (SHA256은 64자)", len(expectedHash))
	}
	if _, err := hex.DecodeString(expectedHash); err != nil {
		return fmt.Errorf("checksums.txt의 해시 형식이 잘못되었습니다: %w", err)
	}

	// Compute SHA256 of downloaded archive
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("아카이브 파일 열기 실패: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("해시 계산 실패: %w", err)
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("체크섬 불일치: expected %s, got %s (파일이 변조되었을 수 있음)", expectedHash, actualHash)
	}

	return nil
}

func downloadFile(reqCtx context.Context, client *http.Client, url, destPath string) error {
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}

	lr := io.LimitReader(resp.Body, maxExtractSize+1)
	n, err := io.Copy(f, lr)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if n > maxExtractSize {
		_ = os.Remove(destPath)
		return fmt.Errorf("다운로드 크기가 한계(%d bytes)를 초과합니다", maxExtractSize)
	}
	return nil
}

func extractBinary(archivePath, destDir, binaryName, archiveType string) (string, error) {
	if archiveType == "zip" {
		return extractFromZip(archivePath, destDir, binaryName)
	}
	return extractFromTarGz(archivePath, destDir, binaryName)
}

func extractFromTarGz(archivePath, destDir, binaryName string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("gzip 디코딩 실패: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar 읽기 실패: %w", err)
		}

		// Security: reject path traversal
		if strings.Contains(hdr.Name, "..") {
			continue
		}

		base := filepath.Base(hdr.Name)
		if base != binaryName {
			continue
		}

		destPath := filepath.Join(destDir, binaryName)
		outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			return "", err
		}

		n, err := io.Copy(outFile, io.LimitReader(tr, maxExtractSize+1))
		_ = outFile.Close()
		if err != nil {
			return "", err
		}
		if n > maxExtractSize {
			return "", fmt.Errorf("추출된 바이너리 크기가 한계(%d bytes)를 초과합니다", maxExtractSize)
		}

		return destPath, nil
	}

	return "", fmt.Errorf("아카이브에서 %s를 찾을 수 없습니다", binaryName)
}

func extractFromZip(archivePath, destDir, binaryName string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("zip 열기 실패: %w", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		// Security: reject path traversal
		if strings.Contains(f.Name, "..") {
			continue
		}

		base := filepath.Base(f.Name)
		if base != binaryName {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", err
		}

		destPath := filepath.Join(destDir, binaryName)
		outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			_ = rc.Close()
			return "", err
		}

		n, err := io.Copy(outFile, io.LimitReader(rc, maxExtractSize+1))
		_ = outFile.Close()
		_ = rc.Close()
		if err != nil {
			return "", err
		}
		if n > maxExtractSize {
			return "", fmt.Errorf("추출된 바이너리 크기가 한계(%d bytes)를 초과합니다", maxExtractSize)
		}

		return destPath, nil
	}

	return "", fmt.Errorf("아카이브에서 %s를 찾을 수 없습니다", binaryName)
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	outFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	_, err = io.Copy(outFile, in)
	if closeErr := outFile.Close(); err == nil {
		err = closeErr
	}
	return err
}
