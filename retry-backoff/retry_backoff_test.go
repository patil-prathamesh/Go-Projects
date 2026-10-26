package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"
)

var errTemp = errors.New("temporary failure")

func fastOpts(attempts int) Options {
	return Options{MaxAttempts: attempts, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
}

func TestRetrySucceedsOnThirdCall(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), fastOpts(5), func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return errTemp
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d, want nil and 3", err, calls)
	}
}

func TestRetryExhaustsAttempts(t *testing.T) {
	calls := 0
	opts := Options{MaxAttempts: 2, BaseDelay: 50 * time.Millisecond, MaxDelay: 50 * time.Millisecond}

	start := time.Now()
	err := Retry(context.Background(), opts, func(ctx context.Context) error {
		calls++
		return errTemp
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error after exhausting attempts, got nil")
	}
	if calls != 2 {
		t.Errorf("calls=%d, want 2", calls)
	}
	if !errors.Is(err, errTemp) {
		t.Errorf("errors.Is(err, errTemp) = false; err=%v", err)
	}
	// exactly one wait (between attempt 1 and 2), none after the last attempt
	if elapsed < 45*time.Millisecond || elapsed > 90*time.Millisecond {
		t.Errorf("elapsed=%v, want about 50ms (one wait only)", elapsed)
	}
}

func TestRetryAlreadyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	err := Retry(ctx, fastOpts(3), func(ctx context.Context) error {
		calls++
		return errTemp
	})

	if calls != 0 {
		t.Errorf("calls=%d, want 0 (fn must not run on a dead context)", calls)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err=%v, want it to match context.Canceled", err)
	}
}

func TestRetryCancelDuringWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	calls := 0
	opts := Options{MaxAttempts: 5, BaseDelay: 200 * time.Millisecond, MaxDelay: time.Second}

	start := time.Now()
	err := Retry(ctx, opts, func(ctx context.Context) error {
		calls++
		return errTemp
	})
	elapsed := time.Since(start)

	if calls != 1 {
		t.Errorf("calls=%d, want 1 (no extra call after the wait is interrupted)", calls)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("elapsed=%v, want about 20ms (wait must be interrupted)", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err=%v, want it to match context.DeadlineExceeded", err)
	}
}

func TestRetryMaxAttemptsZero(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), fastOpts(0), func(ctx context.Context) error {
		calls++
		return errTemp
	})
	// whichever rule you chose is fine, but success without a single call is a lie
	if err == nil && calls == 0 {
		t.Fatal("Retry returned nil without calling fn once")
	}
}

func TestRetryPermanentStopsImmediately(t *testing.T) {
	calls := 0
	opts := Options{MaxAttempts: 5, BaseDelay: 200 * time.Millisecond, MaxDelay: time.Second}

	start := time.Now()
	err := Retry(context.Background(), opts, func(ctx context.Context) error {
		calls++
		return fmt.Errorf("fetching user: %w", Permanent(io.EOF)) // wrapped on purpose
	})
	elapsed := time.Since(start)

	if calls != 1 {
		t.Errorf("calls=%d, want 1", calls)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("elapsed=%v, want no wait at all", elapsed)
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("errors.Is(err, io.EOF) = false; err=%v (check Unwrap)", err)
	}
}

func TestPermanentNil(t *testing.T) {
	if got := Permanent(nil); got != nil {
		t.Errorf("Permanent(nil) = %v, want nil", got)
	}
}

func TestSleepFullDuration(t *testing.T) {
	start := time.Now()
	err := sleep(context.Background(), 50*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("err=%v, want nil", err)
	}
	if elapsed < 45*time.Millisecond {
		t.Errorf("elapsed=%v, returned too early", elapsed)
	}
}

func TestSleepInterrupted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := sleep(ctx, time.Second)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err=%v, want DeadlineExceeded", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("elapsed=%v, want about 20ms, not the full second", elapsed)
	}
}

// Enable once Retry keeps the last error from fn when the context ends.
func TestRetryKeepsLastErrorOnCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	opts := Options{MaxAttempts: 5, BaseDelay: 200 * time.Millisecond, MaxDelay: time.Second}
	err := Retry(ctx, opts, func(ctx context.Context) error { return errTemp })

	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, errTemp) {
		t.Errorf("err=%v, want it to match both DeadlineExceeded and errTemp", err)
	}
}

func TestJitterBounds(t *testing.T) {
	d := 100 * time.Millisecond
	seen := map[time.Duration]bool{}

	for i := 0; i < 10000; i++ {
		got := jitter(d)
		if got < 0 || got >= d {
			t.Fatalf("jitter(%v) = %v, want in [0, %v)", d, got, d)
		}
		seen[got] = true
	}
	if len(seen) < 100 {
		t.Errorf("only %d distinct values in 10000 calls, jitter looks non-random", len(seen))
	}
}

func TestJitterNonPositive(t *testing.T) {
	if got := jitter(0); got != 0 {
		t.Errorf("jitter(0) = %v, want 0", got)
	}
	if got := jitter(-time.Second); got != 0 {
		t.Errorf("jitter(-1s) = %v, want 0", got)
	}
}

func TestRetryWithJitterStillWorks(t *testing.T) {
	calls := 0
	opts := Options{MaxAttempts: 4, BaseDelay: 5 * time.Millisecond, MaxDelay: 20 * time.Millisecond, Jitter: true}

	err := Retry(context.Background(), opts, func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return errTemp
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d, want nil and 3", err, calls)
	}
}
