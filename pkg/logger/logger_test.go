package logger

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func setup(t *testing.T, enabled bool) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	SetOutput(&buf)
	Init(enabled)
	t.Cleanup(func() {
		Init(false)
	})
	return &buf
}

func TestDebug_Enabled(t *testing.T) {
	buf := setup(t, true)
	Debug("hello %s", "world")
	got := buf.String()
	if !strings.Contains(got, "[DEBUG] hello world") {
		t.Errorf("expected [DEBUG] hello world, got: %q", got)
	}
}

func TestDebug_Disabled(t *testing.T) {
	buf := setup(t, false)
	Debug("should not appear")
	if buf.Len() != 0 {
		t.Errorf("expected no output when disabled, got: %q", buf.String())
	}
}

func TestInfo_Enabled(t *testing.T) {
	buf := setup(t, true)
	Info("count=%d", 42)
	got := buf.String()
	if !strings.Contains(got, "[INFO] count=42") {
		t.Errorf("expected [INFO] count=42, got: %q", got)
	}
}

func TestInfo_Disabled(t *testing.T) {
	buf := setup(t, false)
	Info("should not appear")
	if buf.Len() != 0 {
		t.Errorf("expected no output when disabled, got: %q", buf.String())
	}
}

func TestWarn_Enabled(t *testing.T) {
	buf := setup(t, true)
	Warn("caution: %s", "hot")
	got := buf.String()
	if !strings.Contains(got, "[WARN] caution: hot") {
		t.Errorf("expected [WARN] caution: hot, got: %q", got)
	}
}

func TestWarn_Disabled(t *testing.T) {
	buf := setup(t, false)
	Warn("should not appear")
	if buf.Len() != 0 {
		t.Errorf("expected no output when disabled, got: %q", buf.String())
	}
}

func TestError_AlwaysPrints_WhenEnabled(t *testing.T) {
	buf := setup(t, true)
	Error("fatal: %v", "crash")
	got := buf.String()
	if !strings.Contains(got, "[ERROR] fatal: crash") {
		t.Errorf("expected [ERROR] fatal: crash, got: %q", got)
	}
}

func TestError_AlwaysPrints_WhenDisabled(t *testing.T) {
	buf := setup(t, false)
	Error("fatal: %v", "crash")
	got := buf.String()
	if !strings.Contains(got, "[ERROR] fatal: crash") {
		t.Errorf("expected [ERROR] fatal: crash even when disabled, got: %q", got)
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name  string
		debug bool
		want  bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(tt.debug)
			t.Cleanup(func() { Init(false) })
			if got := IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInit_TogglesOutput(t *testing.T) {
	buf := setup(t, false)

	Debug("before enable")
	if buf.Len() != 0 {
		t.Fatal("expected no output before enable")
	}

	Init(true)
	Debug("after enable")
	if !strings.Contains(buf.String(), "[DEBUG] after enable") {
		t.Errorf("expected output after enable, got: %q", buf.String())
	}

	buf.Reset()
	Init(false)
	Debug("after disable")
	if buf.Len() != 0 {
		t.Errorf("expected no output after disable, got: %q", buf.String())
	}
}

func TestOutputContainsTimestamp(t *testing.T) {
	buf := setup(t, true)
	Debug("timestamped")
	got := buf.String()
	// log.Ldate|log.Ltime produces "YYYY/MM/DD HH:MM:SS" prefix
	if !strings.Contains(got, "/") || !strings.Contains(got, ":") {
		t.Errorf("expected timestamp in output, got: %q", got)
	}
}

func TestConcurrent_InitAndLog(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	SetOutput(&buf)
	t.Cleanup(func() { Init(false) })

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		if i%2 == 0 {
			go func() {
				defer wg.Done()
				// Toggle Init on and off
				Init(true)
				Init(false)
			}()
		} else {
			go func() {
				defer wg.Done()
				Debug("concurrent debug %d", 1)
				Info("concurrent info %d", 2)
				Warn("concurrent warn %d", 3)
				Error("concurrent error %d", 4)
			}()
		}
	}

	wg.Wait()
	// If we reach here without panic or race detector failure, the test passes.
}

func TestRapidToggle(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	t.Cleanup(func() { Init(false) })

	for i := 0; i < 1000; i++ {
		if i%2 == 0 {
			Init(true)
		} else {
			Init(false)
		}
		Debug("toggle iteration %d", i)
	}
	// If we reach here without panic, the test passes.
}

func TestSetOutput_NilWriter(t *testing.T) {
	t.Cleanup(func() {
		// Restore a valid writer so other tests don't break
		var buf bytes.Buffer
		SetOutput(&buf)
		Init(false)
	})

	Init(true)
	SetOutput(nil)

	// log.Logger.Output writes to the underlying writer.
	// With a nil writer this will panic inside log.Logger.
	// We document this: SetOutput(nil) causes a panic on write.
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		Error("this should panic with nil writer")
	}()

	if !panicked {
		t.Error("expected panic when logging with nil writer, but none occurred")
	}
}

func TestDebug_Format(t *testing.T) {
	buf := setup(t, true)
	Debug("value=%d name=%s", 42, "test")
	got := buf.String()

	if !strings.Contains(got, "[DEBUG]") {
		t.Errorf("output should contain [DEBUG] prefix, got: %q", got)
	}
	if !strings.Contains(got, "value=42 name=test") {
		t.Errorf("output should contain formatted message, got: %q", got)
	}
}

func TestConcurrent_MixedLevels(t *testing.T) {
	// Not t.Parallel(): this test needs exclusive control of the global logger
	// state to assert on output content. Running parallel with
	// TestConcurrent_InitAndLog would toggle enabled off mid-flight.
	buf := setup(t, true)

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			Debug("debug from goroutine %d", n)
			Info("info from goroutine %d", n)
			Warn("warn from goroutine %d", n)
			Error("error from goroutine %d", n)
		}(i)
	}

	wg.Wait()

	got := buf.String()
	// Verify that output contains all four level prefixes (at least once)
	for _, level := range []string{"[DEBUG]", "[INFO]", "[WARN]", "[ERROR]"} {
		if !strings.Contains(got, level) {
			t.Errorf("expected output to contain %s, but it did not", level)
		}
	}
}

// failWriter is an io.Writer that always returns an error.
type failWriter struct{}

func (failWriter) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("simulated write failure")
}

func TestLogOutput_WriteError(t *testing.T) {
	SetOutput(failWriter{})
	Init(true)
	t.Cleanup(func() {
		var buf bytes.Buffer
		SetOutput(&buf)
		Init(false)
	})

	// All of these silently discard the error via `_ = std.inner.Output(...)`.
	// None should panic.
	Debug("debug with broken writer")
	Info("info with broken writer")
	Warn("warn with broken writer")
	Error("error with broken writer")
}
