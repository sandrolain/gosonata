package evaluator

// Package evaluator implements the JSONata expression evaluation engine.
//
// The evaluator receives a parsed Abstract Syntax Tree (AST) from the parser
// and evaluates it against JSON data. It supports:
//   - Path navigation and filtering
//   - Function calls (built-in and lambdas)
//   - Concurrent evaluation (optional)
//   - Context management and variable bindings
//   - Timeout and cancellation via context.Context
//
// # Example
//
//	evaluator := evaluator.New()
//	result, err := evaluator.Eval(ctx, expr.AST(), data)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Concurrency
//
// The evaluator supports concurrent evaluation of independent expressions.
// This can significantly improve performance for complex queries.
//
//	results, err := evaluator.EvalMany(ctx, queries, data)

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/sandrolain/gosonata/pkg/types"
)

// Evaluator evaluates JSONata expressions against data.
type Evaluator struct {
	opts   EvalOptions
	logger *slog.Logger
}

// EvalOptions configures evaluator behavior.
type EvalOptions struct {
	// Caching enables expression result caching.
	Caching bool
	// Concurrency enables concurrent evaluation.
	Concurrency bool
	// MaxDepth limits recursion depth.
	MaxDepth int
	// Timeout sets evaluation timeout.
	Timeout time.Duration
	// Debug enables debug logging.
	Debug bool
	// Logger for structured logging.
	Logger *slog.Logger
}

// New creates a new Evaluator with default options.
func New(opts ...EvalOption) *Evaluator {
	options := EvalOptions{
		Caching:     false, // Disabled by default
		Concurrency: true,  // Enabled by default
		MaxDepth:    100,
		Timeout:     30 * time.Second,
	}

	for _, opt := range opts {
		opt(&options)
	}

	if options.Logger == nil {
		options.Logger = slog.Default()
	}

	return &Evaluator{
		opts:   options,
		logger: options.Logger,
	}
}

// Eval evaluates an expression against data.
func (e *Evaluator) Eval(ctx context.Context, expr *types.Expression, data interface{}) (interface{}, error) {
	if expr == nil || expr.AST() == nil {
		return nil, fmt.Errorf("invalid expression")
	}

	// Apply timeout if configured
	if e.opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.opts.Timeout)
		defer cancel()
	}

	// Create evaluation context
	evalCtx := NewContext(data)

	// Evaluate the AST
	result, err := e.evalNode(ctx, expr.AST(), evalCtx)
	if err != nil {
		return nil, err
	}

	// Convert types.Null to nil before returning
	result = e.convertNullToNil(result)

	// Singleton array unwrapping: JSONata unwraps singleton arrays at the top level
	// UNLESS the expression has KeepArray flag set (e.g., using [] syntax)
	if arr, ok := result.([]interface{}); ok && len(arr) == 1 {
		// Check if we should keep the array structure
		keepArray := expr.AST().KeepArray || hasKeepArrayInASTChain(expr.AST())
		if !keepArray {
			return arr[0], nil
		}
	}

	return result, nil
}

// hasKeepArrayInASTChain checks if any node in the AST chain has KeepArray set.
func hasKeepArrayInASTChain(node *types.ASTNode) bool {
	if node == nil {
		return false
	}
	if node.KeepArray {
		return true
	}
	// Recursively check LHS chain
	if node.LHS != nil && hasKeepArrayInASTChain(node.LHS) {
		return true
	}
	// Also check RHS chain (for completeness)
	if node.RHS != nil && hasKeepArrayInASTChain(node.RHS) {
		return true
	}
	return false
}

// EvalWithBindings evaluates an expression with custom variable bindings.
func (e *Evaluator) EvalWithBindings(ctx context.Context, expr *types.Expression, data interface{}, bindings map[string]interface{}) (interface{}, error) {
	if expr == nil || expr.AST() == nil {
		return nil, fmt.Errorf("invalid expression")
	}

	// Apply timeout if configured
	if e.opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.opts.Timeout)
		defer cancel()
	}

	// Create evaluation context with bindings
	evalCtx := NewContext(data)
	evalCtx.SetBindings(bindings)

	// Evaluate the AST
	result, err := e.evalNode(ctx, expr.AST(), evalCtx)
	if err != nil {
		return nil, err
	}

	// Convert types.Null to nil before returning
	return e.convertNullToNil(result), nil
}

// EvalOption configures evaluation behavior.
type EvalOption func(*EvalOptions)

// WithCaching enables or disables result caching.
func WithCaching(enabled bool) EvalOption {
	return func(opts *EvalOptions) {
		opts.Caching = enabled
	}
}

// WithConcurrency enables or disables concurrent evaluation.
func WithConcurrency(enabled bool) EvalOption {
	return func(opts *EvalOptions) {
		opts.Concurrency = enabled
	}
}

// WithTimeout sets the evaluation timeout.
func WithTimeout(timeout time.Duration) EvalOption {
	return func(opts *EvalOptions) {
		opts.Timeout = timeout
	}
}

// WithDebug enables or disables debug logging.
func WithDebug(enabled bool) EvalOption {
	return func(opts *EvalOptions) {
		opts.Debug = enabled
	}
}

// WithLogger sets a custom logger.
func WithLogger(logger *slog.Logger) EvalOption {
	return func(opts *EvalOptions) {
		opts.Logger = logger
	}
}

// WithMaxDepth sets the maximum recursion depth.
func WithMaxDepth(depth int) EvalOption {
	return func(opts *EvalOptions) {
		opts.MaxDepth = depth
	}
}

// convertNullToNil recursively converts types.Null to nil in result values.
// This is called at the final return to convert internal types.Null representation
// (which is kept during evaluation to distinguish from undefined) to nil for external API.
func (e *Evaluator) convertNullToNil(value interface{}) interface{} {
	switch v := value.(type) {
	case types.Null:
		return nil
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = e.convertNullToNil(item)
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, item := range v {
			result[key] = e.convertNullToNil(item)
		}
		return result
	case *OrderedObject:
		result := &OrderedObject{
			Keys:   v.Keys,
			Values: make(map[string]interface{}, len(v.Values)),
		}
		for key, item := range v.Values {
			result.Values[key] = e.convertNullToNil(item)
		}
		return result
	default:
		return value
	}
}
