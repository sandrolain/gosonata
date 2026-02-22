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
