package evaluator

import (
	"context"
)

type recurseDepthKey struct{}

// tcoThunk represents a pending tail-call invocation (used for trampolining).

type tcoThunk struct {
	lambda *Lambda
	args   []interface{}
}

// getRecurseDepthPtr returns the depth counter pointer from the context, creating one if absent.

func getRecurseDepthPtr(ctx context.Context) *int {
	if p, ok := ctx.Value(recurseDepthKey{}).(*int); ok {
		return p
	}
	return nil
}

// withNewRecurseDepthPtr returns a context that carries a fresh depth counter pointer.
// Call this once at the start of each top-level evaluation.

func withNewRecurseDepthPtr(ctx context.Context) context.Context {
	d := 0
	return context.WithValue(ctx, recurseDepthKey{}, &d)
}
