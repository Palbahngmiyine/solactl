package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// Test seams
var (
	upgradeHTTPClient     *http.Client
	executablePathFunc    = os.Executable
	githubBaseURL         = "https://api.github.com"
	maxExtractSize  int64 = 100 * 1024 * 1024 // 100MB
	copyFileFunc          = copyFile
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
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

	_, _ = fmt.Fprintf(w, "다운로드 중... %s\n", assetName)

	// 4. Download to temp directory
	tmpDir, err := os.MkdirTemp("", "solactl-upgrade-*")
	if err != nil {
		return fmt.Errorf("임시 디렉토리 생성 실패: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	archivePath := filepath.Join(tmpDir, assetName)
	if err := downloadFile(ctx(), httpC, assetURL, archivePath); err != nil {
		return fmt.Errorf("다운로드 실패: %w", err)
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
