package evaluator

import (
	"context"
	"math"

	"github.com/sandrolain/gosonata/pkg/types"
)

func (e *Evaluator) evalBind(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Evaluate the value
	value, err := e.evalNode(ctx, node.RHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// Set the binding
	varName := node.Value.(string)
	evalCtx.SetBinding(varName, value)

	return value, nil
}

// evalBlock evaluates a sequence of expressions (using semicolon operator).
// The result is the result of the last expression in the sequence.
// Each block creates a new scope for variable bindings.

func (e *Evaluator) evalBlock(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	if len(node.Expressions) == 0 {
		return nil, nil
	}

	// Create a child context for the block scope
	// This ensures variable bindings are local to this block
	blockCtx := &EvalContext{
		data:     evalCtx.Data(),
		parent:   evalCtx,
		bindings: make(map[string]interface{}),
		depth:    evalCtx.Depth() + 1,
	}

	var result interface{}
	var err error

	// Evaluate all expressions in sequence
	for i, expr := range node.Expressions {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result, err = e.evalNode(ctx, expr, blockCtx)
		if err != nil {
			return nil, err
		}

		// For all but the last expression, we only care about side effects (bindings)
		// The result is only the value from the last expression
		_ = i // All expressions are evaluated, their side effects matter
	}

	return result, nil
}

// evalAnd evaluates logical AND (short-circuit).

func (e *Evaluator) evalAnd(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	left, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	if !e.isTruthy(left) {
		return false, nil
	}

	right, err := e.evalNode(ctx, node.RHS, evalCtx)
	if err != nil {
		return nil, err
	}

	return e.isTruthy(right), nil
}

// evalOr evaluates logical OR (short-circuit).

func (e *Evaluator) evalOr(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	left, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	if e.isTruthy(left) {
		return true, nil
	}

	right, err := e.evalNode(ctx, node.RHS, evalCtx)
	if err != nil {
		return nil, err
	}

	return e.isTruthy(right), nil
}

// evalCoalesce evaluates null coalescing operator (??).
// Returns left value if defined (not nil), otherwise returns right value.
// Note: differs from default operator - null is considered a valid value.

func (e *Evaluator) evalCoalesce(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	left, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		// If left side errors, use right side
		right, err2 := e.evalNode(ctx, node.RHS, evalCtx)
		if err2 != nil {
			return nil, err2
		}
		return right, nil
	}

	// If left is not nil, return it (even if it's false, 0, empty string, etc.)
	if left != nil {
		return left, nil
	}

	// Left is nil/undefined, return right
	right, err := e.evalNode(ctx, node.RHS, evalCtx)
	if err != nil {
		return nil, err
	}

	return right, nil
}

// evalDefault evaluates the default operator (?:).
// Returns left value if it's truthy (not nil, not false, not 0, not empty string, etc.),
// otherwise returns right value.

func (e *Evaluator) evalDefault(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	left, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		// If left side errors, use right side
		right, err2 := e.evalNode(ctx, node.RHS, evalCtx)
		if err2 != nil {
			return nil, err2
		}
		return right, nil
	}

	// If left is truthy (using default operator semantics), return it
	if e.isTruthyForDefault(left) {
		return left, nil
	}

	// Left is falsy (nil, false, 0, empty string, array of falsy values, functions, etc.), return right
	right, err := e.evalNode(ctx, node.RHS, evalCtx)
	if err != nil {
		return nil, err
	}

	return right, nil
}

// evalRange evaluates a range expression.

func (e *Evaluator) evalRange(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Evaluate start
	startVal, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// Evaluate end
	endVal, err := e.evalNode(ctx, node.RHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// Validate types: only integer numbers are allowed in ranges
	// T2003: start must be integer number (if not nil)
	if startVal != nil {
		startFloat, startOk := startVal.(float64)
		if !startOk {
			return nil, types.NewError(types.ErrRangeStartNotInteger, "start of range expression must evaluate to an integer", -1)
		}
		if startFloat != math.Trunc(startFloat) {
			return nil, types.NewError(types.ErrRangeStartNotInteger, "start of range expression must evaluate to an integer", -1)
		}
	}

	// T2004: end must be integer number (if not nil)
	if endVal != nil {
		endFloat, endOk := endVal.(float64)
		if !endOk {
			return nil, types.NewError(types.ErrRangeEndNotInteger, "end of range expression must evaluate to an integer", -1)
		}
		if endFloat != math.Trunc(endFloat) {
			return nil, types.NewError(types.ErrRangeEndNotInteger, "end of range expression must evaluate to an integer", -1)
		}
	}

	// If either bound is undefined (nil), return empty array per JSONata spec
	if startVal == nil || endVal == nil {
		return []interface{}{}, nil
	}

	start := int64(startVal.(float64))
	end := int64(endVal.(float64))

	// Per JSONata spec: if start > end, range is empty
	if start > end {
		return []interface{}{}, nil
	}

	// D2014: range too large (> 10,000,000 elements)
	const maxRangeSize = 10_000_000
	if end-start >= maxRangeSize {
		return nil, types.NewError(types.ErrRangeTooLarge, "the size of the sequence allocated by the range expression exceeds the built-in limit", -1)
	}

	// Generate range (ascending only, start <= end guaranteed above)
	size := int(end-start) + 1
	result := make([]interface{}, size)
	for i := 0; i < size; i++ {
		result[i] = float64(start) + float64(i)
	}

	return result, nil
}

// evalApply evaluates an apply expression (~>).
// Syntax: expr ~> $function(args)
// The result of expr becomes the first argument to the function
// Special case: function ~> function creates function composition
