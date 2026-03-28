// Package stream provides a high-throughput multi-expression evaluator.
//
// MultiEval evaluates a set of pre-compiled JSONata expressions against
// a stream of JSON documents. It uses a single gjson.GetManyBytes scan for all
// fast-path expressions and caches the evaluation plan per topic key with a
// lock-free FIFO cache, minimising allocations on the hot path.
//
// # Example
//
//	me := stream.NewMultiEval(nil)
//	idxName, _ := me.Compile("Account.Name")
//	idxAmt, _  := me.Compile("order.total > 100")
//
//	results, err := me.Dispatch(ctx, rawJSON, "schema:v1", []int{idxName, idxAmt})
//	// results[0] = "Firefly", results[1] = true
package stream

import "time"

// EvalObserver receives telemetry callbacks from [MultiEval].
//
// Implementations must be safe for concurrent use. A nil observer is accepted
// everywhere and incurs zero overhead (checked before any call site).
type EvalObserver interface {
	// OnEval is called after each expression is evaluated.
	// exprIndex is the slot index in the MultiEval's expression list.
	// fastPath is true when the expression used the gjson zero-copy path.
	// duration is the wall time for this single expression.
	OnEval(exprIndex int, fastPath bool, duration time.Duration, err error)

	// OnPlanHit is called when an EvalPlan was found in the plan cache.
	OnPlanHit(planKey string)

	// OnPlanMiss is called when no EvalPlan was found and one was built.
	OnPlanMiss(planKey string)

	// OnEviction is called when an EvalPlan is evicted from the plan cache.
	OnEviction()
}

// NoopObserver is a zero-overhead implementation of [EvalObserver] that
// discards every callback. Embed it to satisfy the interface while overriding
// only the methods you care about.
type NoopObserver struct{}

func (NoopObserver) OnEval(_ int, _ bool, _ time.Duration, _ error) {}
func (NoopObserver) OnPlanHit(_ string)                             {}
func (NoopObserver) OnPlanMiss(_ string)                            {}
func (NoopObserver) OnEviction()                                    {}
