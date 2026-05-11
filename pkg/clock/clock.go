// Package clock abstracts wall-clock access so throttling and retry logic
// can be exercised deterministically in tests.
package clock

import (
	"context"
	"sync"
	"time"
)

// Clock abstracts time so exporter throttling can be tested deterministically.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// Sleep blocks for d or until ctx is done. Returns ctx.Err() if canceled,
	// nil if the full duration elapsed. d <= 0 returns immediately (nil).
	Sleep(ctx context.Context, d time.Duration) error
}

// realClock is the stdlib-backed Clock implementation.
type realClock struct{}

// Real returns a Clock backed by time.Now and time.NewTimer.
func Real() Clock {
	return realClock{}
}

// Now returns the current wall-clock time.
func (realClock) Now() time.Time {
	return time.Now()
}

// Sleep blocks for d or until ctx is done.
func (realClock) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	// 이미 취소된 컨텍스트는 타이머 생성을 건너뛰고 즉시 반환한다.
	if err := ctx.Err(); err != nil {
		return err
	}
	t := time.NewTimer(d)
	select {
	case <-ctx.Done():
		t.Stop()
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Fake is a controllable clock for tests.
// Fake methods are safe for concurrent use.
type Fake struct {
	mu     sync.Mutex
	now    time.Time
	sleeps []time.Duration
}

// NewFake creates a Fake clock at the given instant.
func NewFake(now time.Time) *Fake {
	return &Fake{now: now}
}

// Now returns the current fake time.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Sleep records the duration in Sleeps() and advances the clock by d.
// If ctx is already canceled, returns ctx.Err() without recording or advancing.
// d <= 0 returns immediately, no record.
func (f *Fake) Sleep(ctx context.Context, d time.Duration) error {
	// 컨텍스트 취소를 먼저 검사하여 d<=0과의 우선순위를 분명히 한다.
	if err := ctx.Err(); err != nil {
		return err
	}
	if d <= 0 {
		return nil
	}
	f.mu.Lock()
	f.sleeps = append(f.sleeps, d)
	f.now = f.now.Add(d)
	f.mu.Unlock()
	return nil
}

// Sleeps returns a copy of the recorded sleep durations in call order.
func (f *Fake) Sleeps() []time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]time.Duration, len(f.sleeps))
	copy(out, f.sleeps)
	return out
}

// Advance moves the fake clock forward without recording a sleep.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	f.mu.Unlock()
}
