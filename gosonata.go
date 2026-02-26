// Package gosonata provides a high-performance Go implementation of JSONata 2.1.0+.
//
// JSONata is a lightweight query and transformation language for JSON data.
// GoSonata is designed for intensive data streaming scenarios with focus on:
//   - Performance: Optimized parser and evaluator
//   - Concurrency: Native goroutine support
//   - Streaming: Handle large JSON documents efficiently
//   - Conformance: 100% compatibility with JSONata 2.1.0+ spec
//
// # Quick Start
//
//	// Simple evaluation
//	result, err := gosonata.Eval("$.name", data)
//
//	// Compile once, evaluate many times
//	expr, err := gosonata.Compile("$.items[price > 100]")
//	result1, _ := expr.Eval(ctx, data1)
//	result2, _ := expr.Eval(ctx, data2)
//
//	// With options
//	result, err := gosonata.Eval("$.items", data,
//	    gosonata.WithCaching(true),
//	    gosonata.WithTimeout(5*time.Second),
//	)
//
// # Performance
//
// GoSonata is optimized for:
//   - Fast parsing with hand-written recursive descent parser
//   - Efficient evaluation with minimal allocations
//   - Concurrent evaluation for independent expressions
//   - Optional caching for repeated queries
//   - Streaming support for large documents
//
// # Conformance
//
// GoSonata aims for 100% compatibility with JSONata 2.1.0+ specification,
// passing the complete official test suite (91+ test groups).
//
// # More Information
//
// For detailed documentation, see:
//   - Parser: github.com/sandrolain/gosonata/pkg/parser
//   - Evaluator: github.com/sandrolain/gosonata/pkg/evaluator
//   - Functions: github.com/sandrolain/gosonata/pkg/functions
//   - Types: github.com/sandrolain/gosonata/pkg/types
package gosonata

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/functions"
	"github.com/sandrolain/gosonata/pkg/parser"
	"github.com/sandrolain/gosonata/pkg/types"
)

// Version returns the current version of GoSonata.
func Version() string {
	return "v0.1.0-dev"
}

// Compile compiles a JSONata expression for repeated evaluation.
//
// The compiled expression can be evaluated multiple times against different
// data. It is safe for concurrent use.
//
// Example:
//
//	expr, err := gosonata.Compile("$.items[price > 100]")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	result, _ := expr.Eval(ctx, data)
func Compile(query string, opts ...parser.CompileOption) (*types.Expression, error) {
	return parser.Compile(query, opts...)
}

// Eval is a convenience function that compiles and evaluates an expression
// in a single call.
//
// For repeated evaluations of the same expression, use Compile instead.
// If WithCaching(true) is passed in opts, the compiled expression is cached
// and reused on subsequent calls with the same query string.
//
// Example:
//
//	result, err := gosonata.Eval("$.name", data)
func Eval(query string, data interface{}, opts ...evaluator.EvalOption) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return EvalWithContext(ctx, query, data, opts...)
}

// EvalWithContext evaluates an expression with a custom context.
func EvalWithContext(ctx context.Context, query string, data interface{}, opts ...evaluator.EvalOption) (interface{}, error) {
	eval := evaluator.New(opts...)

	var (
		expr *types.Expression
		err  error
	)

	// Use expression cache when available.
	if c := eval.Cache(); c != nil {
		expr, err = c.GetOrCompile(query, func() (*types.Expression, error) {
			return Compile(query)
		})
	} else {
		expr, err = Compile(query)
	}
	if err != nil {
		return nil, err
	}

	return eval.Eval(ctx, expr, data)
}

// MustCompile is like Compile but panics if the expression cannot be compiled.
// It simplifies safe initialization of global variables.
func MustCompile(query string) *types.Expression {
	expr, err := Compile(query)
	if err != nil {
		panic(fmt.Sprintf("gosonata: Compile(%q): %v", query, err))
	}
	return expr
}

// CustomFunc is the signature for user-defined functions callable from JSONata expressions.
// See WithCustomFunction.
type CustomFunc = functions.CustomFunc

// CustomFunctionDef is a type alias for [functions.CustomFunctionDef],
// re-exported so callers only need to import the top-level gosonata package.
type CustomFunctionDef = functions.CustomFunctionDef

// EvalOption is a type alias for evaluator.EvalOption so callers do not need to
// import the evaluator package directly.
type EvalOption = evaluator.EvalOption

// WithCaching re-exports evaluator.WithCaching for convenience.
func WithCaching(enabled bool) EvalOption { return evaluator.WithCaching(enabled) }

// WithCacheSize re-exports evaluator.WithCacheSize for convenience.
func WithCacheSize(size int) EvalOption { return evaluator.WithCacheSize(size) }

// WithConcurrency re-exports evaluator.WithConcurrency for convenience.
func WithConcurrency(enabled bool) EvalOption { return evaluator.WithConcurrency(enabled) }

// WithTimeout re-exports evaluator.WithTimeout for convenience.
func WithTimeout(t time.Duration) EvalOption { return evaluator.WithTimeout(t) }

// WithDebug re-exports evaluator.WithDebug for convenience.
func WithDebug(enabled bool) EvalOption { return evaluator.WithDebug(enabled) }

// WithCustomFunction registers a user-defined function with name (without "$") and
// an optional JSONata type-signature string.
//
// Example:
//
//	result, err := gosonata.Eval(`$greet("World")`, nil,
//	    gosonata.WithCustomFunction("greet", "<s:s>", func(ctx context.Context, args ...interface{}) (interface{}, error) {
//	        return "Hello, " + args[0].(string) + "!", nil
//	    }),
//	)
func WithCustomFunction(name, signature string, fn CustomFunc) EvalOption {
	return evaluator.WithCustomFunction(name, signature, fn)
}

// AdvancedCustomFunc is the signature for higher-order user-defined functions
// that receive a Caller to invoke function arguments from JSONata expressions.
type AdvancedCustomFunc = functions.AdvancedCustomFunc

// AdvancedCustomFunctionDef is a type alias for [functions.AdvancedCustomFunctionDef],
// re-exported so callers only need to import the top-level gosonata package.
type AdvancedCustomFunctionDef = functions.AdvancedCustomFunctionDef

// FunctionEntry is a type alias for functions.FunctionEntry, the common interface
// implemented by both [functions.CustomFunctionDef] and [functions.AdvancedCustomFunctionDef].
// It allows mixing both kinds in a single call to [WithFunctions].
type FunctionEntry = functions.FunctionEntry

// WithFunctions registers any mix of [CustomFunctionDef] and [AdvancedCustomFunctionDef]
// in a single variadic call. Use it to spread the result of AllEntries() from any ext
// sub-package:
//
//	gosonata.WithFunctions(extstring.AllEntries()...)
//	gosonata.WithFunctions(extarray.AllEntries()...)
//	gosonata.WithFunctions(ext.AllEntries()...)
func WithFunctions(defs ...functions.FunctionEntry) EvalOption {
	return evaluator.WithFunctions(defs...)
}

// StreamResult re-exports evaluator.StreamResult for callers that only import gosonata.
type StreamResult = evaluator.StreamResult

// EvalStream compiles query and evaluates it against each JSON value read from r.
//
// It is a convenience wrapper around Compile + Evaluator.EvalStream.
// See evaluator.EvalStream for full documentation.
func EvalStream(ctx context.Context, query string, r io.Reader, opts ...EvalOption) (<-chan StreamResult, error) {
	expr, err := Compile(query)
	if err != nil {
		return nil, err
	}
	eval := evaluator.New(opts...)
	return eval.EvalStream(ctx, expr, r)
}
