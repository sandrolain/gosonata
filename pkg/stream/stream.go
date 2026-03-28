package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tidwall/gjson"

	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/parser"
	"github.com/sandrolain/gosonata/pkg/types"
)

// ----------------------------------------------------------------------------
// EvalPlan
// ----------------------------------------------------------------------------

// EvalPlan is the pre-computed evaluation plan for a specific (topic key,
// expression indices) pair. Once constructed it is immutable and can be shared
// safely across goroutines.
type EvalPlan struct {
	// mergedPaths contains one gjson path per fast-path expression, deduplicated.
	// A single gjson.GetManyBytes call scans the document once for all of them.
	mergedPaths []string

	// fastOffset maps local-index (position in the caller's indices slice)
	// to an offset into mergedPaths / the gjson results slice.
	fastOffset map[int]int

	// fullIndices holds the local indices of expressions that require full AST
	// evaluation via json.Unmarshal + evaluator.Eval.
	fullIndices []int
}

// ----------------------------------------------------------------------------
// MultiEvalOptions / MultiEvalOption
// ----------------------------------------------------------------------------

// MultiEvalOption is a functional option for [NewMultiEval].
type MultiEvalOption func(*multiEvalConfig)

type multiEvalConfig struct {
	maxCachedSchemas int
	evalOpts         []evaluator.EvalOption
}

// WithPlanCacheSize sets the capacity of the EvalPlan cache.
// Default is 256. A value ≤ 0 keeps the default.
func WithPlanCacheSize(n int) MultiEvalOption {
	return func(c *multiEvalConfig) { c.maxCachedSchemas = n }
}

// WithEvaluatorOptions forwards one or more [evaluator.EvalOption] values to the
// internal evaluator used for full-path (non-fast-path) expressions.
//
// Use this to register custom functions:
//
//	me := stream.NewMultiEval(exprs,
//	    stream.WithEvaluatorOptions(
//	        gosonata.WithCustomFunction("myFunc", "", myImpl),
//	    ),
//	)
func WithEvaluatorOptions(opts ...evaluator.EvalOption) MultiEvalOption {
	return func(c *multiEvalConfig) { c.evalOpts = append(c.evalOpts, opts...) }
}

// ----------------------------------------------------------------------------
// MultiEvalStats
// ----------------------------------------------------------------------------

// MultiEvalStats holds aggregate runtime statistics for a [MultiEval].
type MultiEvalStats struct {
	// Plans holds statistics for the EvalPlan cache.
	Plans PlanStats
	// EvalCount is the total number of individual expression evaluations.
	EvalCount int64
	// FastPathCount is the number of evaluations that used the gjson fast path.
	FastPathCount int64
	// ErrorCount is the total number of evaluation errors.
	ErrorCount int64
}

// ----------------------------------------------------------------------------
// MultiEval
// ----------------------------------------------------------------------------

// MultiEval evaluates a set of JSONata expressions against a stream of
// JSON documents with minimal per-document allocations.
//
// Expressions are identified by stable integer indices returned by [Compile].
// For each call to [Dispatch], the evaluator:
//  1. Looks up (or builds) an [EvalPlan] for the (topicKey, indices) pair.
//  2. Calls gjson.GetManyBytes once to extract all fast-path fields.
//  3. Applies comparison / function logic to each extracted result.
//  4. Calls json.Unmarshal + AST eval only for the remaining expressions.
//
// MultiEval is safe for concurrent use.
type MultiEval struct {
	mu          sync.RWMutex
	expressions []compiledExpr

	planCache *evalPlanCache
	ev        *evaluator.Evaluator // for full-path evaluation

	metrics   EvalObserver
	evalCount atomic.Int64
	fastCount atomic.Int64
	errCount  atomic.Int64
}

// compiledExpr wraps an [*types.Expression] for internal bookkeeping.
type compiledExpr struct {
	expr   *types.Expression
	source string
}

// NewMultiEval creates a MultiEval pre-loaded with initial
// expressions. Pass nil (or an empty slice) to start empty.
//
// Each element of initial must be non-nil. Use [Compile] to add further
// expressions after construction.
func NewMultiEval(initial []*types.Expression, opts ...MultiEvalOption) *MultiEval {
	cfg := &multiEvalConfig{maxCachedSchemas: 256}

	// Extract the observer before applying options (it needs special handling).
	var hook EvalObserver
	for _, o := range opts {
		// WithEvaluatorOptions stores options via closure — we capture it here.
		o(cfg)
	}
	// Re-apply to capture hook (pattern kept for extensibility).
	_ = hook

	se := &MultiEval{
		planCache: newEvalPlanCache(cfg.maxCachedSchemas),
		ev:        evaluator.New(cfg.evalOpts...),
	}

	for _, e := range initial {
		if e != nil {
			se.expressions = append(se.expressions, compiledExpr{expr: e, source: e.Source()})
		}
	}
	return se
}

// newMultiEvalInternal is the internal constructor that also accepts an observer.
// Called by NewMultiEvalWithObserver.
func newMultiEvalInternal(initial []*types.Expression, hook EvalObserver, cfg *multiEvalConfig) *MultiEval {
	se := &MultiEval{
		planCache: newEvalPlanCache(cfg.maxCachedSchemas),
		ev:        evaluator.New(cfg.evalOpts...),
		metrics:   hook,
	}
	for _, e := range initial {
		if e != nil {
			se.expressions = append(se.expressions, compiledExpr{expr: e, source: e.Source()})
		}
	}
	return se
}

// NewMultiEvalWithObserver creates a MultiEval with an [EvalObserver].
//
// This constructor lets you combine an observer with other [MultiEvalOption]
// values:
//
//	me := stream.NewMultiEvalWithObserver(nil, myObserver,
//	    stream.WithPlanCacheSize(1000),
//	    stream.WithEvaluatorOptions(gosonata.WithCustomFunction(...)),
//	)
func NewMultiEvalWithObserver(initial []*types.Expression, observer EvalObserver, opts ...MultiEvalOption) *MultiEval {
	cfg := &multiEvalConfig{maxCachedSchemas: 256}
	for _, o := range opts {
		o(cfg)
	}
	return newMultiEvalInternal(initial, observer, cfg)
}

// ----------------------------------------------------------------------------
// Expression management
// ----------------------------------------------------------------------------

// Compile compiles query and adds it to the MultiEval.
// Returns the stable index that can be passed to [Dispatch] / [DispatchOne].
// Invalidates any cached EvalPlan that references this slot if it was a Replace.
func (se *MultiEval) Compile(query string) (int, error) {
	expr, err := parser.Compile(query)
	if err != nil {
		return -1, fmt.Errorf("stream: compile %q: %w", query, err)
	}
	se.mu.Lock()
	idx := len(se.expressions)
	se.expressions = append(se.expressions, compiledExpr{expr: expr, source: query})
	se.mu.Unlock()
	// A new slot never appeared in any existing plan — no cache invalidation needed.
	return idx, nil
}

// Add adds a pre-compiled expression to the MultiEval.
// Returns the stable index. The expression must be non-nil.
func (se *MultiEval) Add(expr *types.Expression) int {
	if expr == nil {
		panic("stream: Add called with nil expression")
	}
	se.mu.Lock()
	idx := len(se.expressions)
	se.expressions = append(se.expressions, compiledExpr{expr: expr, source: expr.Source()})
	se.mu.Unlock()
	return idx
}

// Replace swaps the expression at idx with a newly compiled query.
// All cached EvalPlans are invalidated because the fast-path classification
// of slot idx may have changed.
// Returns an error if idx is out of range or if compilation fails.
func (se *MultiEval) Replace(idx int, query string) error {
	expr, err := parser.Compile(query)
	if err != nil {
		return fmt.Errorf("stream: replace[%d] compile %q: %w", idx, query, err)
	}
	se.mu.Lock()
	if idx < 0 || idx >= len(se.expressions) {
		se.mu.Unlock()
		return fmt.Errorf("stream: Replace: index %d out of range [0, %d)", idx, len(se.expressions))
	}
	se.expressions[idx] = compiledExpr{expr: expr, source: query}
	se.mu.Unlock()
	// Invalidate the entire plan cache because the fast-path class of slot idx
	// may differ from the old expression.
	se.planCache.clear()
	return nil
}

// Remove marks the slot at idx as empty (nil expression).
// The index remains allocated; subsequent Dispatch calls that include idx will
// always return nil for that slot without error.
// All cached EvalPlans are invalidated.
// Returns an error if idx is out of range.
func (se *MultiEval) Remove(idx int) error {
	se.mu.Lock()
	if idx < 0 || idx >= len(se.expressions) {
		se.mu.Unlock()
		return fmt.Errorf("stream: Remove: index %d out of range [0, %d)", idx, len(se.expressions))
	}
	se.expressions[idx] = compiledExpr{} // nil expr signals removed slot
	se.mu.Unlock()
	se.planCache.clear()
	return nil
}

// Reset removes all expressions and clears the plan cache.
func (se *MultiEval) Reset() {
	se.mu.Lock()
	se.expressions = se.expressions[:0]
	se.mu.Unlock()
	se.planCache.clear()
}

// Len returns the current number of expression slots (including removed ones).
func (se *MultiEval) Len() int {
	se.mu.RLock()
	n := len(se.expressions)
	se.mu.RUnlock()
	return n
}

// Stats returns a point-in-time snapshot of runtime statistics.
func (se *MultiEval) Stats() MultiEvalStats {
	return MultiEvalStats{
		Plans:         se.planCache.stats(),
		EvalCount:     se.evalCount.Load(),
		FastPathCount: se.fastCount.Load(),
		ErrorCount:    se.errCount.Load(),
	}
}

// ----------------------------------------------------------------------------
// Evaluation
// ----------------------------------------------------------------------------

// Dispatch evaluates the expressions at the given indices against rawJSON.
//
// topicKey is an opaque string that identifies the JSON document schema.
// It is used as part of the EvalPlan cache key. Callers that deal with
// homogeneous streams should use a fixed, short key (e.g. "v1"). Callers
// that mix schemas should use a key that changes with the schema (e.g. a
// content-type or schema fingerprint).
//
// The returned slice has the same length as indices. A nil element means the
// expression returned JSONata undefined or the slot was removed.
//
// Dispatch is safe for concurrent use.
func (se *MultiEval) Dispatch(
	ctx context.Context,
	rawJSON json.RawMessage,
	topicKey string,
	indices []int,
) ([]any, error) {
	if len(indices) == 0 {
		return nil, nil
	}

	// Snapshot the expression slice under read lock.
	se.mu.RLock()
	exprs := se.expressions
	se.mu.RUnlock()

	// Validate indices.
	for _, idx := range indices {
		if idx < 0 || idx >= len(exprs) {
			return nil, fmt.Errorf("stream: Dispatch: index %d out of range [0, %d)", idx, len(exprs))
		}
	}

	// Retrieve or build EvalPlan.
	planKey := buildDispatchKey(topicKey, indices)
	plan, ok := se.planCache.get(planKey)
	if ok {
		if se.metrics != nil {
			se.metrics.OnPlanHit(planKey)
		}
	} else {
		if se.metrics != nil {
			se.metrics.OnPlanMiss(planKey)
		}
		plan = buildEvalPlan(exprs, indices)
		evicted := se.planCache.set(planKey, plan)
		if evicted && se.metrics != nil {
			se.metrics.OnEviction()
		}
	}

	results := make([]any, len(indices))

	// Fast-path: single gjson scan for all fast-path expressions.
	if len(plan.mergedPaths) > 0 {
		gjsonResults := gjson.GetManyBytes(rawJSON, plan.mergedPaths...)

		for localIdx, offset := range plan.fastOffset {
			exprSlot := exprs[indices[localIdx]]
			if exprSlot.expr == nil {
				continue
			}

			start := time.Now()
			val, _, err := evaluator.EvalFastFromResult(exprSlot.expr.FastPath, gjsonResults[offset])

			se.evalCount.Add(1)
			se.fastCount.Add(1)
			if err != nil {
				se.errCount.Add(1)
			}
			if se.metrics != nil {
				se.metrics.OnEval(indices[localIdx], true, time.Since(start), err)
			}

			if err != nil {
				return nil, fmt.Errorf("stream: fast-path eval[%d] %q: %w",
					indices[localIdx], exprSlot.source, err)
			}
			results[localIdx] = val
		}
	}

	// Full-path: unmarshal lazily, only when needed.
	if len(plan.fullIndices) > 0 {
		var data any
		if err := json.Unmarshal(rawJSON, &data); err != nil {
			return nil, fmt.Errorf("stream: unmarshal: %w", err)
		}

		for _, localIdx := range plan.fullIndices {
			exprSlot := exprs[indices[localIdx]]
			if exprSlot.expr == nil {
				continue
			}

			start := time.Now()
			val, err := se.ev.Eval(ctx, exprSlot.expr, data)

			se.evalCount.Add(1)
			if err != nil {
				se.errCount.Add(1)
			}
			if se.metrics != nil {
				se.metrics.OnEval(indices[localIdx], false, time.Since(start), err)
			}

			if err != nil {
				return nil, fmt.Errorf("stream: full-path eval[%d] %q: %w",
					indices[localIdx], exprSlot.source, err)
			}
			results[localIdx] = val
		}
	}

	return results, nil
}

// DispatchOne evaluates a single expression at exprIndex against rawJSON.
// It is a convenience wrapper around [Dispatch].
func (se *MultiEval) DispatchOne(
	ctx context.Context,
	rawJSON json.RawMessage,
	topicKey string,
	exprIndex int,
) (any, error) {
	results, err := se.Dispatch(ctx, rawJSON, topicKey, []int{exprIndex})
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

// ----------------------------------------------------------------------------
// Internal helpers
// ----------------------------------------------------------------------------

// buildEvalPlan analyses the expressions at the given indices and constructs
// an EvalPlan that separates fast-path expressions from those needing full eval.
func buildEvalPlan(exprs []compiledExpr, indices []int) *EvalPlan {
	plan := &EvalPlan{
		fastOffset: make(map[int]int, len(indices)),
	}
	// pathIndex deduplicates gjson paths so we don't scan the same field twice.
	pathIndex := make(map[string]int, len(indices))

	for localIdx, exprIdx := range indices {
		slot := exprs[exprIdx]
		if slot.expr == nil || !slot.expr.IsFastPath() {
			plan.fullIndices = append(plan.fullIndices, localIdx)
			continue
		}

		gjsonPath := slot.expr.FastPath.GJSONPath
		offset, exists := pathIndex[gjsonPath]
		if !exists {
			offset = len(plan.mergedPaths)
			plan.mergedPaths = append(plan.mergedPaths, gjsonPath)
			pathIndex[gjsonPath] = offset
		}
		plan.fastOffset[localIdx] = offset
	}

	return plan
}

// buildDispatchKey returns a cache key that encodes both the topic key and the
// ordered set of expression indices.
func buildDispatchKey(topicKey string, indices []int) string {
	var b strings.Builder
	b.WriteString(topicKey)
	b.WriteByte('\x00')
	for i, idx := range indices {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(idx))
	}
	return b.String()
}
