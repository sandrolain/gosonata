package evaluator

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strconv"

	"github.com/sandrolain/gosonata/pkg/types"
)

func (e *Evaluator) evalBinary(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	op := node.Value.(string)

	// OPT-01: Binary operands are never in tail position. Clear the TCO flag on the shared
	// EvalContext (cheap bool field write) instead of wrapping context with context.WithValue.
	// Restore on return so callers that checked tcoTail before dispatching here still see
	// the original value (e.g. a block whose last expr is a binary op).
	prevTCO := evalCtx.tcoTail
	evalCtx.tcoTail = false
	defer func() { evalCtx.tcoTail = prevTCO }()

	// Handle special operators
	switch op {
	case "and":
		return e.evalAnd(ctx, node, evalCtx)
	case "or":
		return e.evalOr(ctx, node, evalCtx)
	case "??":
		return e.evalCoalesce(ctx, node, evalCtx)
	case "?:":
		return e.evalDefault(ctx, node, evalCtx)
	case "..":
		return e.evalRange(ctx, node, evalCtx)
	case "~>":
		return e.evalApply(ctx, node, evalCtx)
	}

	// Evaluate both sides
	left, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	right, err := e.evalNode(ctx, node.RHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// Unwrap contextBoundValues: CVs must not be visible to operators
	left = unwrapCVsDeep(left)
	right = unwrapCVsDeep(right)

	// Fast-path for the most common case: both operands are float64.
	// Avoids the toNumber() type-assertion chain and generic switch below.
	// Arithmetic ops include an inline overflow/NaN guard equivalent to checkArithmeticResult.
	if lf, ok := left.(float64); ok {
		if rf, ok := right.(float64); ok {
			switch op {
			case "+":
				r := lf + rf
				if math.IsNaN(r) || math.IsInf(r, 0) {
					return nil, types.NewError(types.ErrNumberTooLarge, "number out of range", -1)
				}
				return r, nil
			case "-":
				r := lf - rf
				if math.IsNaN(r) || math.IsInf(r, 0) {
					return nil, types.NewError(types.ErrNumberTooLarge, "number out of range", -1)
				}
				return r, nil
			case "*":
				r := lf * rf
				if math.IsNaN(r) || math.IsInf(r, 0) {
					return nil, types.NewError(types.ErrNumberTooLarge, "number out of range", -1)
				}
				return r, nil
			case "/":
				if rf == 0 {
					return nil, types.NewError(types.ErrNumberTooLarge, "division by zero", -1)
				}
				r := lf / rf
				if math.IsNaN(r) || math.IsInf(r, 0) {
					return nil, types.NewError(types.ErrNumberTooLarge, "number out of range", -1)
				}
				return r, nil
			case "%":
				if rf == 0 {
					return nil, types.NewError(types.ErrNumberTooLarge, "division by zero", -1)
				}
				r := math.Mod(lf, rf)
				if math.IsNaN(r) || math.IsInf(r, 0) {
					return nil, types.NewError(types.ErrNumberTooLarge, "number out of range", -1)
				}
				return r, nil
			case "=":
				return lf == rf, nil
			case "!=":
				return lf != rf, nil
			case "<":
				return lf < rf, nil
			case "<=":
				return lf <= rf, nil
			case ">":
				return lf > rf, nil
			case ">=":
				return lf >= rf, nil
			}
		}
	}

	// Apply operator
	switch op {
	case "+":
		return e.opAdd(left, right)
	case "-":
		return e.opSubtract(left, right)
	case "*":
		return e.opMultiply(left, right)
	case "/":
		return e.opDivide(left, right)
	case "%":
		return e.opModulo(left, right)
	case "=":
		return e.opEqual(left, right), nil
	case "!=":
		return !e.opEqual(left, right), nil
	case "<":
		return e.opLess(left, right)
	case "<=":
		return e.opLessEqual(left, right)
	case ">":
		return e.opGreater(left, right)
	case ">=":
		return e.opGreaterEqual(left, right)
	case "&":
		return e.opConcat(left, right)
	case "in":
		return e.opIn(left, right)
	default:
		return nil, fmt.Errorf("unsupported binary operator: %s", op)
	}
}

// evalUnary evaluates a unary operator expression.

func (e *Evaluator) evalUnary(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	op := node.Value.(string)

	// Evaluate operand
	operand, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	switch op {
	case "-":
		return e.opNegate(operand)
	default:
		return nil, fmt.Errorf("unsupported unary operator: %s", op)
	}
}

// evalPathInfixObjectConstructor handles infix object constructor when applied to array via path.
// For each item in the path-evaluated array:
// 1. Evaluate node.LHS (e.g., "Product") to get sub-collection
// 2. Group sub-collection items by key
// 3. Merge all individual results into a final merged object

func requireNumericOperand(value interface{}) error {
	if value == nil {
		return nil // nil (undefined) is allowed - propagates
	}
	switch value.(type) {
	case float64, int:
		return nil // numeric types OK
	default:
		return types.NewError(types.ErrLeftSideAssignment, fmt.Sprintf("left %T operand of arithmetic operation must be a number", value), -1)
	}
}

// checkArithmeticResult validates that an arithmetic result is a finite number.
// Returns D1001 error for NaN, +Infinity, or -Infinity.

func checkArithmeticResult(result float64) error {
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return types.NewError(types.ErrNumberTooLarge, "number out of range", -1)
	}
	return nil
}

func (e *Evaluator) opAdd(left, right interface{}) (interface{}, error) {
	// Validate operand types (T2001 for non-numeric non-nil)
	if err := requireNumericOperand(left); err != nil {
		return nil, err
	}
	if err := requireNumericOperand(right); err != nil {
		return nil, err
	}
	// Propagate undefined
	if left == nil || right == nil {
		return nil, nil
	}
	l, _ := e.toNumber(left)
	r, _ := e.toNumber(right)
	result := l + r
	if err := checkArithmeticResult(result); err != nil {
		return nil, err
	}
	return result, nil
}

func (e *Evaluator) opSubtract(left, right interface{}) (interface{}, error) {
	// Validate operand types (T2001 for non-numeric non-nil)
	if err := requireNumericOperand(left); err != nil {
		return nil, err
	}
	if err := requireNumericOperand(right); err != nil {
		return nil, err
	}
	// Propagate undefined
	if left == nil || right == nil {
		return nil, nil
	}
	l, _ := e.toNumber(left)
	r, _ := e.toNumber(right)
	result := l - r
	if err := checkArithmeticResult(result); err != nil {
		return nil, err
	}
	return result, nil
}

func (e *Evaluator) opMultiply(left, right interface{}) (interface{}, error) {
	// Validate operand types (T2001 for non-numeric non-nil)
	if err := requireNumericOperand(left); err != nil {
		return nil, err
	}
	if err := requireNumericOperand(right); err != nil {
		return nil, err
	}
	// Propagate undefined
	if left == nil || right == nil {
		return nil, nil
	}
	l, _ := e.toNumber(left)
	r, _ := e.toNumber(right)
	result := l * r
	if err := checkArithmeticResult(result); err != nil {
		return nil, err
	}
	return result, nil
}

func (e *Evaluator) opDivide(left, right interface{}) (interface{}, error) {
	// Validate operand types (T2001 for non-numeric non-nil)
	if err := requireNumericOperand(left); err != nil {
		return nil, err
	}
	if err := requireNumericOperand(right); err != nil {
		return nil, err
	}
	// Propagate undefined
	if left == nil || right == nil {
		return nil, nil
	}
	l, _ := e.toNumber(left)
	r, _ := e.toNumber(right)
	if r == 0 {
		return nil, types.NewError(types.ErrNumberTooLarge, "division by zero", -1)
	}
	result := l / r
	if err := checkArithmeticResult(result); err != nil {
		return nil, err
	}
	return result, nil
}

func (e *Evaluator) opModulo(left, right interface{}) (interface{}, error) {
	// Validate operand types (T2001 for non-numeric non-nil)
	if err := requireNumericOperand(left); err != nil {
		return nil, err
	}
	if err := requireNumericOperand(right); err != nil {
		return nil, err
	}
	// Propagate undefined
	if left == nil || right == nil {
		return nil, nil
	}
	l, _ := e.toNumber(left)
	r, _ := e.toNumber(right)
	if r == 0 {
		return nil, fmt.Errorf("modulo by zero")
	}
	result := math.Mod(l, r)
	if err := checkArithmeticResult(result); err != nil {
		return nil, err
	}
	return result, nil
}

func (e *Evaluator) opNegate(operand interface{}) (interface{}, error) {
	// Propagate undefined
	if operand == nil {
		return nil, nil
	}
	n, err := e.toNumber(operand)
	if err != nil {
		return nil, err
	}
	return -n, nil
}

// Comparison operators

func (e *Evaluator) opEqual(left, right interface{}) bool {
	// Handle nil
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	// Handle JSONata null
	if _, ok := left.(types.Null); ok {
		_, rightIsNull := right.(types.Null)
		return rightIsNull
	}
	if _, ok := right.(types.Null); ok {
		return false
	}

	// Handle bool explicitly (before numeric conversion)
	// This ensures bool values are treated correctly: true != 1 in JSON terms
	lBool, lIsBool := left.(bool)
	rBool, rIsBool := right.(bool)
	if lIsBool && rIsBool {
		return lBool == rBool
	}
	// But bool can equal numbers when converted: true == 1, false == 0
	if lIsBool {
		if rNum, rIsNum := e.tryNumber(right); rIsNum {
			if lBool {
				return 1.0 == rNum
			}
			return 0.0 == rNum
		}
	}
	if rIsBool {
		if lNum, lIsNum := e.tryNumber(left); lIsNum {
			if rBool {
				return lNum == 1.0
			}
			return lNum == 0.0
		}
	}

	// Try numeric comparison for non-bool numbers
	lNum, lIsNum := e.tryNumber(left)
	rNum, rIsNum := e.tryNumber(right)
	if lIsNum && rIsNum {
		return lNum == rNum
	}

	// Fall back to deep equal for other types
	return reflect.DeepEqual(left, right)
}

func (e *Evaluator) opLess(left, right interface{}) (interface{}, error) {
	if _, ok := left.(types.Null); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	if _, ok := right.(types.Null); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	if _, ok := left.(bool); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	if _, ok := right.(bool); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	// Handle nil - comparing with undefined returns undefined
	if left == nil || right == nil {
		return nil, nil
	}

	// Check if both are numbers
	lNum, lIsNum := e.tryNumber(left)
	rNum, rIsNum := e.tryNumber(right)
	if lIsNum && rIsNum {
		return lNum < rNum, nil
	}

	// Check if both are strings
	lStr, lIsStr := left.(string)
	rStr, rIsStr := right.(string)
	if lIsStr && rIsStr {
		return lStr < rStr, nil
	}

	// Type mismatch
	return nil, fmt.Errorf("T2009: Cannot compare %T with %T", left, right)
}

func (e *Evaluator) opLessEqual(left, right interface{}) (interface{}, error) {
	if _, ok := left.(types.Null); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	if _, ok := right.(types.Null); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	if _, ok := left.(bool); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	if _, ok := right.(bool); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	// Handle nil - comparing with undefined returns undefined
	if left == nil || right == nil {
		return nil, nil
	}

	// Check if both are numbers
	lNum, lIsNum := e.tryNumber(left)
	rNum, rIsNum := e.tryNumber(right)
	if lIsNum && rIsNum {
		return lNum <= rNum, nil
	}

	// Check if both are strings
	lStr, lIsStr := left.(string)
	rStr, rIsStr := right.(string)
	if lIsStr && rIsStr {
		return lStr <= rStr, nil
	}

	// Type mismatch
	return nil, fmt.Errorf("T2009: Cannot compare %T with %T", left, right)
}

func (e *Evaluator) opGreater(left, right interface{}) (interface{}, error) {
	if _, ok := left.(types.Null); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	if _, ok := right.(types.Null); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	if _, ok := left.(bool); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	if _, ok := right.(bool); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	// Handle nil - comparing with undefined returns undefined
	if left == nil || right == nil {
		return nil, nil
	}

	// Check if both are numbers
	lNum, lIsNum := e.tryNumber(left)
	rNum, rIsNum := e.tryNumber(right)
	if lIsNum && rIsNum {
		return lNum > rNum, nil
	}

	// Check if both are strings
	lStr, lIsStr := left.(string)
	rStr, rIsStr := right.(string)
	if lIsStr && rIsStr {
		return lStr > rStr, nil
	}

	// Type mismatch
	return nil, fmt.Errorf("T2009: Cannot compare %T with %T", left, right)
}

func (e *Evaluator) opGreaterEqual(left, right interface{}) (interface{}, error) {
	if _, ok := left.(types.Null); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	if _, ok := right.(types.Null); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	if _, ok := left.(bool); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	if _, ok := right.(bool); ok {
		return nil, fmt.Errorf("T2010: Cannot compare %T with %T", left, right)
	}
	// Handle nil - comparing with undefined returns undefined
	if left == nil || right == nil {
		return nil, nil
	}

	// Check if both are numbers
	lNum, lIsNum := e.tryNumber(left)
	rNum, rIsNum := e.tryNumber(right)
	if lIsNum && rIsNum {
		return lNum >= rNum, nil
	}

	// Check if both are strings
	lStr, lIsStr := left.(string)
	rStr, rIsStr := right.(string)
	if lIsStr && rIsStr {
		return lStr >= rStr, nil
	}

	// Type mismatch
	return nil, fmt.Errorf("T2009: Cannot compare %T with %T", left, right)
}

// String operator

func (e *Evaluator) opConcat(left, right interface{}) (interface{}, error) {
	l := e.toString(left)
	r := e.toString(right)
	return l + r, nil
}

// In operator

func (e *Evaluator) opIn(left, right interface{}) (interface{}, error) {
	// Convert right to array
	arr, err := e.toArray(right)
	if err != nil {
		return nil, err
	}

	// Check if left is in array
	for _, item := range arr {
		if e.opEqual(left, item) {
			return true, nil
		}
	}

	return false, nil
}

// evalParent evaluates the parent operator (%).
// Returns the parent context's data.

func (e *Evaluator) evalParent(node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Walk up context chain looking for a context that was created as an array item.
	// Only valid when inside a path that iterates over array elements.
	for ctx := evalCtx; ctx != nil; ctx = ctx.Parent() {
		if ctx.IsArrayItem() {
			// The parent of the array item context is the containing object
			if ctx.Parent() != nil {
				return ctx.Parent().Data(), nil
			}
			return nil, nil
		}
	}
	// No array iteration context found — % is invalid here
	return nil, types.NewError(types.ErrInvalidParentUse, "The % operator can only be used within a path that is a member of an array", node.Position)
}

// evalContextBind evaluates the context variable binding operator (@$var).
// Semantics (from JSONata spec):
//   - Evaluates LHS to get a sequence of items.
//   - Each item is bound to $var.
//   - The PARENT context (the data from which LHS was resolved) BECOMES the new current context
//     for subsequent path steps.  This enables cross-collection joins.

func compareValues(left, right interface{}) int {
	// Nil values are treated as equal to each other and less than non-nil
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return -1
	}
	if right == nil {
		return 1
	}

	// Try numeric comparison
	lNum, lIsNum := tryNumber(left)
	rNum, rIsNum := tryNumber(right)
	if lIsNum && rIsNum {
		if lNum < rNum {
			return -1
		} else if lNum > rNum {
			return 1
		}
		return 0
	}

	// Try string comparison
	lStr, lIsStr := left.(string)
	rStr, rIsStr := right.(string)
	if lIsStr && rIsStr {
		if lStr < rStr {
			return -1
		} else if lStr > rStr {
			return 1
		}
		return 0
	}

	// Try boolean comparison (false < true)
	lBool, lIsBool := left.(bool)
	rBool, rIsBool := right.(bool)
	if lIsBool && rIsBool {
		if !lBool && rBool {
			return -1 // false < true
		} else if lBool && !rBool {
			return 1 // true > false
		}
		return 0
	}

	// Arrays and objects are compared by identity
	// If they're the same object, they're equal
	// Otherwise, we use a stable sort (don't reorder)
	return 0
}

// tryNumber attempts to convert a value to a float64.
// Returns the number and a boolean indicating success.
// This is a helper function used for numeric comparisons.

func tryNumber(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case string:
		// Try to parse string as number
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
