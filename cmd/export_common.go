package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/solapi/solactl/pkg/progress"
)

// resolveProgressMode maps --progress / --no-progress flags to progress.Mode.
func resolveProgressMode(flag string, noProgress bool) (progress.Mode, error) {
	if noProgress {
		flag = "off"
	}
	switch flag {
	case "auto":
		return progress.ModeAuto, nil
	case "on":
		return progress.ModeOn, nil
	case "off":
		return progress.ModeOff, nil
	}
	return 0, fmt.Errorf("잘못된 --progress 값: %s (auto|on|off)", flag)
}

// parseExportDate accepts "2006-01-02", "2006-01-02T15:04:05Z", or RFC3339.
func parseExportDate(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("지원되는 날짜 형식: 2006-01-02, 2006-01-02T15:04:05Z, RFC3339")
}

// nopWriteCloser wraps stdout-like writers that must not be closed by the caller.
type nopWriteCloser struct{}

func (nopWriteCloser) Close() error { return nil }

// openExportOutput resolves the --output flag to (writer, closer). path == "-"
// returns the global stdout writer plus a no-op closer. append=false rejects an
// existing file via O_EXCL so users do not silently overwrite previous exports.
func openExportOutput(path string, appendMode bool) (io.Writer, io.Closer, error) {
	if path == "-" {
		return out(), nopWriteCloser{}, nil
	}
	if appendMode {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nil, err
		}
		return f, f, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, nil, fmt.Errorf("출력 파일이 이미 존재: %s (--append를 사용하거나 파일을 삭제하세요)", path)
		}
		return nil, nil, err
	}
	return f, f, nil
}
