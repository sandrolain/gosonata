package evaluator

import (
	"context"
	"sort"

	"github.com/sandrolain/gosonata/pkg/types"
)

func (e *Evaluator) evalSort(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Evaluate the sequence to sort
	sequence, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// Handle undefined: return undefined
	if sequence == nil {
		return nil, nil
	}

	// Convert to array
	var items []interface{}
	switch s := sequence.(type) {
	case []interface{}:
		items = s
	default:
		// Single item, wrap as array for sorting then unwrap
		items = []interface{}{s}
	}

	if len(items) == 0 {
		return items, nil
	}

	// Collect sort key expressions (support both single RHS and multiple Expressions)
	type sortSpec struct {
		expr      *types.ASTNode
		ascending bool
	}
	var sortSpecs []sortSpec

	if len(node.Expressions) > 0 {
		// Multiple sort keys
		for _, keyExpr := range node.Expressions {
			ascending := true
			key := keyExpr
			if keyExpr.Type == types.NodeUnary && keyExpr.Value == "<" {
				ascending = true
				key = keyExpr.LHS
			} else if keyExpr.Type == types.NodeUnary && keyExpr.Value == ">" {
				ascending = false
				key = keyExpr.LHS
			}
			sortSpecs = append(sortSpecs, sortSpec{expr: key, ascending: ascending})
		}
	} else if node.RHS != nil {
		// Single sort key
		sortKeyExpr := node.RHS
		ascending := true
		keyExpr := sortKeyExpr
		if sortKeyExpr.Type == types.NodeUnary && sortKeyExpr.Value == "<" {
			ascending = true
			keyExpr = sortKeyExpr.LHS
		} else if sortKeyExpr.Type == types.NodeUnary && sortKeyExpr.Value == ">" {
			ascending = false
			keyExpr = sortKeyExpr.LHS
		}
		sortSpecs = append(sortSpecs, sortSpec{expr: keyExpr, ascending: ascending})
	}

	if len(sortSpecs) == 0 {
		return items, nil
	}

	// Pre-evaluate all sort keys for all items
	type itemKeys struct {
		value interface{}
		keys  []interface{}
	}

	sortData := make([]itemKeys, len(items))
	for idx, item := range items {
		if item == nil {
			sortData[idx] = itemKeys{value: item, keys: make([]interface{}, len(sortSpecs))}
			continue
		}

		// Extract actual value and bindings from contextBoundValue if present
		actualSortItem, sortBindings := extractBoundItem(item)
		itemCtx := evalCtx.NewChildContext(actualSortItem)
		if len(sortBindings) > 0 {
			applyBindingsToCtx(itemCtx, sortBindings)
		}
		keys := make([]interface{}, len(sortSpecs))

		for specIdx, spec := range sortSpecs {
			key, err := e.evalNode(ctx, spec.expr, itemCtx)
			if err != nil {
				return nil, err
			}
			keys[specIdx] = key
		}

		sortData[idx] = itemKeys{value: item, keys: keys}
	}

	// Validate sort key types: all non-nil keys for a given spec must be the same type
	// and must be strings or numbers (T2007/T2008). Nil keys sort last (treated as undefined).
	for specIdx := range sortSpecs {
		var firstType string
		for _, sd := range sortData {
			key := sd.keys[specIdx]
			if key == nil {
				continue // nil sorts last, no error
			}
			var keyType string
			switch key.(type) {
			case float64, int:
				keyType = "number"
			case string:
				keyType = "string"
			default:
				return nil, types.NewError(types.ErrSortNotComparable, "argument to sort must be a string or number", -1)
			}
			if firstType == "" {
				firstType = keyType
			} else if firstType != keyType {
				return nil, types.NewError(types.ErrSortMixedTypes, "sort arguments must be of the same type", -1)
			}
		}
	}

	// Sort using all sort keys (stable, lexicographic on multiple keys)
	var sortErr error
	sort.SliceStable(sortData, func(i, j int) bool {
		if sortErr != nil {
			return false
		}

		for specIdx, spec := range sortSpecs {
			ki := sortData[i].keys[specIdx]
			kj := sortData[j].keys[specIdx]

			// Nil values go to end
			if ki == nil && kj == nil {
				continue
			}
			if ki == nil {
				return false
			}
			if kj == nil {
				return true
			}

			cmp := compareValues(ki, kj)
			if cmp == 0 {
				continue // Tie in this key, check next key
			}

			if spec.ascending {
				return cmp < 0
			}
			return cmp > 0
		}
		return false // All keys equal
	})

	if sortErr != nil {
		return nil, sortErr
	}

	// Extract sorted values
	result := make([]interface{}, len(sortData))
	for i, sd := range sortData {
		result[i] = sd.value
	}

	// Singleton unwrap: if input was single item, return single item
	if len(result) == 1 {
		return result[0], nil
	}

	return result, nil
}

// deepClone performs a deep copy of a JSON-like value.
// Maps and slices are cloned recursively; scalars are returned as-is (value types).

func deepClone(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		clone := make(map[string]interface{}, len(val))
		for k, v2 := range val {
			clone[k] = deepClone(v2)
		}
		return clone
	case *OrderedObject:
		clone := &OrderedObject{
			Keys:   make([]string, len(val.Keys)),
			Values: make(map[string]interface{}, len(val.Values)),
		}
		copy(clone.Keys, val.Keys)
		for k, v2 := range val.Values {
			clone.Values[k] = deepClone(v2)
		}
		return clone
	case []interface{}:
		clone := make([]interface{}, len(val))
		for i, v2 := range val {
			clone[i] = deepClone(v2)
		}
		return clone
	default:
		return val // scalars (nil, bool, float64, string, etc.) are value types
	}
}

// applyUpdateToMap merges the update object into a map[string]interface{}.
// The update is evaluated in the context of the matched node.

func applyUpdateToMap(target map[string]interface{}, update interface{}) {
	switch uv := update.(type) {
	case map[string]interface{}:
		for k, v := range uv {
			target[k] = v
		}
	case *OrderedObject:
		for _, k := range uv.Keys {
			target[k] = uv.Values[k]
		}
	}
}

// applyDeleteToMap removes fields from a map[string]interface{} based on the delete expression result.

func applyDeleteToMap(target map[string]interface{}, del interface{}) {
	switch dv := del.(type) {
	case string:
		delete(target, dv)
	case []interface{}:
		for _, d := range dv {
			if s, ok := d.(string); ok {
				delete(target, s)
			}
		}
	}
}

// evalTransformNode applies a NodeTransform expression to data.
// Used both as a standalone transform and when piped via ~>.

func (e *Evaluator) evalTransformNode(ctx context.Context, data interface{}, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	if data == nil {
		return nil, nil
	}

	// Deep clone the data to avoid mutating the original
	cloned := deepClone(data)

	path := node.LHS   // path expression to locate matching nodes
	update := node.RHS // update object expression
	var delExpr *types.ASTNode
	if len(node.Expressions) > 0 {
		delExpr = node.Expressions[0]
	}

	// Evaluate path expression on the cloned data to get matching nodes.
	// Since Go maps are reference types, these are aliases into the cloned tree.
	rootCtx := evalCtx.NewChildContext(cloned)
	matches, err := e.evalNode(ctx, path, rootCtx)
	if err != nil {
		// Path doesn't match anything – no transformation needed
		return cloned, nil
	}
	if matches == nil {
		return cloned, nil
	}

	// Normalize to slice
	var matchList []interface{}
	switch mv := matches.(type) {
	case []interface{}:
		matchList = mv
	default:
		matchList = []interface{}{mv}
	}

	// Apply update/delete to each matched node
	for _, matchedNode := range matchList {
		// Unwrap contextBoundValues - transforms need to mutate the actual objects
		matchedNode = unwrapCVsDeep(matchedNode)
		// Evaluate update expression in context of matched node
		matchCtx := evalCtx.NewChildContext(matchedNode)
		updateVal, err := e.evalNode(ctx, update, matchCtx)
		if err != nil {
			return nil, err
		}
		if updateVal != nil {
			// Validate update is an object (T2011)
			switch updateVal.(type) {
			case map[string]interface{}, *OrderedObject:
				// OK
			default:
				return nil, types.NewError(types.ErrTransformUpdateNotObj, "the second argument of the transform expression must be an object", -1)
			}
		}

		// Gather fields to delete
		var delFields []string
		if delExpr != nil {
			delVal, _ := e.evalNode(ctx, delExpr, matchCtx)
			if delVal != nil {
				switch dv := delVal.(type) {
				case string:
					delFields = []string{dv}
				case []interface{}:
					for _, d := range dv {
						if s, ok := d.(string); ok {
							delFields = append(delFields, s)
						}
					}
				default:
					_ = dv
					return nil, types.NewError(types.ErrTransformDeleteNotArr, "the third argument of the transform expression must be an array of strings", -1)
				}
			}
		}

		// Apply to map[string]interface{}
		if matchedMap, ok := matchedNode.(map[string]interface{}); ok {
			if updateVal != nil {
				applyUpdateToMap(matchedMap, updateVal)
			}
			for _, f := range delFields {
				delete(matchedMap, f)
			}
			continue
		}

		// Apply to *OrderedObject
		if matchedObj, ok := matchedNode.(*OrderedObject); ok {
			if updateVal != nil {
				switch uv := updateVal.(type) {
				case map[string]interface{}:
					for k, v := range uv {
						if _, exists := matchedObj.Values[k]; !exists {
							matchedObj.Keys = append(matchedObj.Keys, k)
						}
						matchedObj.Values[k] = v
					}
				case *OrderedObject:
					for _, k := range uv.Keys {
						if _, exists := matchedObj.Values[k]; !exists {
							matchedObj.Keys = append(matchedObj.Keys, k)
						}
						matchedObj.Values[k] = uv.Values[k]
					}
				}
			}
			for _, f := range delFields {
				if _, exists := matchedObj.Values[f]; exists {
					delete(matchedObj.Values, f)
					// Remove from Keys slice
					newKeys := matchedObj.Keys[:0]
					for _, k := range matchedObj.Keys {
						if k != f {
							newKeys = append(newKeys, k)
						}
					}
					matchedObj.Keys = newKeys
				}
			}
		}
	}

	return cloned, nil
}

// callLambda calls a lambda function with the given arguments.
