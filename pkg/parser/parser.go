package parser

// Package parser implements a high-performance JSONata 2.1.0+ parser.
//
// The parser uses a hand-written recursive descent approach for maximum
// performance and control. It supports incremental parsing for streaming
// scenarios and provides detailed error reporting with source positions.
//
// # Architecture
//
// The parser consists of three main components:
//   - Lexer: Tokenizes the input expression into a stream of tokens
//   - Parser: Builds an Abstract Syntax Tree (AST) from tokens
//   - Error Recovery: Optional mode for parsing malformed expressions
//
// # Example
//
//	expr, err := parser.Parse("$.items[price > 100]")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	ast := expr.AST()
//
// # Performance
//
// The parser is optimized for:
//   - Low memory allocation
//   - Fast tokenization
//   - Efficient AST construction
//   - Minimal garbage collection pressure

import (
	"github.com/sandrolain/gosonata/pkg/types"
)

// Parse parses a JSONata expression and returns the compiled Expression.
//
// The function tokenizes the input, builds an AST, and validates the syntax.
// If parsing fails, it returns a detailed error with position information.
//
// Example:
//
//	expr, err := parser.Parse("$.name")
//	if err != nil {
//	    fmt.Printf("Parse error at position %d\n", err.Position)
//	    return
//	}
func Parse(query string) (*types.Expression, error) {
	p := NewParser(query)
	return p.Parse()
}

// Compile is an alias for Parse, provided for API consistency.
func Compile(query string, opts ...CompileOption) (*types.Expression, error) {
	p := NewParser(query, opts...)
	return p.Parse()
}

// CompileOption configures compilation behavior.
type CompileOption func(*CompileOptions)

// CompileOptions holds parser configuration.
type CompileOptions struct {
	// EnableRecovery enables error recovery mode for parsing invalid syntax.
	EnableRecovery bool
	// MaxDepth limits recursion depth to prevent stack overflow.
	MaxDepth int
}

// WithRecovery enables error recovery mode.
func WithRecovery(enable bool) CompileOption {
	return func(opts *CompileOptions) {
		opts.EnableRecovery = enable
	}
}

// WithMaxDepth sets the maximum parsing depth.
func WithMaxDepth(depth int) CompileOption {
	return func(opts *CompileOptions) {
		opts.MaxDepth = depth
	}
}
