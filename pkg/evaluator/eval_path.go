package evaluator

import (
	"context"

	"github.com/sandrolain/gosonata/pkg/types"
)

func (e *Evaluator) evalPath(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Special case: if LHS is a string literal, treat it as a field name in current context
	var left interface{}
	var err error

	if node.LHS.Type == types.NodeString {
		// String literal as LHS means field name lookup in current context
		fieldName := node.LHS.Value.(string)
		left, err = e.evalNameString(fieldName, evalCtx)
	} else {
		// Normal evaluation of LHS
		left, err = e.evalNode(ctx, node.LHS, evalCtx)
	}

	if err != nil {
		return nil, err
	}

	// If left is nil, path evaluation stops
	if left == nil {
		return nil, nil
	}

	// Check if we should keep singleton arrays
	// This traverses the LHS chain to find any KeepArray flags
	keepArray := node.KeepArray || hasKeepArrayInChain(node.LHS)

	// Special case: if RHS is a prefix object constructor (expr.{...}),
	// do grouping evaluation instead of simple array map.
	// BUT only if there are no contextBoundValues (from @$ / #$ operators) in the array,
	// because those carry per-item bindings that must be applied during per-item evaluation.
	// Note: we intentionally do NOT optimize this into evalObjectGroupedWithArray
	// when the items might need % (parent) operator support -- let the normal
	// per-item loop handle those, so that each item's NewArrayItemContext provides
	// the correct parent chain for % to walk up.

	// Check if left is an array (or a single contextBoundValue) - JSONata applies path to each element
	// Unwrap a single contextBoundValue to allow the single-item path to work correctly
	if singleCV, isCV := left.(*contextBoundValue); isCV {
		left = []interface{}{singleCV}
	}

	if arr, ok := left.([]interface{}); ok {
		// Special case: if RHS is an infix object constructor (node.RHS.LHS != nil),
		// and no inherited bindings, apply the constructor to each item, then merge all results
		hasBindings := false
		for _, item := range arr {
			if _, ok := item.(*contextBoundValue); ok {
				hasBindings = true
				break
			}
		}
		if !hasBindings && node.RHS.Type == types.NodeObject && node.RHS.LHS != nil {
			return e.evalPathInfixObjectConstructor(ctx, node.RHS, arr, evalCtx)
		}

		// Apply path to each element of the array
		result := make([]interface{}, 0, len(arr))
		for _, item := range arr {
			// Extract value and bindings from contextBoundValue if present
			actualItem, inheritedBindings := extractBoundItem(item)
			// For @$var CVs, the parent field holds the rewound context for the next path step.
			// Use that as the execution context; for #$var or plain items, use the value itself.
			contextData := actualItem
			if cv, ok := item.(*contextBoundValue); ok && cv.parent != nil {
				contextData = cv.parent
			}

			// Create context with appropriate data for the next path step
			var itemCtx *EvalContext
			if cv, ok := item.(*contextBoundValue); ok && cv.parentObj != nil && cv.parent == nil {
				// This CV carries parent-object info for % semantics (not @$ rewind).
				// Create a parent context with the container object, then create the array item context.
				parentObjCtx := evalCtx.NewChildContext(cv.parentObj)
				itemCtx = parentObjCtx.NewArrayItemContext(contextData)
			} else {
				itemCtx = evalCtx.NewArrayItemContext(contextData)
			}
			// Apply inherited bindings from @$ / #$ operators
			if len(inheritedBindings) > 0 {
				applyBindingsToCtx(itemCtx, inheritedBindings)
			}

			// Evaluate right side in item context
			var value interface{}
			if node.RHS.Type == types.NodeString {
				value, err = e.evalNameString(node.RHS.Value.(string), itemCtx)
			} else if node.RHS.Type == types.NodeName {
				value, err = e.evalName(node.RHS, itemCtx)
			} else if node.RHS.Type == types.NodeFunction && node.RHS.LHS != nil && node.RHS.LHS.Type == types.NodeLambda {
				// Special case: lambda call in path context
				value, err = e.evalFunctionWithContextInjection(ctx, node.RHS, itemCtx, actualItem)
			} else {
				value, err = e.evalNode(ctx, node.RHS, itemCtx)
			}
			if err != nil {
				return nil, err
			}

			// Flatten: if value is an array, append its elements
			// UNLESS the RHS is an explicit array constructor or a filter wrapping one,
			// in which case we keep the inner array intact.
			if value != nil {
				// Check if the RHS is an array constructor (possibly wrapped in a filter with [])
				rhsIsArrayCtor := node.RHS.Type == types.NodeArray ||
					(node.RHS.Type == types.NodeFilter && node.RHS.LHS != nil && node.RHS.LHS.Type == types.NodeArray)

				if len(inheritedBindings) > 0 {
					// Propagate inherited bindings to each sub-result so they remain accessible
					// in subsequent path steps (@$ / #$ cross-join semantics).
					// parent=nil: the sub-result becomes the new context (no further rewind).
					if subArr, isArr := value.([]interface{}); isArr && !rhsIsArrayCtor {
						for _, subItem := range subArr {
							result = append(result, mergeBoundBindings(subItem, inheritedBindings, nil))
						}
					} else {
						result = append(result, mergeBoundBindings(value, inheritedBindings, nil))
					}
				} else {
					if subArr, isArr := value.([]interface{}); isArr && !rhsIsArrayCtor {
						// When flattening a sub-array, wrap each sub-item with parent info
						// so that the % (parent) operator can find the containing object.
						// Each sub-item's parent is the current item (e.g., Products' parent = Order).
						for _, subItem := range subArr {
							// Only wrap if the sub-item doesn't already have parent info
							if _, alreadyCV := subItem.(*contextBoundValue); !alreadyCV {
								result = append(result, &contextBoundValue{
									value:     subItem,
									parent:    nil,
									bindings:  map[string]interface{}{},
									parentObj: actualItem,
								})
							} else {
								result = append(result, subItem)
							}
						}
					} else {
						result = append(result, value)
					}
				}
			}
		}

		// Return empty array as nil per JSONata semantics
		if len(result) == 0 {
			return nil, nil
		}

		// Unwrap contextBoundValues from the final result only if there's no further
		// wrapping needed: if all results are plain values, return directly.
		// If results are cvs, keep them (they'll be handled by the next path/filter step
		// or unwrapped by the final return).
		allPlain := true
		for _, r := range result {
			if _, ok := r.(*contextBoundValue); ok {
				allPlain = false
				break
			}
		}
		if allPlain {
			// Standard result: unwrap singletons
			if len(result) == 1 && !keepArray {
				return result[0], nil
			}
			return result, nil
		}
		// Results contain cvs – leave them for the next stage; do NOT unwrap singletons
		// (caller is another evalPath or evalFilter which will handle them)
		if len(result) == 1 && !keepArray {
			return result[0], nil
		}
		return result, nil
	}

	// For non-array, create new context with left as data
	pathCtx := evalCtx.NewChildContext(left)

	if node.RHS.Type == types.NodeString {
		return e.evalNameString(node.RHS.Value.(string), pathCtx)
	}
	if node.RHS.Type == types.NodeName {
		return e.evalName(node.RHS, pathCtx)
	}
	if node.RHS.Type == types.NodeFunction && node.RHS.LHS != nil && node.RHS.LHS.Type == types.NodeLambda {
		// Special case: lambda call in path context
		// The left value should be injected as the first argument
		return e.evalFunctionWithContextInjection(ctx, node.RHS, pathCtx, left)
	}

	// Evaluate right side in new context
	return e.evalNode(ctx, node.RHS, pathCtx)
}

// evalDescendent evaluates a descendent expression (recursive field search).
// The descendent operator ** returns ALL descendants, then RHS is applied as a path to each.

func (e *Evaluator) evalDescendent(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Special case: if LHS is a string literal, treat it as a field name in current context
	var left interface{}
	var err error

	if node.LHS.Type == types.NodeString {
		// String literal as LHS means field name lookup in current context
		fieldName := node.LHS.Value.(string)
		left, err = e.evalNameString(fieldName, evalCtx)
	} else {
		// Normal evaluation of LHS
		left, err = e.evalNode(ctx, node.LHS, evalCtx)
	}

	if err != nil {
		return nil, err
	}

	// If left is nil, return nil
	if left == nil {
		return nil, nil
	}

	// Check if we should keep singleton arrays
	keepArray := node.KeepArray || hasKeepArrayInChain(node.LHS)

	// If no RHS, use JS-style collection: collect ALL non-array descendants including root.
	// This matches the JSONata reference implementation's recurseDescendants behavior.
	if node.RHS == nil {
		var results []interface{}
		var recurseDescendants func(data interface{}) error
		recurseDescendants = func(data interface{}) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if data == nil {
				return nil
			}
			// Add the item itself if it's not an array (matches JS reference)
			if _, isArray := data.([]interface{}); !isArray {
				results = append(results, data)
			}
			// Recurse into children
			switch v := data.(type) {
			case map[string]interface{}:
				for _, fieldValue := range v {
					if err := recurseDescendants(fieldValue); err != nil {
						return err
					}
				}
			case *OrderedObject:
				for _, k := range v.Keys {
					if err := recurseDescendants(v.Values[k]); err != nil {
						return err
					}
				}
			case []interface{}:
				for _, item := range v {
					if err := recurseDescendants(item); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if err := recurseDescendants(left); err != nil {
			return nil, err
		}
		results = deduplicateResults(results)
		if len(results) == 0 {
			return nil, nil
		}
		if len(results) == 1 && !keepArray {
			return results[0], nil
		}
		return results, nil
	}

	// With RHS: collect all descendants and apply the RHS path to each candidate.
	// This supports expressions like **.foo (apply "foo" to each descendant).
	var descendants []interface{}

	// Helper function to recursively collect ALL descendant values
	var collectDescendants func(data interface{}) error
	collectDescendants = func(data interface{}) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if data == nil {
			return nil
		}

		// Recursively collect from nested structures
		switch v := data.(type) {
		case map[string]interface{}:
			for _, fieldValue := range v {
				// Skip nil values
				if fieldValue == nil {
					continue
				}

				// Don't add arrays as candidates - we'll add their elements when we recurse
				// This prevents evalPath from traversing the array twice (once here, once in evalPath)
				if _, isArray := fieldValue.([]interface{}); !isArray {
					descendants = append(descendants, fieldValue)
				}

				// Recurse into this value (arrays will have their elements added during recursion)
				if err := collectDescendants(fieldValue); err != nil {
					return err
				}
			}
		case []interface{}:
			// For arrays, add each item as a descendant (but not the array itself)
			// and recurse into each item
			for _, item := range v {
				// Add this item to descendants
				if item != nil {
					descendants = append(descendants, item)
				}
				// Recurse into this item to get its descendants
				if err := collectDescendants(item); err != nil {
					return err
				}
			}
		}

		return nil
	}

	// Start collecting all descendants from left
	// Also evaluate RHS on left itself (not just descendants)
	if err := collectDescendants(left); err != nil {
		return nil, err
	}

	// Add left itself as first candidate for RHS evaluation
	allCandidates := append([]interface{}{left}, descendants...)

	// Now apply RHS as a path to each candidate (including left)
	var results []interface{}
	for _, candidate := range allCandidates {
		// Create context with candidate as data
		candCtx := evalCtx.NewChildContext(candidate)

		// Evaluate RHS in candidate context
		var value interface{}
		if node.RHS.Type == types.NodeString {
			value, err = e.evalNameString(node.RHS.Value.(string), candCtx)
		} else if node.RHS.Type == types.NodeName {
			value, err = e.evalName(node.RHS, candCtx)
		} else {
			value, err = e.evalNode(ctx, node.RHS, candCtx)
		}

		// Add non-nil results
		if err == nil && value != nil {
			if arr, ok := value.([]interface{}); ok {
				results = append(results, arr...)
			} else {
				results = append(results, value)
			}
		}
	}

	// Deduplicate results before returning
	results = deduplicateResults(results)

	// Return nil if no results found
	if len(results) == 0 {
		return nil, nil
	}

	// Unwrap singleton arrays unless keepArray is set
	if len(results) == 1 && !keepArray {
		return results[0], nil
	}

	return results, nil
}

// deduplicateResults removes duplicate values from a slice while preserving order.
// Uses deep equality comparison for complex types (maps, slices).
// Note: Only deduplicates objects and arrays, not primitive values (numbers, strings, bools).

func deduplicateResults(results []interface{}) []interface{} {
	if len(results) <= 1 {
		return results
	}

	seen := make([]interface{}, 0, len(results))
	for _, item := range results {
		// Only deduplicate complex types (maps and slices)
		// Primitive values (numbers, strings, bools) can repeat
		if !isComplexType(item) {
			seen = append(seen, item)
			continue
		}

		duplicate := false
		for _, seenItem := range seen {
			if deepEqual(item, seenItem) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			seen = append(seen, item)
		}
	}
	return seen
}

// isComplexType returns true if the value is a map or slice (complex types that should be deduplicated).
// OPT-04: type switch avoids reflect.ValueOf allocation for the common runtime types.
func isComplexType(v interface{}) bool {
	switch v.(type) {
	case []interface{}, map[string]interface{}, *OrderedObject:
		return true
	}
	return false
}

// deepEqual performs deep equality comparison between two values.
// Handles maps, slices, and primitive types.
// OPT-04: type switch avoids reflect.ValueOf allocation; covers all runtime types
// produced by encoding/json and the evaluator itself.
func deepEqual(a, b interface{}) bool {
	if a == nil {
		return b == nil
	}
	switch av := a.(type) {
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case types.Null:
		_, ok := b.(types.Null)
		return ok
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !deepEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			if !deepEqual(v, bv[k]) {
				return false
			}
		}
		return true
	case *OrderedObject:
		bv, ok := b.(*OrderedObject)
		if !ok || len(av.Keys) != len(bv.Keys) {
			return false
		}
		for _, k := range av.Keys {
			if !deepEqual(av.Values[k], bv.Values[k]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// evalWildcard evaluates a wildcard expression (*).
// Returns all values from an object or all elements from an array.

func (e *Evaluator) evalWildcard(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Get current context data
	data := evalCtx.Data()

	if data == nil {
		return nil, nil
	}

	var results []interface{}

	switch v := data.(type) {
	case map[string]interface{}:
		// For objects, return all values
		for _, value := range v {
			if value != nil {
				// Flatten arrays
				if arr, ok := value.([]interface{}); ok {
					results = append(results, arr...)
				} else {
					results = append(results, value)
				}
			}
		}
	case []interface{}:
		// For arrays, flatten and return all elements
		for _, item := range v {
			if item != nil {
				if arr, ok := item.([]interface{}); ok {
					results = append(results, arr...)
				} else {
					results = append(results, item)
				}
			}
		}
	default:
		// For other types, return the value itself
		return data, nil
	}

	// Return nil if no results
	if len(results) == 0 {
		return nil, nil
	}

	// Check if we should keep singleton arrays
	keepArray := node.KeepArray

	// Unwrap singleton arrays unless keepArray is set
	if len(results) == 1 && !keepArray {
		return results[0], nil
	}

	return results, nil
}

// hasKeepArrayInChain recursively checks if any node in the LHS chain has KeepArray set.
// This helper traverses the node tree to find [] syntax anywhere in the path chain.

func hasKeepArrayInChain(node *types.ASTNode) bool {
	if node == nil {
		return false
	}
	if node.KeepArray {
		return true
	}
	// Recursively check LHS chain (for nested paths and filters)
	if node.LHS != nil && hasKeepArrayInChain(node.LHS) {
		return true
	}
	return false
}

// evalBinary evaluates a binary operator expression.
