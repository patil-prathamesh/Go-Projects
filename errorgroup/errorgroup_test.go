package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// runTasks runs `tasks` tasks of 200ms each. limit <= 0 means SetLimit is never called.
func runTasks(t *testing.T, limit, tasks int) (peak int64, elapsed time.Duration) {
	t.Helper()

	var running, maxSeen atomic.Int64

	grp, _ := NewGroup(context.Background())
	if limit > 0 {
		grp.SetLimit(limit)
	}

	start := time.Now()
	for i := 0; i < tasks; i++ {
		grp.Go(func(ctx context.Context) error {
			cur := running.Add(1)
			defer running.Add(-1)

			// raise maxSeen to cur, safely, if cur is larger
			for {
				old := maxSeen.Load()
				if cur <= old || maxSeen.CompareAndSwap(old, cur) {
					break
				}
			}

			time.Sleep(200 * time.Millisecond)
			return nil
		})
	}

	if err := grp.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return maxSeen.Load(), time.Since(start)
}

func TestSetLimit(t *testing.T) {
	peak, elapsed := runTasks(t, 2, 6)
	t.Logf("limit=2: peak=%d elapsed=%v", peak, elapsed)

	if peak > 2 {
		t.Errorf("peak concurrency %d exceeds limit 2", peak)
	}
	if elapsed < 550*time.Millisecond {
		t.Errorf("finished in %v, expected about 600ms (3 rounds of 2)", elapsed)
	}
}

func TestNoLimit(t *testing.T) {
	peak, elapsed := runTasks(t, 0, 6)
	t.Logf("no limit: peak=%d elapsed=%v", peak, elapsed)

	if peak != 6 {
		t.Errorf("peak concurrency %d, expected 6", peak)
	}
	if elapsed > 450*time.Millisecond {
		t.Errorf("took %v, expected about 200ms", elapsed)
	}
}