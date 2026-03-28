package unit_test

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sandrolain/gosonata/pkg/stream"
	"github.com/sandrolain/gosonata/pkg/types"

	gosonata "github.com/sandrolain/gosonata"
)

// streamTestData is the shared JSON document for MultiEval tests.
var streamTestData = json.RawMessage(`{"Account":{"Name":"Firefly","Active":true},"order":{"total":250.0,"currency":"USD"},"items":[{"name":"widget","price":9.99},{"name":"gadget","price":49.99}],"score":42}`)

// compileStreamExpr compiles a JSONata expression for stream tests.
func compileStreamExpr(t testing.TB, query string) *types.Expression {
	t.Helper()
	expr, err := gosonata.Compile(query)
	if err != nil {
		t.Fatalf("compile %q: %v", query, err)
	}
	return expr
}

// ----------------------------------------------------------------------------
// DispatchOne — fast path and full path
// ----------------------------------------------------------------------------

func TestMultiEval_DispatchOne_FastPath(t *testing.T) {
	se := stream.NewMultiEval(nil)
	idx := se.Add(compileStreamExpr(t, "Account.Name"))

	got, err := se.DispatchOne(context.Background(), streamTestData, "s1", idx)
	if err != nil {
		t.Fatalf("DispatchOne: %v", err)
	}
	if got != "Firefly" {
		t.Errorf("got %v, want Firefly", got)
	}
}

func TestMultiEval_DispatchOne_FullPath(t *testing.T) {
	se := stream.NewMultiEval(nil)
	// $map requires full eval (not a fast-path expression).
	idx := se.Add(compileStreamExpr(t, "$map(items, function($v){$v.name})"))

	got, err := se.DispatchOne(context.Background(), streamTestData, "s1", idx)
	if err != nil {
		t.Fatalf("DispatchOne: %v", err)
	}
	names, ok := got.([]interface{})
	if !ok || len(names) != 2 {
		t.Fatalf("got %v (type %T), want []interface{} len 2", got, got)
	}
	if names[0] != "widget" || names[1] != "gadget" {
		t.Errorf("got %v, want [widget gadget]", names)
	}
}

// ----------------------------------------------------------------------------
// Dispatch — mixed fast/full paths
// ----------------------------------------------------------------------------

func TestMultiEval_Dispatch_MixedPaths(t *testing.T) {
	se := stream.NewMultiEval(nil)
	idxName, _ := se.Compile("Account.Name")     // fast: pure path
	idxActive, _ := se.Compile("Account.Active") // fast: pure path
	idxCount, _ := se.Compile("$count(items)")   // fast: func
	// A filter expression that requires full evaluation.
	idxFilter, _ := se.Compile("items[price > 10].name")

	indices := []int{idxName, idxActive, idxCount, idxFilter}
	results, err := se.Dispatch(context.Background(), streamTestData, "mixed", indices)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("len=%d, want 4", len(results))
	}
	if results[0] != "Firefly" {
		t.Errorf("[0] got %v, want Firefly", results[0])
	}
	if results[1] != true {
		t.Errorf("[1] got %v, want true", results[1])
	}
	if results[2] != float64(2) {
		t.Errorf("[2] got %v, want 2", results[2])
	}
}

func TestMultiEval_Dispatch_EmptyIndices(t *testing.T) {
	se := stream.NewMultiEval(nil)
	got, err := se.Dispatch(context.Background(), streamTestData, "s1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil for empty indices", got)
	}
}

func TestMultiEval_Dispatch_OutOfRange(t *testing.T) {
	se := stream.NewMultiEval(nil)
	_, err := se.Dispatch(context.Background(), streamTestData, "s1", []int{99})
	if err == nil {
		t.Error("expected error for out-of-range index, got nil")
	}
}

// ----------------------------------------------------------------------------
// Initial expressions via constructor
// ----------------------------------------------------------------------------

func TestMultiEval_InitialExpressions(t *testing.T) {
	exprs := []*types.Expression{
		compileStreamExpr(t, "Account.Name"),
		compileStreamExpr(t, "score"),
	}
	se := stream.NewMultiEval(exprs)
	if se.Len() != 2 {
		t.Fatalf("Len=%d, want 2", se.Len())
	}
	results, err := se.Dispatch(context.Background(), streamTestData, "s1", []int{0, 1})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if results[0] != "Firefly" {
		t.Errorf("[0] got %v, want Firefly", results[0])
	}
	if results[1] != float64(42) {
		t.Errorf("[1] got %v, want 42", results[1])
	}
}

// ----------------------------------------------------------------------------
// EvalPlan cache hit rate
// ----------------------------------------------------------------------------

// streamCountingHook counts plan-cache events for assertions.
type streamCountingHook struct {
	hits      atomic.Int64
	misses    atomic.Int64
	evictions atomic.Int64
	evals     atomic.Int64
}

func (h *streamCountingHook) OnEval(_ int, _ bool, _ time.Duration, _ error) { h.evals.Add(1) }
func (h *streamCountingHook) OnPlanHit(_ string)                             { h.hits.Add(1) }
func (h *streamCountingHook) OnPlanMiss(_ string)                            { h.misses.Add(1) }
func (h *streamCountingHook) OnEviction()                                    { h.evictions.Add(1) }

func TestMultiEval_CacheHitRate(t *testing.T) {
	hook := &streamCountingHook{}
	se := stream.NewMultiEvalWithObserver(nil, hook)

	idxName, _ := se.Compile("Account.Name")
	idxScore, _ := se.Compile("score")
	indices := []int{idxName, idxScore}

	ctx := context.Background()
	const N = 10
	for i := 0; i < N; i++ {
		if _, err := se.Dispatch(ctx, streamTestData, "schema:v1", indices); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}

	if hook.misses.Load() != 1 {
		t.Errorf("misses=%d, want 1 (first call builds the plan)", hook.misses.Load())
	}
	if hook.hits.Load() != int64(N-1) {
		t.Errorf("hits=%d, want %d", hook.hits.Load(), N-1)
	}
}

// Different schema keys must produce independent EvalPlan entries.
func TestMultiEval_CacheSeparateKeys(t *testing.T) {
	hook := &streamCountingHook{}
	se := stream.NewMultiEvalWithObserver(nil, hook)
	idx, _ := se.Compile("Account.Name")

	ctx := context.Background()
	se.DispatchOne(ctx, streamTestData, "schemaA", idx) //nolint:errcheck
	se.DispatchOne(ctx, streamTestData, "schemaB", idx) //nolint:errcheck
	se.DispatchOne(ctx, streamTestData, "schemaA", idx) //nolint:errcheck

	// schemaA miss + schemaB miss = 2 misses; schemaA hit = 1 hit
	if hook.misses.Load() != 2 {
		t.Errorf("misses=%d, want 2", hook.misses.Load())
	}
	if hook.hits.Load() != 1 {
		t.Errorf("hits=%d, want 1", hook.hits.Load())
	}
}

// ----------------------------------------------------------------------------
// Mutation: Compile / Replace / Remove / Reset
// ----------------------------------------------------------------------------

func TestMultiEval_Compile_Error(t *testing.T) {
	se := stream.NewMultiEval(nil)
	_, err := se.Compile("$$$invalid!!!!")
	if err == nil {
		t.Error("expected compile error, got nil")
	}
}

func TestMultiEval_Replace_InvalidatesCache(t *testing.T) {
	hook := &streamCountingHook{}
	se := stream.NewMultiEvalWithObserver(nil, hook)

	idx, _ := se.Compile("Account.Name")
	ctx := context.Background()

	// Warm up the plan cache.
	if _, err := se.DispatchOne(ctx, streamTestData, "s1", idx); err != nil {
		t.Fatal(err)
	}
	// Replace → full cache invalidation.
	if err := se.Replace(idx, "score"); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	// Next call must result in a cache miss.
	if _, err := se.DispatchOne(ctx, streamTestData, "s1", idx); err != nil {
		t.Fatal(err)
	}

	// 1 miss (warm-up) + 1 miss (after Replace) = 2
	if hook.misses.Load() != 2 {
		t.Errorf("misses=%d, want 2", hook.misses.Load())
	}

	// Verify the replaced expression evaluates correctly.
	got, err := se.DispatchOne(ctx, streamTestData, "s1", idx)
	if err != nil {
		t.Fatal(err)
	}
	if got != float64(42) {
		t.Errorf("got %v, want 42", got)
	}
}

func TestMultiEval_Replace_OutOfRange(t *testing.T) {
	se := stream.NewMultiEval(nil)
	if err := se.Replace(0, "score"); err == nil {
		t.Error("expected out-of-range error")
	}
}

func TestMultiEval_Remove_SlotBecomesNil(t *testing.T) {
	se := stream.NewMultiEval(nil)
	idx, _ := se.Compile("Account.Name")
	if err := se.Remove(idx); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	// The slot index is still valid; result must be nil.
	results, err := se.Dispatch(context.Background(), streamTestData, "s1", []int{idx})
	if err != nil {
		t.Fatalf("Dispatch after Remove: %v", err)
	}
	if results[0] != nil {
		t.Errorf("got %v, want nil for removed slot", results[0])
	}
}

func TestMultiEval_Remove_OutOfRange(t *testing.T) {
	se := stream.NewMultiEval(nil)
	if err := se.Remove(5); err == nil {
		t.Error("expected out-of-range error")
	}
}

func TestMultiEval_Reset(t *testing.T) {
	se := stream.NewMultiEval(nil)
	se.Compile("Account.Name") //nolint:errcheck
	se.Compile("score")        //nolint:errcheck
	se.Reset()
	if se.Len() != 0 {
		t.Errorf("Len=%d after Reset, want 0", se.Len())
	}
}

// ----------------------------------------------------------------------------
// Path deduplication in EvalPlan
// ----------------------------------------------------------------------------

func TestMultiEval_PathDeduplication(t *testing.T) {
	se := stream.NewMultiEval(nil)
	// Both expressions reference "score" → EvalPlan should merge to 1 path.
	idx1, _ := se.Compile("score")
	idx2, _ := se.Compile(`score = 42`)

	results, err := se.Dispatch(context.Background(), streamTestData, "dedup", []int{idx1, idx2})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if results[0] != float64(42) {
		t.Errorf("[0] got %v, want 42", results[0])
	}
	if results[1] != true {
		t.Errorf("[1] got %v, want true", results[1])
	}
}

// ----------------------------------------------------------------------------
// Stats
// ----------------------------------------------------------------------------

func TestMultiEval_Stats(t *testing.T) {
	se := stream.NewMultiEval(nil)
	idx, _ := se.Compile("Account.Name")

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := se.DispatchOne(ctx, streamTestData, "s1", idx); err != nil {
			t.Fatal(err)
		}
	}

	stats := se.Stats()
	if stats.EvalCount != 5 {
		t.Errorf("EvalCount=%d, want 5", stats.EvalCount)
	}
	if stats.FastPathCount != 5 {
		t.Errorf("FastPathCount=%d, want 5", stats.FastPathCount)
	}
	if stats.ErrorCount != 0 {
		t.Errorf("ErrorCount=%d, want 0", stats.ErrorCount)
	}
	if stats.Plans.Hits != 4 {
		t.Errorf("PlanCache.Hits=%d, want 4", stats.Plans.Hits)
	}
	if stats.Plans.Misses != 1 {
		t.Errorf("PlanCache.Misses=%d, want 1", stats.Plans.Misses)
	}
}

// ----------------------------------------------------------------------------
// Concurrency safety
// ----------------------------------------------------------------------------

func TestMultiEval_Concurrent(t *testing.T) {
	se := stream.NewMultiEval(nil)
	idx1, _ := se.Compile("Account.Name")
	idx2, _ := se.Compile("score")
	idx3, _ := se.Compile("$count(items)")

	const goroutines = 50
	const iters = 100
	ctx := context.Background()

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				res, err := se.Dispatch(ctx, streamTestData, "concurrent", []int{idx1, idx2, idx3})
				if err != nil {
					errCh <- err
					return
				}
				if res[0] != "Firefly" || res[1] != float64(42) || res[2] != float64(2) {
					errCh <- nil
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Errorf("concurrent eval error: %v", err)
		}
	}
}

// ----------------------------------------------------------------------------
// NoopEvalObserver satisfies EvalObserver with zero overhead
// ----------------------------------------------------------------------------

func TestMultiEval_NoopEvalObserver(t *testing.T) {
	noop := stream.NoopObserver{}
	// Should not panic.
	noop.OnEval(0, true, time.Millisecond, nil)
	noop.OnPlanHit("k")
	noop.OnPlanMiss("k")
	noop.OnEviction()
}

// ----------------------------------------------------------------------------
// Benchmarks
// ----------------------------------------------------------------------------

var streamBenchExprs = []string{
	"Account.Name",  // fast: pure path
	"score",         // fast: pure path
	`score = 42`,    // fast: comparison
	"$count(items)", // fast: func
}

func BenchmarkMultiEval_Dispatch_4(b *testing.B) {
	se := stream.NewMultiEval(nil)
	indices := make([]int, len(streamBenchExprs))
	for i, q := range streamBenchExprs {
		idx, err := se.Compile(q)
		if err != nil {
			b.Fatalf("compile: %v", err)
		}
		indices[i] = idx
	}
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = se.Dispatch(ctx, streamTestData, "bench", indices)
	}
}

func BenchmarkMultiEval_Dispatch_20(b *testing.B) {
	se := stream.NewMultiEval(nil)
	indices := make([]int, 0, 20)
	for rep := 0; rep < 5; rep++ {
		for _, q := range streamBenchExprs {
			idx, _ := se.Compile(q)
			indices = append(indices, idx)
		}
	}
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = se.Dispatch(ctx, streamTestData, "bench20", indices)
	}
}

func BenchmarkMultiEval_Parallel(b *testing.B) {
	se := stream.NewMultiEval(nil)
	indices := make([]int, len(streamBenchExprs))
	for i, q := range streamBenchExprs {
		idx, _ := se.Compile(q)
		indices[i] = idx
	}
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = se.Dispatch(ctx, streamTestData, "bench-par", indices)
		}
	})
}

func BenchmarkMultiEval_DispatchOne_FastPath(b *testing.B) {
	se := stream.NewMultiEval(nil)
	idx := se.Add(compileStreamExpr(b, "Account.Name"))
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = se.DispatchOne(ctx, streamTestData, "bench-one", idx)
	}
}
