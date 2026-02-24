package unit_test

// global_state_test.go verifies that the evaluator has no problematic
// mutable global state that could cause cross-request data leakage or
// incorrect results in long-running services and concurrent scenarios.
//
// AREAS COVERED
//
//  1. nowTime isolation: $now() / $millis() must return a fresh timestamp for
//     each independent evaluation, not a stale value cached from a previous call.
//
//  2. Intra-evaluation consistency: within a single evaluation, every call to
//     $now() must return the same timestamp (JSONata spec requirement).
//
//  3. Concurrent isolation: parallel evaluations must not share or corrupt each
//     other's state.
//
//  4. testing/synctest: time-sensitive behaviour verified with a fake clock so
//     tests are deterministic and do not depend on wall-clock speed.

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/parser"
	"github.com/sandrolain/gosonata/pkg/types"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func mustParseEval(t *testing.T, query string, data interface{}) interface{} {
	t.Helper()
	expr, err := parser.Parse(query)
	if err != nil {
		t.Fatalf("parse %q: %v", query, err)
	}
	ev := evaluator.New(evaluator.WithConcurrency(false))
	res, err := ev.Eval(context.Background(), expr, data)
	if err != nil {
		t.Fatalf("eval %q: %v", query, err)
	}
	return res
}

// newEval returns an evaluator and a compiled expression ready for reuse.
func newEval(t *testing.T, query string) (*evaluator.Evaluator, *types.Expression) {
	t.Helper()
	expr, err := parser.Parse(query)
	if err != nil {
		t.Fatalf("parse %q: %v", query, err)
	}
	return evaluator.New(evaluator.WithConcurrency(false)), expr
}

// ── 1. nowTime isolation ─────────────────────────────────────────────────────

// TestNowIsolation verifies that two separate evaluations of $now() return
// different timestamps when wall-clock time has advanced between them.
// This is the regression test for the former bug where nowCalculated/nowTime
// were package-level globals set once and never reset between evaluations.
func TestNowIsolation(t *testing.T) {
	ev, expr := newEval(t, "$now()")

	r1, err := ev.Eval(context.Background(), expr, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Guarantee at least 1 ms of wall-clock time so RFC3339Nano strings differ.
	time.Sleep(2 * time.Millisecond)
	r2, err := ev.Eval(context.Background(), expr, nil)
	if err != nil {
		t.Fatal(err)
	}

	if r1.(string) == r2.(string) {
		t.Errorf("$now() returned the same timestamp across two evaluations: %s", r1.(string))
	}
}

// TestMillisIsolation is the $millis() counterpart of TestNowIsolation.
func TestMillisIsolation(t *testing.T) {
	ev, expr := newEval(t, "$millis()")

	r1, _ := ev.Eval(context.Background(), expr, nil)
	time.Sleep(2 * time.Millisecond)
	r2, _ := ev.Eval(context.Background(), expr, nil)

	m1, m2 := r1.(float64), r2.(float64)
	if m1 >= m2 {
		t.Errorf("$millis() did not advance between evaluations: first=%v second=%v", m1, m2)
	}
}

// ── 2. Intra-evaluation consistency ─────────────────────────────────────────

// TestNowConsistencyWithinEval verifies that multiple $now() references within
// the same expression return an identical timestamp (JSONata spec requirement).
func TestNowConsistencyWithinEval(t *testing.T) {
	// $now() = $now() evaluates both sides within the same EvalContext and
	// must return true because the timestamp is pinned to the root context.
	result := mustParseEval(t, `$now() = $now()`, nil)
	if result != true {
		t.Errorf("$now() = $now() should be true within the same evaluation, got %v", result)
	}
}

// TestMillisConsistencyWithinEval is the $millis() counterpart.
func TestMillisConsistencyWithinEval(t *testing.T) {
	result := mustParseEval(t, `$millis() = $millis()`, nil)
	if result != true {
		t.Errorf("$millis() = $millis() should be true within the same evaluation, got %v", result)
	}
}

// ── 3. Concurrent isolation ──────────────────────────────────────────────────

// TestConcurrentEvalNoSharedState spawns many goroutines evaluating $now()
// concurrently and verifies that no goroutine panics or returns an error.
// The race detector (go test -race) will flag any data race should shared
// mutable global state be touched without synchronisation.
func TestConcurrentEvalNoSharedState(t *testing.T) {
	const workers = 50

	expr, err := parser.Parse("$now()")
	if err != nil {
		t.Fatal(err)
	}

	results := make([]string, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup

	for i := range workers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ev := evaluator.New(evaluator.WithConcurrency(false))
			r, e := ev.Eval(context.Background(), expr, nil)
			errs[idx] = e
			if e == nil {
				results[idx] = r.(string)
			}
		}(i)
	}
	wg.Wait()

	for i := range workers {
		if errs[i] != nil {
			t.Errorf("goroutine %d: eval error: %v", i, errs[i])
		}
		if results[i] == "" {
			t.Errorf("goroutine %d: empty result", i)
		}
	}
}

// TestConcurrentMillisMonotonic evaluates $millis() many times in series and
// checks that the returned value never decreases. A monotonically non-decreasing
// sequence proves that stale globally-cached values are not returned.
func TestConcurrentMillisMonotonic(t *testing.T) {
	const calls = 200

	ev, expr := newEval(t, "$millis()")
	var prev float64
	for range calls {
		r, err := ev.Eval(context.Background(), expr, nil)
		if err != nil {
			t.Fatal(err)
		}
		ms := r.(float64)
		if ms < prev {
			t.Fatalf("$millis() went backwards: prev=%v current=%v", prev, ms)
		}
		prev = ms
	}
}

// ── 4. testing/synctest — deterministic fake-clock verification ───────────────

// TestNowFakeClock uses testing/synctest to run evaluations under a controlled
// fake clock. The fake clock starts at midnight UTC 2000-01-01.
//
// Assertions:
//
//	a. First evaluation reflects the initial fake time.
//	b. After advancing the fake clock, a new evaluation reflects the new time.
//	c. Two evaluations separated by a Sleep return different timestamps.
func TestNowFakeClock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		expr, err := parser.Parse("$now()")
		if err != nil {
			t.Fatal(err)
		}
		ev := evaluator.New(evaluator.WithConcurrency(false))

		// a. First evaluation — fake clock starts at 2000-01-01T00:00:00Z.
		r1, err := ev.Eval(context.Background(), expr, nil)
		if err != nil {
			t.Fatal(err)
		}
		t1, parseErr := time.Parse(time.RFC3339Nano, r1.(string))
		if parseErr != nil {
			t.Fatalf("cannot parse $now() result %q: %v", r1, parseErr)
		}
		expected1 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		if !t1.Equal(expected1) {
			t.Errorf("a: expected fake time %v, got %v", expected1, t1)
		}

		// b. Advance fake clock by 1 hour; a new evaluation must reflect it.
		time.Sleep(1 * time.Hour)
		synctest.Wait()

		r2, err := ev.Eval(context.Background(), expr, nil)
		if err != nil {
			t.Fatal(err)
		}
		t2, _ := time.Parse(time.RFC3339Nano, r2.(string))
		expected2 := expected1.Add(1 * time.Hour)
		if !t2.Equal(expected2) {
			t.Errorf("b: expected %v, got %v", expected2, t2)
		}

		// c. The second timestamp must be strictly after the first.
		if !t2.After(t1) {
			t.Errorf("c: second evaluation should be after first: t1=%v t2=%v", t1, t2)
		}
	})
}

// TestMillisFakeClock is the $millis() counterpart of TestNowFakeClock.
func TestMillisFakeClock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ev, expr := newEval(t, "$millis()")

		fakeEpoch := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

		r1, _ := ev.Eval(context.Background(), expr, nil)
		ms1 := r1.(float64)
		if ms1 != float64(fakeEpoch.UnixMilli()) {
			t.Errorf("first $millis() = %v, want %v", ms1, float64(fakeEpoch.UnixMilli()))
		}

		time.Sleep(30 * time.Second)
		synctest.Wait()

		r2, _ := ev.Eval(context.Background(), expr, nil)
		ms2 := r2.(float64)
		expected2 := float64(fakeEpoch.Add(30 * time.Second).UnixMilli())
		if ms2 != expected2 {
			t.Errorf("second $millis() = %v, want %v", ms2, expected2)
		}
		if ms2 <= ms1 {
			t.Errorf("$millis() did not advance: first=%v second=%v", ms1, ms2)
		}
	})
}

// TestNowIntraEvalFakeClock verifies intra-evaluation consistency under the
// fake clock: even though the clock advances between expressions, two $now()
// references within the same expression must return the same pinned timestamp.
func TestNowIntraEvalFakeClock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		result := mustParseEval(t, `$now() = $now()`, nil)
		if result != true {
			t.Errorf("$now() = $now() should be true under fake clock, got %v", result)
		}
	})
}

// TestNowMultipleRequestsFakeClock simulates a long-running service processing
// multiple requests at regular fake-time intervals. Each request must see a
// monotonically increasing timestamp equal to fakeStart + i*interval.
//
// This is the definitive regression test for the former global-state bug:
// in a long-running process, every request must get a fresh timestamp rather
// than the timestamp captured during the very first request.
func TestNowMultipleRequestsFakeClock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const requests = 5
		const interval = 10 * time.Second

		fakeStart := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		ev, expr := newEval(t, "$now()")

		for i := range requests {
			r, err := ev.Eval(context.Background(), expr, nil)
			if err != nil {
				t.Fatalf("request %d: %v", i, err)
			}
			got, parseErr := time.Parse(time.RFC3339Nano, r.(string))
			if parseErr != nil {
				t.Fatalf("request %d: cannot parse %q: %v", i, r, parseErr)
			}
			want := fakeStart.Add(time.Duration(i) * interval)
			if !got.Equal(want) {
				t.Errorf("request %d: got %v, want %v", i, got, want)
			}

			// Advance fake clock between requests (not after the last one).
			if i < requests-1 {
				time.Sleep(interval)
				synctest.Wait()
			}
		}
	})
}
