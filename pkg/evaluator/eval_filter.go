package evaluator

import (
	"context"
	"sort"

	"github.com/sandrolain/gosonata/pkg/types"
)

func (e *Evaluator) evalFilter(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Evaluate the collection
	collection, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	if collection == nil {
		return nil, nil
	}

	// Check for empty filter (RHS == nil) - means flatten/iterate all items
	if node.RHS == nil {
		// $[] means return all items as array
		arr, err := e.toArray(collection)
		if err != nil {
			return nil, err
		}
		if len(arr) == 0 {
			return nil, nil
		}
		// Always return as array when using [] syntax (node.KeepArray is set by parser)
		return arr, nil
	}

	// Check if RHS is a direct number (index access) without evaluating variables
	// This avoids evaluating predicates like [$<=3] with wrong context
	if node.RHS.Type == types.NodeNumber {
		indexFloat, ok := node.RHS.Value.(float64)
		if ok {
			// Get array from collection
			arr, err := e.toArray(collection)
			if err != nil {
				return nil, err
			}

			index := int(indexFloat)

			// Handle negative indices (from end)
			if index < 0 {
				index = len(arr) + index
			}

			// Check bounds
			if index < 0 || index >= len(arr) {
				return nil, nil
			}

			return arr[index], nil
		}
	}

	// For expressions that might be indices (like variables, unary minus, etc.)
	// Try to evaluate as number and use as index
	// This handles cases like [-1], [$i], etc.
	rhsValue, err := e.evalNode(ctx, node.RHS, evalCtx)
	if err == nil {
		if indexFloat, ok := rhsValue.(float64); ok {
			// Get array from collection
			arr, err := e.toArray(collection)
			if err != nil {
				return nil, err
			}

			index := int(indexFloat)

			// Handle negative indices (from end)
			if index < 0 {
				index = len(arr) + index
			}

			// Check bounds
			if index < 0 || index >= len(arr) {
				return nil, nil
			}

			return arr[index], nil
		}

		// Handle multi-index selection: when filter evaluates to an array of numbers
		// e.g., arr[[1..3,8,-1]] selects elements at multiple indices
		// Indices are applied in sorted order (i.e., result is in original array order)
		if indices, ok := rhsValue.([]interface{}); ok {
			allNumbers := true
			for _, idx := range indices {
				if _, isNum := idx.(float64); !isNum {
					allNumbers = false
					break
				}
			}
			if allNumbers {
				arr, err := e.toArray(collection)
				if err != nil {
					return nil, err
				}
				// Collect resolved indices (handling negatives), sort them
				resolvedIndices := make([]int, 0, len(indices))
				for _, idx := range indices {
					index := int(idx.(float64))
					if index < 0 {
						index = len(arr) + index
					}
					if index >= 0 && index < len(arr) {
						resolvedIndices = append(resolvedIndices, index)
					}
				}
				// Sort indices to preserve original array order
				sort.Ints(resolvedIndices)
				// Build result in sorted index order
				result := make([]interface{}, 0, len(resolvedIndices))
				for _, index := range resolvedIndices {
					result = append(result, arr[index])
				}
				if len(result) == 0 {
					return nil, nil
				}
				return result, nil
			}
		}
	}

	// Check if collection is an array
	arr, isArray := collection.([]interface{})
	if !isArray {
		// If not an array, treat filter as conditional
		// Handle contextBoundValue transparently
		actualCollection, collBindings := extractBoundItem(collection)
		objCtx := evalCtx.NewChildContext(actualCollection)
		if len(collBindings) > 0 {
			applyBindingsToCtx(objCtx, collBindings)
		}
		match, err := e.evalNode(ctx, node.RHS, objCtx)
		if err != nil {
			return nil, err
		}

		// If predicate is true, return the (original) object; otherwise nil
		if e.isTruthy(match) {
			return collection, nil
		}
		return nil, nil
	}

	// Otherwise treat as array filter predicate
	result := make([]interface{}, 0)
	for _, item := range arr {
		// Extract value and bindings from contextBoundValue if present
		actualItem, inheritedBindings := extractBoundItem(item)

		// Create context with item as data
		itemCtx := evalCtx.NewChildContext(actualItem)
		if len(inheritedBindings) > 0 {
			applyBindingsToCtx(itemCtx, inheritedBindings)
		}

		// Evaluate filter expression
		match, err := e.evalNode(ctx, node.RHS, itemCtx)
		if err != nil {
			return nil, err
		}

		// Check if matches (truthy value)
		if e.isTruthy(match) {
			result = append(result, item) // keep original item (may be a cv)
		}
	}

	// Empty result returns nil
	if len(result) == 0 {
		return nil, nil
	}

	return result, nil
}

// evalCondition evaluates a conditional expression.

func (e *Evaluator) evalCondition(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Condition must NOT be in tail position - only branches can be tail.
	condCtx := withoutTCOTail(ctx)

	// Evaluate condition
	condition, err := e.evalNode(condCtx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// Check if condition is truthy
	if e.isTruthy(condition) {
		// Evaluate then branch (propagate tail position)
		return e.evalNode(ctx, node.RHS, evalCtx)
	}

	// Evaluate else branch (propagate tail position)
	if len(node.Expressions) > 0 && node.Expressions[0] != nil {
		return e.evalNode(ctx, node.Expressions[0], evalCtx)
	}

	return nil, nil
}

// evalFunction evaluates a function call.
