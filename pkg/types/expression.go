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

// Expression represents a compiled JSONata expression.
//
// An Expression can be evaluated multiple times against different data
// by passing it to [evaluator.Evaluator.Eval]. It is safe for concurrent use
// by multiple goroutines.
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

// String returns a string representation of the expression.
func (e *Expression) String() string {
	return e.source
}
