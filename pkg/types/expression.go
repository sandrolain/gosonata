// Package types defines the core type system for GoSonata.
//
// This package contains type definitions for:
//   - Expression: Compiled JSONata expressions
//   - ASTNode: Abstract Syntax Tree nodes
//   - Value: Runtime values with type information
//   - Sequence: JSONata sequence type
//   - Lambda: Lambda function type
//   - Error types: Structured errors with codes
package types

import (
	"context"
	"fmt"
)

// Expression represents a compiled JSONata expression.
//
// An Expression can be evaluated multiple times against different data.
// It is safe for concurrent use by multiple goroutines.
type Expression struct {
	ast    *ASTNode
	source string
	errors []error
}

// NewExpression creates a new Expression from an AST.
func NewExpression(ast *ASTNode, source string) *Expression {
	return &Expression{
		ast:    ast,
		source: source,
	}
}

// AST returns the Abstract Syntax Tree of the expression.
func (e *Expression) AST() *ASTNode {
	return e.ast
}

// Source returns the original source code of the expression.
func (e *Expression) Source() string {
	return e.source
}

// Errors returns any errors collected during parsing (in recovery mode).
func (e *Expression) Errors() []error {
	return e.errors
}

// AddError adds an error to the expression's error list.
func (e *Expression) AddError(err error) {
	e.errors = append(e.errors, err)
}

// Eval evaluates the expression against the provided data.
//
// The context is used for timeout and cancellation. If the context
// is canceled or times out, Eval returns an error.
//
// Example:
//
//	result, err := expr.Eval(ctx, data)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (e *Expression) Eval(ctx context.Context, data interface{}) (interface{}, error) {
	// This will be implemented by calling the evaluator
	// For now, return an error to avoid import cycle
	// The actual implementation will be in the evaluator package
	return nil, fmt.Errorf("use evaluator.Eval() to evaluate expressions")
}

// EvalWithBindings evaluates the expression with custom variable bindings.
func (e *Expression) EvalWithBindings(ctx context.Context, data interface{}, bindings map[string]interface{}) (interface{}, error) {
	// Same as above - implementation in evaluator package
	return nil, fmt.Errorf("use evaluator.EvalWithBindings() to evaluate expressions with bindings")
}

// String returns a string representation of the expression.
func (e *Expression) String() string {
	return e.source
}
