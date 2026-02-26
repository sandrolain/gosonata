// Package functions provides types for registering custom JSONata functions.
//
// Users of GoSonata can define their own functions and register them via
// [gosonata.WithCustomFunction], making them available inside JSONata expressions
// with the "$" prefix.
//
// # Example
//
//	result, err := gosonata.Eval(`$greet(name)`,
//	    map[string]interface{}{"name": "World"},
//	    gosonata.WithCustomFunction("greet", "<s:s>", func(ctx context.Context, args ...interface{}) (interface{}, error) {
//	        return "Hello, " + args[0].(string) + "!", nil
//	    }),
//	)
//	// result == "Hello, World!"
package functions

import "context"

// CustomFunc is the signature for user-defined custom functions.
// args contains the evaluated function arguments in order.
// The function should return a JSON-compatible value or an error.
type CustomFunc func(ctx context.Context, args ...interface{}) (interface{}, error)

// CustomFunctionDef describes a user-defined function together with its
// optional JSONata type-signature string (e.g. "<s-s:s>").
// An empty Signature string disables type-checking.
type CustomFunctionDef struct {
	// Name is the function name as it will appear inside expressions (without the "$" prefix).
	Name string
	// Signature is the optional JSONata function signature used for argument type-checking.
	// Leave empty to skip type validation.
	Signature string
	// Fn is the implementation.
	Fn CustomFunc
}

// Caller can invoke a JSONata function value (lambda or built-in) that was
// passed as an argument. It is provided to AdvancedCustomFunc implementations
// so they can call back into the evaluator for higher-order functions.
type Caller interface {
	// Call invokes fn (a *Lambda, *FunctionDef or any other callable value
	// that the evaluator recognizes) with the supplied args.
	Call(ctx context.Context, fn interface{}, args ...interface{}) (interface{}, error)
}

// AdvancedCustomFunc is like CustomFunc but also receives a Caller so the
// implementation can invoke function values passed as arguments (e.g. for
// higher-order functions like $groupBy, $mapValues, $pipe).
type AdvancedCustomFunc func(ctx context.Context, caller Caller, args ...interface{}) (interface{}, error)

// AdvancedCustomFunctionDef is the struct counterpart of AdvancedCustomFunc.
type AdvancedCustomFunctionDef struct {
	// Name is the function name without the "$" prefix.
	Name string
	// Signature is the optional JSONata type-signature string.
	Signature string
	// Fn is the implementation.
	Fn AdvancedCustomFunc
}

// FunctionEntry is a common marker interface implemented by both
// [CustomFunctionDef] and [AdvancedCustomFunctionDef].
// It allows mixing both kinds in a single variadic call to [WithFunctions].
type FunctionEntry interface {
	isFunctionEntry()
}

func (c CustomFunctionDef) isFunctionEntry()         {}
func (a AdvancedCustomFunctionDef) isFunctionEntry() {}
