// Package functions implements all JSONata built-in functions.
//
// This package provides 66+ built-in functions organized by category:
//   - String functions (13): substring, uppercase, lowercase, etc.
//   - Numeric functions (12): sum, count, max, min, round, etc.
//   - Array functions (10): append, reverse, sort, etc.
//   - Aggregate functions (6): sum, average, min, max, etc.
//   - Higher-order functions (5): map, filter, reduce, etc.
//   - Boolean functions (3): boolean, not, exists
//   - Date/Time functions (7): now, fromMillis, formatDateTime, etc.
//   - Encoding functions (4): encodeUrl, decodeUrl, etc.
//   - Special functions (4): type, eval, assert, error
//
// # Example
//
//	registry := functions.DefaultRegistry()
//	fn, ok := registry.Lookup("$sum")
//	if ok {
//	    result, err := fn(ctx, []float64{1, 2, 3})
//	}
package functions

import (
	"context"
	"fmt"
)

// BuiltinFunc is the signature for built-in functions.
type BuiltinFunc func(ctx context.Context, args ...interface{}) (interface{}, error)

// FunctionRegistry manages built-in function registration and lookup.
type FunctionRegistry struct {
	functions  map[string]BuiltinFunc
	signatures map[string]string
}

// NewRegistry creates a new function registry.
func NewRegistry() *FunctionRegistry {
	return &FunctionRegistry{
		functions:  make(map[string]BuiltinFunc),
		signatures: make(map[string]string),
	}
}

// DefaultRegistry returns a registry with all built-in functions registered.
func DefaultRegistry() *FunctionRegistry {
	registry := NewRegistry()

	// TODO: Register all built-in functions in Phase 5
	// registerStringFunctions(registry)
	// registerNumericFunctions(registry)
	// registerArrayFunctions(registry)
	// registerAggregateFunctions(registry)
	// registerHigherOrderFunctions(registry)
	// registerBooleanFunctions(registry)
	// registerDateTimeFunctions(registry)
	// registerEncodingFunctions(registry)
	// registerSpecialFunctions(registry)

	return registry
}

// Register adds a function to the registry.
func (r *FunctionRegistry) Register(name string, fn BuiltinFunc, signature string) {
	r.functions[name] = fn
	r.signatures[name] = signature
}

// Lookup retrieves a function from the registry.
func (r *FunctionRegistry) Lookup(name string) (BuiltinFunc, bool) {
	fn, ok := r.functions[name]
	return fn, ok
}

// Signature returns the signature for a function.
func (r *FunctionRegistry) Signature(name string) (string, bool) {
	sig, ok := r.signatures[name]
	return sig, ok
}

// List returns all registered function names.
func (r *FunctionRegistry) List() []string {
	names := make([]string, 0, len(r.functions))
	for name := range r.functions {
		names = append(names, name)
	}
	return names
}

// Example placeholder function
func exampleFunction(ctx context.Context, args ...interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented yet")
}
