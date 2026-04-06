package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync/atomic"
)

// Logger provides leveled diagnostic logging to stderr.
// Output is suppressed when disabled, except for Error which always prints.
// The enabled flag uses atomic.Bool for lock-free reads on the hot path;
// log.Logger provides its own internal synchronization for writes.
type Logger struct {
	enabled atomic.Bool
	inner   *log.Logger
}

var std = &Logger{
	inner: log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds),
}

// Init sets whether diagnostic logging is enabled.
func Init(debug bool) {
	std.enabled.Store(debug)
}

// SetOutput sets the output writer (for testing).
func SetOutput(w io.Writer) {
	std.inner.SetOutput(w)
}

// IsEnabled returns whether debug logging is active.
func IsEnabled() bool {
	return std.enabled.Load()
}

// Debug logs a message at DEBUG level (only when enabled).
func Debug(format string, args ...any) {
	if !std.enabled.Load() {
		return
	}
	_ = std.inner.Output(2, fmt.Sprintf("[DEBUG] "+format, args...))
}

// Info logs a message at INFO level (only when enabled).
func Info(format string, args ...any) {
	if !std.enabled.Load() {
		return
	}
	_ = std.inner.Output(2, fmt.Sprintf("[INFO] "+format, args...))
}

// Warn logs a message at WARN level (only when enabled).
func Warn(format string, args ...any) {
	if !std.enabled.Load() {
		return
	}
	_ = std.inner.Output(2, fmt.Sprintf("[WARN] "+format, args...))
}

// Error logs a message at ERROR level (always prints, regardless of enabled state).
func Error(format string, args ...any) {
	_ = std.inner.Output(2, fmt.Sprintf("[ERROR] "+format, args...))
}
