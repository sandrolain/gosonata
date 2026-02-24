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

	"github.com/sandrolain/gosonata/pkg/cache"
	"github.com/sandrolain/gosonata/pkg/functions"
	"github.com/sandrolain/gosonata/pkg/types"
)

// Evaluator evaluates JSONata expressions against data.
type Evaluator struct {
	opts      EvalOptions
	logger    *slog.Logger
	cache     *cache.Cache            // non-nil when Caching is enabled
	customFns map[string]*FunctionDef // user-registered custom functions
}

// EvalOptions configures evaluator behavior.
type EvalOptions struct {
	// Caching enables expression compilation caching.
	// When true, compiled expressions are cached by query string.
	// The default cache holds up to 256 entries with LRU eviction.
	Caching bool
	// CacheSize sets the maximum number of cached expressions.
	// Only used when Caching is true and no explicit Cache is provided.
	// Defaults to 256.
	CacheSize int
	// Cache is a custom expression cache. If non-nil, Caching is implicitly enabled.
	Cache *cache.Cache
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
	// CustomFunctions holds user-defined functions to register with the evaluator.
	CustomFunctions []functions.CustomFunctionDef
}

// defaultConcurrency controls the default value of EvalOptions.Concurrency for
// newly created Evaluators. It is true on all platforms except WebAssembly
// targets (js/wasm, wasip1), where it is set to false by init() in
// evaluator_wasm.go to avoid deadlocks in the single-threaded JS event loop.
var defaultConcurrency = true

// New creates a new Evaluator with default options.
func New(opts ...EvalOption) *Evaluator {
	options := EvalOptions{
		Caching:     false,              // Disabled by default
		Concurrency: defaultConcurrency, // false on WASM targets
		MaxDepth:    10000,
		Timeout:     30 * time.Second,
	}

	for _, opt := range opts {
		opt(&options)
	}

	if options.Logger == nil {
		options.Logger = slog.Default()
	}

	// Initialise expression cache when caching is enabled.
	var c *cache.Cache
	if options.Cache != nil {
		c = options.Cache
	} else if options.Caching {
		size := options.CacheSize
		if size <= 0 {
			size = 256
		}
		c = cache.New(size)
	}

	// Build custom function lookup map.
	customFns := make(map[string]*FunctionDef, len(options.CustomFunctions))
	for _, cfd := range options.CustomFunctions {
		// Capture loop variable.
		cfd := cfd
		customFns[cfd.Name] = &FunctionDef{
			Name:    cfd.Name,
			MinArgs: 0,
			MaxArgs: -1, // unlimited; type-checking done via Signature if set
			Impl: func(ctx context.Context, _ *Evaluator, _ *EvalContext, args []interface{}) (interface{}, error) {
				return cfd.Fn(ctx, args...)
			},
		}
	}

	return &Evaluator{
		opts:      options,
		logger:    options.Logger,
		cache:     c,
		customFns: customFns,
	}
}

// Cache returns the expression cache, or nil if caching is disabled.
func (e *Evaluator) Cache() *cache.Cache {
	return e.cache
}

// getCustomFunction returns a user-defined custom function by name, or (nil, false).
func (e *Evaluator) getCustomFunction(name string) (*FunctionDef, bool) {
	if len(e.customFns) == 0 {
		return nil, false
	}
	fn, ok := e.customFns[name]
	return fn, ok
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

	// Initialise a shared depth counter for this evaluation tree.
	// evalNode increments/decrements it on every node visit (stack-style),
	// matching the JSONata JS test runner's timeboxExpression semantics.
	if e.opts.MaxDepth > 0 {
		ctx = withNewRecurseDepthPtr(ctx)
	}

	// Evaluate the AST
	result, err := e.evalNode(ctx, expr.AST(), evalCtx)
	if err != nil {
		return nil, err
	}

	// Convert types.Null to nil before returning
	result = e.convertNullToNil(result)

	// Unwrap any contextBoundValues that escaped to the top level
	// (e.g. from @$var or #$var expressions at the end of a path with no further steps)
	result = unwrapCVsDeep(result)

	// NOTE: Singleton-sequence unwrapping (returning the single item of a 1-element
	// collection instead of the collection itself) is intentionally NOT done here.
	// Each evaluator that builds a sequence (evalPath, evalNameString on an array
	// context) already performs the unwrap at the right level.
	// Doing it here too would incorrectly unwrap actual array VALUES that happen to
	// have one element (e.g. a JSON field whose value is ["Account"]) — those must
	// be returned as-is, just like JSONata JS does.

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

	// Initialise a shared depth counter for this evaluation tree.
	if e.opts.MaxDepth > 0 {
		ctx = withNewRecurseDepthPtr(ctx)
	}

	// Evaluate the AST
	result, err := e.evalNode(ctx, expr.AST(), evalCtx)
	if err != nil {
		return nil, err
	}

	// Convert types.Null to nil before returning
	result = e.convertNullToNil(result)

	// Unwrap any contextBoundValues that escaped to the top level
	result = unwrapCVsDeep(result)

	return result, nil
}

// EvalOption configures evaluation behavior.
type EvalOption func(*EvalOptions)

// WithCaching enables or disables expression compilation caching.
// When enabled, a default LRU cache of 256 entries is created.
// To control the cache size use WithCacheSize; to supply your own cache use WithCache.
func WithCaching(enabled bool) EvalOption {
	return func(opts *EvalOptions) {
		opts.Caching = enabled
	}
}

// WithCacheSize sets the maximum number of cached expressions.
// Only effective when combined with WithCaching(true).
func WithCacheSize(size int) EvalOption {
	return func(opts *EvalOptions) {
		opts.CacheSize = size
	}
}

// WithCache attaches an external expression cache.
// The evaluator will use this cache regardless of the Caching flag.
func WithCache(c *cache.Cache) EvalOption {
	return func(opts *EvalOptions) {
		opts.Cache = c
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

// WithCustomFunction registers a user-defined function with the evaluator.
// name is the function name without the leading "$" (the expression must use "$name" to call it).
// signature is an optional JSONata type-signature string (e.g. "<s:s>") — pass "" to skip.
// fn is the implementation.
//
// Example:
//
//	gosonata.Eval(`$greet(name)`, data, gosonata.WithCustomFunction("greet", "", func(ctx context.Context, args ...interface{}) (interface{}, error) {
//	    return "Hello, " + args[0].(string) + "!", nil
//	}))
func WithCustomFunction(name, signature string, fn functions.CustomFunc) EvalOption {
	return func(opts *EvalOptions) {
		opts.CustomFunctions = append(opts.CustomFunctions, functions.CustomFunctionDef{
			Name:      name,
			Signature: signature,
			Fn:        fn,
		})
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
