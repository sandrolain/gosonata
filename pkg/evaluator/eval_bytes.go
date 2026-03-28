package evaluator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sandrolain/gosonata/pkg/types"
)

// EvalBytes evaluates expr against raw JSON bytes.
//
// For expressions classified as fast-path at compile time (see [types.Expression.IsFastPath]),
// EvalBytes uses gjson to extract fields with zero-copy string views — the JSON document
// is never fully decoded with json.Unmarshal. This can yield 10–100× lower latency
// for simple path navigations and comparisons.
//
// For all other expressions, EvalBytes decodes raw into a Go value and delegates
// to [Evaluator.Eval].
//
// Example:
//
//	expr, _ := gosonata.Compile(`user.email = "admin@example.com"`)
//	result, _ := ev.EvalBytes(ctx, expr, rawJSON)
//	fmt.Println(expr.IsFastPath()) // true — zero-copy evaluation
func (e *Evaluator) EvalBytes(ctx context.Context, expr *types.Expression, raw json.RawMessage) (any, error) {
	if expr == nil || expr.AST() == nil {
		return nil, fmt.Errorf("invalid expression")
	}

	// Apply timeout if configured (mirrors Evaluator.Eval).
	if e.opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.opts.Timeout)
		defer cancel()
	}

	// Fast-path: extract fields via gjson without unmarshaling the whole doc.
	if expr.IsFastPath() {
		result, ok, err := EvalFast(ctx, expr.FastPath, raw)
		if ok {
			return result, err
		}
	}

	// Full-path fallback: unmarshal then evaluate.
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("EvalBytes: unmarshal failed: %w", err)
	}

	return e.Eval(ctx, expr, data)
}
