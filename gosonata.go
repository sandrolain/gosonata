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
	"time"

	"github.com/sandrolain/gosonata/pkg/evaluator"
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
//
// Example:
//
//	result, err := gosonata.Eval("$.name", data)
func Eval(query string, data interface{}, opts ...evaluator.EvalOption) (interface{}, error) {
	expr, err := Compile(query)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	eval := evaluator.New(opts...)
	return eval.Eval(ctx, expr, data)
}

// EvalWithContext evaluates an expression with a custom context.
func EvalWithContext(ctx context.Context, query string, data interface{}, opts ...evaluator.EvalOption) (interface{}, error) {
	expr, err := Compile(query)
	if err != nil {
		return nil, err
	}

	eval := evaluator.New(opts...)
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
