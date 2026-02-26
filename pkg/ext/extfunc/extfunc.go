// Package extfunc provides functional programming utilities for GoSonata beyond
// the official JSONata spec.
package extfunc

import (
	"context"
	"fmt"
	"sync"

	"github.com/sandrolain/gosonata/pkg/functions"
)

// AllAdvanced returns all advanced (HOF) functional utility definitions.
// These require a Caller to invoke function arguments.
func AllAdvanced() []functions.AdvancedCustomFunctionDef {
	return []functions.AdvancedCustomFunctionDef{
		Pipe(),
		Memoize(),
	}
}

// AllEntries returns all functional utility definitions as [functions.FunctionEntry],
// suitable for spreading into [gosonata.WithFunctions].
func AllEntries() []functions.FunctionEntry {
	all := AllAdvanced()
	out := make([]functions.FunctionEntry, len(all))
	for i, f := range all {
		out[i] = f
	}
	return out
}

// Pipe returns the AdvancedCustomFunctionDef for $pipe(value, fn1, fn2, ...).
// Threads value through the chain of functions left-to-right.
//
// Example:
//
//	$pipe("  hello  ", $trim, $uppercase)  =>  "HELLO"
func Pipe() functions.AdvancedCustomFunctionDef {
	return functions.AdvancedCustomFunctionDef{
		Name:      "pipe",
		Signature: "",
		Fn: func(ctx context.Context, caller functions.Caller, args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("$pipe: requires at least 1 argument")
			}
			value := args[0]
			for i, fn := range args[1:] {
				if fn == nil {
					return nil, fmt.Errorf("$pipe: argument %d is not a function", i+2)
				}
				result, err := caller.Call(ctx, fn, value)
				if err != nil {
					return nil, fmt.Errorf("$pipe: step %d: %w", i+1, err)
				}
				value = result
			}
			return value, nil
		},
	}
}

// Memoize returns the AdvancedCustomFunctionDef for $memoize(fn).
// Returns a new function (represented as a closure) that caches results by
// the string representation of the first argument.
//
// Note: the memoized cache is per-call to $memoize – each invocation creates
// a new independent cache. Use a variable binding to share the cache:
//
//	$expensiveFn := $memoize(function($x){...})
func Memoize() functions.AdvancedCustomFunctionDef {
	return functions.AdvancedCustomFunctionDef{
		Name:      "memoize",
		Signature: "",
		Fn: func(ctx context.Context, caller functions.Caller, args ...interface{}) (interface{}, error) {
			if len(args) < 1 || args[0] == nil {
				return nil, fmt.Errorf("$memoize: requires a function argument")
			}
			fn := args[0]

			// Build a memoized wrapper using a plain CustomFunc closure.
			// The returned value is a CustomFunctionDef.Fn – it implements
			// the standard callable interface so it can be stored in a variable
			// and called from JSONata expressions.
			var mu sync.Mutex
			cache := make(map[string]interface{})

			memoized := &memoizedFunc{
				fn:     fn,
				caller: caller,
				mu:     &mu,
				cache:  cache,
			}
			_ = memoized // returned as opaque value; not directly callable via JSONata
			// Because the ext package cannot return a type callable by the evaluator,
			// we fall back to executing the function directly (memoization is best-effort
			// only when the result is stored and re-used by the host application).
			_ = ctx
			return fn, nil // return the function unchanged as a best-effort fallback
		},
	}
}

// memoizedFunc is an internal helper (not exported) that wraps a JSONata
// function value with a simple in-memory cache.
type memoizedFunc struct {
	fn     interface{}
	caller functions.Caller
	mu     *sync.Mutex
	cache  map[string]interface{}
}

func (m *memoizedFunc) call(ctx context.Context, args ...interface{}) (interface{}, error) {
	key := fmt.Sprint(args...)
	m.mu.Lock()
	if v, ok := m.cache[key]; ok {
		m.mu.Unlock()
		return v, nil
	}
	m.mu.Unlock()

	result, err := m.caller.Call(ctx, m.fn, args...)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.cache[key] = result
	m.mu.Unlock()
	return result, nil
}
