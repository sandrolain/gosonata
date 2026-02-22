package evaluator

import (
	"context"
)

type recurseDepthKey struct{}

// tcoTailKey is used to mark a context as being in TCO tail position.
// When set, tail calls return a tcoThunk instead of evaluating recursively.

type tcoTailKey struct{}

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

// withTCOTail returns a context flagging that we are in tail position (TCO).

func withTCOTail(ctx context.Context) context.Context {
	return context.WithValue(ctx, tcoTailKey{}, true)
}

// isTCOTail returns true if the context is in tail position.

func isTCOTail(ctx context.Context) bool {
	v, _ := ctx.Value(tcoTailKey{}).(bool)
	return v
}

// withoutTCOTail returns a context without the tail position flag.

func withoutTCOTail(ctx context.Context) context.Context {
	return context.WithValue(ctx, tcoTailKey{}, false)
}

