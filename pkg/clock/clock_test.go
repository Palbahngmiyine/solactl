package clock

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestReal_Sleep_ZeroOrNegative(t *testing.T) {
	t.Parallel()
	c := Real()
	cases := []struct {
		name string
		d    time.Duration
	}{
		{"zero", 0},
		{"negative", -1 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			start := time.Now()
			if err := c.Sleep(context.Background(), tc.d); err != nil {
				t.Fatalf("Sleep returned err: %v", err)
			}
			if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
				t.Fatalf("Sleep with d=%v took too long: %v", tc.d, elapsed)
			}
		})
	}
}

func TestReal_Sleep_Completes(t *testing.T) {
	t.Parallel()
	c := Real()
	const target = 10 * time.Millisecond
	start := time.Now()
	if err := c.Sleep(context.Background(), target); err != nil {
		t.Fatalf("Sleep returned err: %v", err)
	}
	elapsed := time.Since(start)
	// flake 방지를 위해 넓은 범위(5~100ms)를 허용한다.
	if elapsed < 5*time.Millisecond || elapsed > 100*time.Millisecond {
		t.Fatalf("Sleep elapsed=%v outside [5ms, 100ms]", elapsed)
	}
}

func TestReal_Sleep_CtxCancel_Immediate(t *testing.T) {
	t.Parallel()
	c := Real()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	err := c.Sleep(ctx, 1*time.Second)
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("immediate-cancel Sleep too slow: %v", elapsed)
	}
}

func TestReal_Sleep_CtxCancel_DuringSleep(t *testing.T) {
	t.Parallel()
	c := Real()
	ctx, cancel := context.WithCancel(context.Background())
	const cancelAfter = 50 * time.Millisecond
	go func() {
		time.Sleep(cancelAfter)
		cancel()
	}()
	start := time.Now()
	err := c.Sleep(ctx, 1*time.Second)
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// 취소 시점(50ms) 근처에서 반환되어야 한다. 넉넉히 5~500ms 허용.
	if elapsed < 5*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Fatalf("cancel-during Sleep elapsed=%v outside [5ms, 500ms]", elapsed)
	}
}

func TestFake_Now(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFake(t0)
	if got := f.Now(); !got.Equal(t0) {
		t.Fatalf("Now()=%v, want %v", got, t0)
	}
	f.Advance(1 * time.Second)
	if got, want := f.Now(), t0.Add(1*time.Second); !got.Equal(want) {
		t.Fatalf("Now() after Advance=%v, want %v", got, want)
	}
}

func TestFake_Sleep_ZeroOrNegative(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		d    time.Duration
	}{
		{"zero", 0},
		{"negative", -1 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := NewFake(t0)
			if err := f.Sleep(context.Background(), tc.d); err != nil {
				t.Fatalf("Sleep returned err: %v", err)
			}
			if got := f.Sleeps(); len(got) != 0 {
				t.Fatalf("Sleeps() should be empty, got %v", got)
			}
			if got := f.Now(); !got.Equal(t0) {
				t.Fatalf("Now() must not advance, got %v want %v", got, t0)
			}
		})
	}
}

func TestFake_Sleep_Records(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFake(t0)
	const d = 500 * time.Millisecond
	for range 3 {
		if err := f.Sleep(context.Background(), d); err != nil {
			t.Fatalf("Sleep returned err: %v", err)
		}
	}
	got := f.Sleeps()
	want := []time.Duration{d, d, d}
	if len(got) != len(want) {
		t.Fatalf("Sleeps len=%d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("Sleeps[%d]=%v, want %v", i, got[i], want[i])
		}
	}
	if got, want := f.Now(), t0.Add(1500*time.Millisecond); !got.Equal(want) {
		t.Fatalf("Now()=%v, want %v", got, want)
	}
}

func TestFake_Sleep_CtxCancel(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFake(t0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Sleep(ctx, 1*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if got := f.Sleeps(); len(got) != 0 {
		t.Fatalf("Sleeps() must be empty, got %v", got)
	}
	if got := f.Now(); !got.Equal(t0) {
		t.Fatalf("Now() must not advance, got %v want %v", got, t0)
	}
}

func TestFake_Advance(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFake(t0)
	f.Advance(2 * time.Second)
	if got, want := f.Now(), t0.Add(2*time.Second); !got.Equal(want) {
		t.Fatalf("Now()=%v, want %v", got, want)
	}
	if got := f.Sleeps(); len(got) != 0 {
		t.Fatalf("Advance must not record sleep, got %v", got)
	}
}

func TestFake_Concurrent(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFake(t0)
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n * 3)
	for range n {
		go func() {
			defer wg.Done()
			_ = f.Sleep(context.Background(), 1*time.Millisecond)
		}()
		go func() {
			defer wg.Done()
			_ = f.Now()
		}()
		go func() {
			defer wg.Done()
			f.Advance(1 * time.Millisecond)
		}()
	}
	wg.Wait()
	// Sleep 호출 횟수만 카운트한다.
	if got := len(f.Sleeps()); got != n {
		t.Fatalf("Sleeps len=%d, want %d", got, n)
	}
}

func TestFake_SleepsCopy(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFake(t0)
	if err := f.Sleep(context.Background(), 1*time.Second); err != nil {
		t.Fatalf("Sleep returned err: %v", err)
	}
	got := f.Sleeps()
	if len(got) != 1 {
		t.Fatalf("initial Sleeps len=%d, want 1", len(got))
	}
	// 반환된 슬라이스를 변조해도 내부 상태에 영향이 없어야 한다.
	got[0] = 999 * time.Hour
	got = append(got, 7*time.Hour)
	again := f.Sleeps()
	if len(again) != 1 {
		t.Fatalf("after mutation Sleeps len=%d, want 1", len(again))
	}
	if again[0] != 1*time.Second {
		t.Fatalf("after mutation Sleeps[0]=%v, want 1s", again[0])
	}
}

// Clock 인터페이스 만족 여부를 컴파일 타임에 검증한다.
var _ Clock = realClock{}
var _ Clock = (*Fake)(nil)
