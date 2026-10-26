package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"
)

// jitter returns a random duration in [0, d) ("full jitter").
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0 // Int64N panics on a non-positive argument
	}
	return time.Duration(rand.Int64N(int64(d)))
}

type Options struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      bool
}

type permanentError struct {
	err error
}

func (p *permanentError) Error() string {
	return p.err.Error()
}

func (p *permanentError) Unwrap() error {
	return p.err
}

func Permanent(err error) error {
	if err == nil {
		return nil // nothing to wrap: keep "no error" as no error
	}
	return &permanentError{err: err} // keep the receiver form you chose
}

func Retry(ctx context.Context, opts Options, fn func(ctx context.Context) error) error {
	if ctx.Err() != nil {
		return ctx.Err() // never call fn on a dead context
	}
	if opts.MaxAttempts <= 0 { // rule: MaxAttempts must be positive
		return errors.New("MaxAttempts must be positive")
	}

	var lastErr error
	for i := 0; i < opts.MaxAttempts; i++ {
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		var pErr *permanentError
		if errors.As(lastErr, &pErr) {
			return pErr.err // permanent: stop now, no wait
		}

		if ctx.Err() != nil { // caller gave up while fn was running
			return stopped(i+1, ctx.Err(), lastErr)
		}

		if i == opts.MaxAttempts-1 {
			break // last attempt: no point waiting
		}

		wait := delay(i, opts.BaseDelay, opts.MaxDelay)
		if opts.Jitter {
			wait = jitter(wait)
		}
		if err := sleep(ctx, wait); err != nil {
			return stopped(i+1, err, lastErr)
		}
	}

	return fmt.Errorf("gave up after %d attempts: %w", opts.MaxAttempts, lastErr)
}

func stopped(attempts int, ctxErr, last error) error {
	return fmt.Errorf("stopped after %d attempts: %w (last error: %w)", attempts, ctxErr, last)
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func delay(attempt int, base, max time.Duration) time.Duration {
	d := base
	for i := 0; i < attempt; i++ {
		if d >= max/2 { // doubling would reach or pass max, so stop here
			return max
		}
		d *= 2
	}
	return d
}
