package evaluator

import (
	"context"

	"github.com/sandrolain/gosonata/pkg/types"
)

func (e *Evaluator) evalContextBind(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	varName := node.RHS.Value.(string) // e.g. "l" for @$l

	// Determine parent data for the rewind:
	// - When LHS is a path (A.B), parent = value of A evaluated in evalCtx
	// - Otherwise, parent = evalCtx.Data() itself
	var parentData interface{}
	if node.LHS.Type == types.NodePath && node.LHS.LHS != nil {
		var err error
		parentData, err = e.evalNode(ctx, node.LHS.LHS, evalCtx)
		if err != nil {
			return nil, err
		}
	} else {
		parentData = evalCtx.Data()
	}

	// Evaluate LHS to obtain the sequence of items
	left, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}
	if left == nil {
		return nil, nil
	}

	arr, err := e.toArray(left)
	if err != nil {
		return nil, err
	}
	if len(arr) == 0 {
		return nil, nil
	}

	result := make([]interface{}, 0, len(arr))
	for _, item := range arr {
		actualItem, existingBindings := extractBoundItem(item)

		// Effective parent: if the item itself carried a parent (from a previous @$),
		// use it; otherwise use the parent we just computed.
		effectiveParent := parentData
		if cv, ok := item.(*contextBoundValue); ok && cv.parent != nil {
			effectiveParent = cv.parent
		}

		// Build new bindings: inherit parent's, then bind varName → current item
		newBindings := make(map[string]interface{}, len(existingBindings)+1)
		for k, v := range existingBindings {
			newBindings[k] = v
		}
		newBindings[varName] = actualItem

		result = append(result, &contextBoundValue{
			value:    actualItem,      // original item (used for final output unwrapping)
			parent:   effectiveParent, // rewind-to context for the very next path step
			bindings: newBindings,
		})
	}

	if len(result) == 1 && !node.KeepArray {
		return result[0], nil
	}
	return result, nil
}

// evalIndexBind evaluates the positional variable binding operator (#$var).
// Semantics: for each item at position i in the input sequence, binds $var = i (0-based).
// The items themselves are unchanged; $var is available in subsequent filter/path steps.

func (e *Evaluator) evalIndexBind(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	varName := node.RHS.Value.(string) // e.g. "i" for #$i

	// Evaluate LHS to obtain the sequence of items
	left, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}
	if left == nil {
		return nil, nil
	}

	arr, err := e.toArray(left)
	if err != nil {
		return nil, err
	}
	if len(arr) == 0 {
		return nil, nil
	}

	result := make([]interface{}, 0, len(arr))
	for i, item := range arr {
		actualItem, existingBindings := extractBoundItem(item)
		var existingParent interface{}
		if cv, ok := item.(*contextBoundValue); ok {
			existingParent = cv.parent
		}

		// Merge existing with position binding
		newBindings := make(map[string]interface{}, len(existingBindings)+1)
		for k, v := range existingBindings {
			newBindings[k] = v
		}
		newBindings[varName] = float64(i)

		result = append(result, &contextBoundValue{
			value:    actualItem,
			parent:   existingParent,
			bindings: newBindings,
		})
	}

	if len(result) == 1 && !node.KeepArray {
		return result[0], nil
	}
	return result, nil
}

// compareValues compares two values and returns:
// -1 if left < right
//
//	0 if left == right
//	1 if left > right
//
// This is used for sorting operations.
