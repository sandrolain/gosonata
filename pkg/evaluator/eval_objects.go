package evaluator

import (
	"context"
	"fmt"

	"github.com/sandrolain/gosonata/pkg/types"
)

func (e *Evaluator) evalPathInfixObjectConstructor(ctx context.Context, node *types.ASTNode, items []interface{}, evalCtx *EvalContext) (interface{}, error) {
	if len(items) == 0 {
		return &OrderedObject{
			Keys:   make([]string, 0),
			Values: make(map[string]interface{}),
		}, nil
	}

	// Collect all sub-items from all path items and track which path-item they came from
	allSubItems := make([]interface{}, 0)
	subItemToPathItem := make(map[int]int) // subItem index -> pathItem index

	for pathItemIdx, pathItem := range items {
		if pathItem == nil {
			continue
		}

		pathItemCtx := evalCtx.NewChildContext(pathItem)

		// Evaluate node.LHS in context of this path item (e.g., Product from order)
		subCollection, err := e.evalNode(ctx, node.LHS, pathItemCtx)
		if err != nil {
			return nil, err
		}

		if subCollection == nil {
			continue
		}

		// Convert to array if needed
		var subItems []interface{}
		if arr, ok := subCollection.([]interface{}); ok {
			subItems = arr
		} else {
			subItems = []interface{}{subCollection}
		}

		// Track all sub-items and remember which path-item they came from
		for _, subItem := range subItems {
			if subItem != nil {
				allSubItems = append(allSubItems, subItem)
				subItemToPathItem[len(allSubItems)-1] = pathItemIdx
			}
		}
	}

	if len(allSubItems) == 0 {
		return &OrderedObject{
			Keys:   make([]string, 0),
			Values: make(map[string]interface{}),
		}, nil
	}

	// Now apply grouped semantics to allSubItems
	// Track which sub-items contribute to which keys
	groups := make(map[string][]int) // key -> list of subItem indices
	pairPerKey := make(map[string]int)

	// First, group items by evaluating keys for all pairs
	for pairIdx, pair := range node.Expressions {
		if pair.Type != types.NodeBinary || pair.Value != ":" {
			return nil, fmt.Errorf("invalid object property")
		}

		for subItemIdx, subItem := range allSubItems {
			if subItem == nil {
				continue
			}
			subItemCtx := evalCtx.NewChildContext(subItem)
			keys, err := e.evalObjectKeys(ctx, pair.LHS, subItemCtx, false)
			if err != nil {
				return nil, err
			}
			for _, key := range keys {
				// Check for duplicate key from different pairs (D1009 error)
				if existingPair, exists := pairPerKey[key]; exists && existingPair != pairIdx {
					return nil, fmt.Errorf("D1009: Duplicate object key %s", key)
				}
				pairPerKey[key] = pairIdx
				groups[key] = append(groups[key], subItemIdx)
			}
		}
	}

	// Merge semantics: return single object with grouped values
	result := &OrderedObject{
		Keys:   make([]string, 0, len(groups)),
		Values: make(map[string]interface{}, len(groups)),
	}

	for key := range groups {
		pairIdx := pairPerKey[key]
		pair := node.Expressions[pairIdx]

		// Create group context with all sub-items that have this key
		groupItems := make([]interface{}, 0, len(groups[key]))
		for _, subItemIdx := range groups[key] {
			groupItems = append(groupItems, allSubItems[subItemIdx])
		}

		// Evaluate value expression in the context of the group
		// If only one item, evaluate in that single item's context
		// If multiple items, evaluate in array context
		var value interface{}
		var err error
		if len(groupItems) == 1 {
			// Single item: evaluate in its context
			groupCtx := evalCtx.NewChildContext(groupItems[0])
			value, err = e.evalNode(ctx, pair.RHS, groupCtx)
		} else {
			// Multiple items: evaluate in array context
			groupCtx := evalCtx.NewChildContext(groupItems)
			value, err = e.evalNode(ctx, pair.RHS, groupCtx)
		}
		if err != nil {
			return nil, err
		}
		if value != nil {
			result.Keys = append(result.Keys, key)
			result.Values[key] = value
		}
	}

	return result, nil
}

// evalObjectGroupedWithArray is a helper for evaluating object constructors
// applied to arrays via paths (expr.{...})

func (e *Evaluator) evalObjectGroupedWithArray(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext, items []interface{}) (interface{}, error) {
	if len(items) == 0 {
		return &OrderedObject{
			Keys:   make([]string, 0),
			Values: make(map[string]interface{}),
		}, nil
	}

	// Track which items contribute to which keys
	groups := make(map[string][]int)
	pairPerKey := make(map[string]int)

	// First, group items by evaluating keys for all expressions
	for pairIdx, pair := range node.Expressions {
		if pair.Type != types.NodeBinary || pair.Value != ":" {
			return nil, fmt.Errorf("invalid object property")
		}

		for itemIdx, item := range items {
			if item == nil {
				continue
			}
			itemCtx := evalCtx.NewArrayItemContext(item)
			keys, err := e.evalObjectKeys(ctx, pair.LHS, itemCtx, false)
			if err != nil {
				return nil, err
			}
			for _, key := range keys {
				// Check for duplicate key from different pair expressions
				if existingPair, exists := pairPerKey[key]; exists && existingPair != pairIdx {
					return nil, fmt.Errorf("D1009: Duplicate object key %s", key)
				}
				pairPerKey[key] = pairIdx
				groups[key] = append(groups[key], itemIdx)
			}
		}
	}

	// Prefix constructor applied to array via path: ALWAYS return array of objects, one per item
	result := make([]interface{}, len(items))
	for itemIdx, item := range items {
		if item == nil {
			result[itemIdx] = &OrderedObject{
				Keys:   make([]string, 0),
				Values: make(map[string]interface{}),
			}
			continue
		}

		objResult := &OrderedObject{
			Keys:   make([]string, 0, len(node.Expressions)),
			Values: make(map[string]interface{}, len(node.Expressions)),
		}

		// Find all keys that this item contributed to
		for key, indices := range groups {
			for _, idx := range indices {
				if idx == itemIdx {
					// Find which pair created this key to evaluate value
					pairIdx := pairPerKey[key]
					pair := node.Expressions[pairIdx]

					itemCtx := evalCtx.NewArrayItemContext(item)
					value, err := e.evalNode(ctx, pair.RHS, itemCtx)
					if err != nil {
						return nil, err
					}
					if value != nil {
						objResult.Keys = append(objResult.Keys, key)
						objResult.Values[key] = value
					}
					break
				}
			}
		}

		result[itemIdx] = objResult
	}
	return result, nil
}

// evalArray evaluates an array constructor.

func (e *Evaluator) evalArray(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	result := make([]interface{}, 0, len(node.Expressions))

	for _, expr := range node.Expressions {
		value, err := e.evalNode(ctx, expr, evalCtx)
		if err != nil {
			return nil, err
		}

		// Flatten arrays from range operators or other operations that generate arrays
		// BUT do NOT flatten explicitly nested array literals like [[1,2,3]]
		// Only flatten if the expression is NOT an array literal (NodeArray)
		if value != nil {
			if subArr, isArr := value.([]interface{}); isArr && expr.Type != types.NodeArray {
				// Flatten: this is an array from a range or other operation
				result = append(result, subArr...)
			} else {
				// Keep as-is: either not an array, or an explicit array literal
				result = append(result, value)
			}
		}
	}

	return result, nil
}

// evalObject evaluates an object constructor.

func (e *Evaluator) evalObject(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	if node.LHS == nil {
		return e.evalObjectLiteral(ctx, node, evalCtx)
	}

	return e.evalObjectGrouped(ctx, node, evalCtx)
}

func (e *Evaluator) evalObjectLiteral(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	result := &OrderedObject{
		Keys:   make([]string, 0, len(node.Expressions)),
		Values: make(map[string]interface{}, len(node.Expressions)),
	}

	for _, pair := range node.Expressions {
		if pair.Type != types.NodeBinary || pair.Value != ":" {
			return nil, fmt.Errorf("invalid object property")
		}

		keys, err := e.evalObjectKeys(ctx, pair.LHS, evalCtx, true)
		if err != nil {
			return nil, err
		}
		if len(keys) == 0 {
			continue
		}

		value, err := e.evalNode(ctx, pair.RHS, evalCtx)
		if err != nil {
			return nil, err
		}
		if value == nil {
			continue
		}

		// Unwrap any contextBoundValues that escaped from path expressions (e.g. #$i bindings).
		// The bindings have already been consumed by inner expressions; we only need the plain value.
		value = unwrapCVsDeep(value)

		for _, key := range keys {
			if _, exists := result.Values[key]; exists {
				return nil, fmt.Errorf("D1009: Duplicate object key %s", key)
			}
			result.Keys = append(result.Keys, key)
			result.Values[key] = value
		}
	}

	return result, nil
}

func (e *Evaluator) evalObjectGrouped(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	collection, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	if collection == nil {
		// Return empty object
		return &OrderedObject{
			Keys:   make([]string, 0),
			Values: make(map[string]interface{}),
		}, nil
	}

	items := []interface{}{}
	if arr, ok := collection.([]interface{}); ok {
		items = arr
	} else {
		items = append(items, collection)
	}
	if len(items) == 0 {
		return &OrderedObject{
			Keys:   make([]string, 0),
			Values: make(map[string]interface{}),
		}, nil
	}

	// Track which items contribute to which keys
	// groups: key -> list of item indices that produced that key
	groups := make(map[string][]int)
	// pair_per_key: key -> which pair expression created it
	pairPerKey := make(map[string]int)

	// First, group items by evaluating keys for all pairs
	for pairIdx, pair := range node.Expressions {
		if pair.Type != types.NodeBinary || pair.Value != ":" {
			return nil, fmt.Errorf("invalid object property")
		}

		for itemIdx, item := range items {
			if item == nil {
				continue
			}
			itemCtx := evalCtx.NewChildContext(item)
			keys, err := e.evalObjectKeys(ctx, pair.LHS, itemCtx, false)
			if err != nil {
				return nil, err
			}
			for _, key := range keys {
				// Check for duplicate key from different pair expressions
				if existingPair, exists := pairPerKey[key]; exists && existingPair != pairIdx {
					return nil, fmt.Errorf("D1009: Duplicate object key %s", key)
				}
				pairPerKey[key] = pairIdx
				groups[key] = append(groups[key], itemIdx)
			}
		}
	}

	// Determine if we should return array of objects or single merged object
	// - Infix grouping (isGrouping=true): ALWAYS merge, return single object
	// - Prefix constructor applied via path: ALWAYS return array of objects, one per item
	if !node.IsGrouping {
		// Prefix constructor: return array of objects, one per item
		result := make([]interface{}, len(items))
		for itemIdx, item := range items {
			if item == nil {
				result[itemIdx] = &OrderedObject{
					Keys:   make([]string, 0),
					Values: make(map[string]interface{}),
				}
				continue
			}

			objResult := &OrderedObject{
				Keys:   make([]string, 0, len(node.Expressions)),
				Values: make(map[string]interface{}, len(node.Expressions)),
			}

			// Find all keys that this item contributed to
			for key, indices := range groups {
				for _, idx := range indices {
					if idx == itemIdx {
						// Find which pair created this key to evaluate value
						pairIdx := pairPerKey[key]
						pair := node.Expressions[pairIdx]

						itemCtx := evalCtx.NewChildContext(item)
						value, err := e.evalNode(ctx, pair.RHS, itemCtx)
						if err != nil {
							return nil, err
						}
						if value != nil {
							objResult.Keys = append(objResult.Keys, key)
							objResult.Values[key] = value
						}
						break
					}
				}
			}

			result[itemIdx] = objResult
		}
		return result, nil
	}

	// Merge semantics: return single object (used for infix grouping or non-one-to-one prefix)
	result := &OrderedObject{
		Keys:   make([]string, 0, len(groups)),
		Values: make(map[string]interface{}, len(groups)),
	}

	for key := range groups {
		pairIdx := pairPerKey[key]
		pair := node.Expressions[pairIdx]
		groupItems := make([]interface{}, 0, len(groups[key]))
		for _, itemIdx := range groups[key] {
			groupItems = append(groupItems, items[itemIdx])
		}

		groupCtx := evalCtx.NewChildContext(groupItems)
		value, err := e.evalNode(ctx, pair.RHS, groupCtx)
		if err != nil {
			return nil, err
		}
		if value != nil {
			result.Keys = append(result.Keys, key)
			result.Values[key] = value
		}
	}

	return result, nil
}

func (e *Evaluator) evalObjectKeys(ctx context.Context, keyNode *types.ASTNode, evalCtx *EvalContext, literal bool) ([]string, error) {
	// For string literals, use the value directly
	if keyNode.Type == types.NodeString {
		return []string{keyNode.Value.(string)}, nil
	}

	// NodeName keys are ALWAYS evaluated as expressions (never treated as string literals).
	// In JSONata: `{name: val}` evaluates `name` as a field path, not as the string "name".
	// If the field does not exist (nil), the key is omitted (no entry created).
	// This is true for both standalone and path-step object constructors.
	// (String literal keys, e.g. `{"name": val}`, are handled above via NodeString.)
	_ = literal

	// Evaluate as expression
	keyVal, err := e.evalNode(ctx, keyNode, evalCtx)
	if err != nil {
		return nil, err
	}
	if keyVal == nil {
		return nil, nil
	}
	if _, ok := keyVal.(types.Null); ok {
		return nil, fmt.Errorf("T1003: Object key must be a string")
	}

	switch v := keyVal.(type) {
	case string:
		return []string{v}, nil
	case []interface{}:
		keys := make([]string, 0, len(v))
		for _, item := range v {
			if item == nil {
				continue
			}
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("T1003: Object key must be a string, got %T", item)
			}
			keys = append(keys, str)
		}
		return keys, nil
	default:
		return nil, fmt.Errorf("T1003: Object key must be a string, got %T", keyVal)
	}
}

// evalFilter evaluates a filter expression.
