package evaluator

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/sandrolain/gosonata/pkg/types"
)

// evalNode evaluates an AST node in the given context.
func (e *Evaluator) evalNode(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Check recursion depth
	if evalCtx.Depth() > e.opts.MaxDepth {
		return nil, fmt.Errorf("maximum recursion depth exceeded")
	}

	if node == nil {
		return nil, nil
	}

	// Debug logging
	if e.opts.Debug {
		e.logger.Debug("evaluating node",
			"type", node.Type,
			"value", node.Value,
			"depth", evalCtx.Depth())
	}

	// Dispatch based on node type
	switch node.Type {
	case types.NodeString:
		return e.evalString(node)
	case types.NodeNumber:
		return e.evalNumber(node)
	case "value": // NodeBoolean or NodeNull
		// Keep types.Null as-is during evaluation
		// Will be converted to nil at final return
		return node.Value, nil
	case types.NodeName:
		return e.evalName(node, evalCtx)
	case types.NodeVariable:
		return e.evalVariable(node, evalCtx)
	case types.NodePath:
		return e.evalPath(ctx, node, evalCtx)
	case types.NodeBinary:
		return e.evalBinary(ctx, node, evalCtx)
	case types.NodeUnary:
		return e.evalUnary(ctx, node, evalCtx)
	case types.NodeArray:
		return e.evalArray(ctx, node, evalCtx)
	case types.NodeObject:
		return e.evalObject(ctx, node, evalCtx)
	case types.NodeFilter:
		return e.evalFilter(ctx, node, evalCtx)
	case types.NodeCondition:
		return e.evalCondition(ctx, node, evalCtx)
	case types.NodeFunction:
		return e.evalFunction(ctx, node, evalCtx)
	case types.NodeLambda:
		return e.evalLambda(node, evalCtx)
	case types.NodeBind:
		return e.evalBind(ctx, node, evalCtx)
	case types.NodeBlock:
		return e.evalBlock(ctx, node, evalCtx)
	case types.NodeSort:
		return e.evalSort(ctx, node, evalCtx)
	case types.NodeParent:
		return e.evalParent(node, evalCtx)
	default:
		return nil, fmt.Errorf("unsupported node type: %s", node.Type)
	}
}

// evalString evaluates a string literal.
func (e *Evaluator) evalString(node *types.ASTNode) (interface{}, error) {
	return node.Value, nil
}

// evalNumber evaluates a number literal.
func (e *Evaluator) evalNumber(node *types.ASTNode) (interface{}, error) {
	return node.Value, nil
}

// evalBoolean evaluates a boolean literal.
func (e *Evaluator) evalBoolean(node *types.ASTNode) (interface{}, error) {
	return node.Value, nil
}

// evalName evaluates a name (field reference).
func (e *Evaluator) evalName(node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	name := node.Value.(string)
	return e.evalNameString(name, evalCtx)
}

func (e *Evaluator) evalNameString(name string, evalCtx *EvalContext) (interface{}, error) {
	data := evalCtx.Data()

	if obj, ok := data.(map[string]interface{}); ok {
		if value, exists := obj[name]; exists {
			return value, nil
		}
	}
	if obj, ok := data.(*OrderedObject); ok {
		if value, exists := obj.Get(name); exists {
			return value, nil
		}
	}
	if arr, ok := data.([]interface{}); ok {
		result := make([]interface{}, 0, len(arr))
		for _, item := range arr {
			if obj, ok := item.(map[string]interface{}); ok {
				if value, exists := obj[name]; exists {
					if subArr, isArr := value.([]interface{}); isArr {
						result = append(result, subArr...)
					} else {
						result = append(result, value)
					}
				}
			} else if obj, ok := item.(*OrderedObject); ok {
				if value, exists := obj.Get(name); exists {
					if subArr, isArr := value.([]interface{}); isArr {
						result = append(result, subArr...)
					} else {
						result = append(result, value)
					}
				}
			}
		}
		if len(result) == 0 {
			return nil, nil
		}
		return result, nil
	}

	return nil, nil
}

// evalVariable evaluates a variable reference.
func (e *Evaluator) evalVariable(node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	varName := node.Value.(string)

	// $ refers to current context
	if varName == "" {
		return evalCtx.Data(), nil
	}

	// $$ refers to parent context
	if varName == "$" {
		if evalCtx.Parent() != nil {
			return evalCtx.Parent().Data(), nil
		}
		return nil, nil
	}

	// Named variable - check bindings
	value, found := evalCtx.GetBinding(varName)
	if !found {
		// If a built-in function exists with this name, return it as a value
		if fnDef, ok := GetFunction(varName); ok {
			return fnDef, nil
		}
		// Per JSONata spec: undefined variables return nil (undefined), not error
		return nil, nil
	}

	return value, nil

	return value, nil
}

// evalPath evaluates a path expression (field navigation).
func (e *Evaluator) evalPath(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Evaluate left side
	left, err := e.evalNode(ctx, node.LHS, evalCtx)
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
	// do grouping evaluation instead of simple array map
	if node.RHS.Type == types.NodeObject && node.RHS.LHS == nil {
		// This is path.{...} applied to an array
		if arr, ok := left.([]interface{}); ok {
			// Create a modified node for grouping evaluation
			groupNode := &types.ASTNode{
				Type:        types.NodeObject,
				Value:       node.RHS.Value,
				Expressions: node.RHS.Expressions,
				LHS:         &types.ASTNode{Type: types.NodeVariable, Value: ""}, // Placeholder for left
				IsGrouping:  false,                                               // Prefix constructor, not infix
			}
			// Manually set up the grouping evaluation
			return e.evalObjectGroupedWithArray(ctx, groupNode, evalCtx, arr)
		}
	}

	// Check if left is an array - JSONata applies path to each element
	if arr, ok := left.([]interface{}); ok {
		// Special case: if RHS is an infix object constructor (node.RHS.LHS != nil),
		// apply the constructor to each item, then merge all results
		if node.RHS.Type == types.NodeObject && node.RHS.LHS != nil {
			return e.evalPathInfixObjectConstructor(ctx, node.RHS, arr, evalCtx)
		}

		// Apply path to each element of the array
		result := make([]interface{}, 0, len(arr))
		for _, item := range arr {
			// Create context with item as data
			itemCtx := evalCtx.NewChildContext(item)

			// Evaluate right side in item context
			var value interface{}
			if node.RHS.Type == types.NodeString {
				value, err = e.evalNameString(node.RHS.Value.(string), itemCtx)
			} else if node.RHS.Type == types.NodeName {
				value, err = e.evalName(node.RHS, itemCtx)
			} else {
				value, err = e.evalNode(ctx, node.RHS, itemCtx)
			}
			if err != nil {
				return nil, err
			}

			// Flatten: if value is an array, append its elements
			// Otherwise append the value itself (if not nil)
			if value != nil {
				if subArr, isArr := value.([]interface{}); isArr {
					result = append(result, subArr...)
				} else {
					result = append(result, value)
				}
			}
		}

		// Return empty array as nil per JSONata semantics
		if len(result) == 0 {
			return nil, nil
		}

		// If keepArray is false and we have a singleton, unwrap it
		// This implements the JSONata behavior where singleton arrays are flattened
		// unless explicitly marked to keep (e.g., with [] syntax)
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

	// Evaluate right side in new context
	return e.evalNode(ctx, node.RHS, pathCtx)
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
func (e *Evaluator) evalBinary(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	op := node.Value.(string)

	// Handle special operators
	switch op {
	case "and":
		return e.evalAnd(ctx, node, evalCtx)
	case "or":
		return e.evalOr(ctx, node, evalCtx)
	case "??":
		return e.evalCoalesce(ctx, node, evalCtx)
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
		Keys:   make([]string, 0),
		Values: make(map[string]interface{}),
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
			Keys:   make([]string, 0),
			Values: make(map[string]interface{}),
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
				Keys:   make([]string, 0),
				Values: make(map[string]interface{}),
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
		Keys:   make([]string, 0),
		Values: make(map[string]interface{}),
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

	// For name nodes with spaces, use the name literally ONLY if literal=true
	// Otherwise, evaluate the name like any other expression
	if keyNode.Type == types.NodeName {
		keyName := keyNode.Value.(string)
		if literal {
			// For literal mode, check if context data is array
			// If so, validate that all items have the same string value for this key
			data := evalCtx.Data()
			if arr, ok := data.([]interface{}); ok && len(arr) > 0 {
				// Context is array - validate the key values
				keyVal, err := e.evalNode(ctx, keyNode, evalCtx)
				if err != nil {
					return nil, err
				}
				// If keyVal is array with mixed values, error
				if keyArr, ok := keyVal.([]interface{}); ok {
					if len(keyArr) == 0 {
						return []string{keyName}, nil
					}
					// Check all values are strings and all equal
					var firstVal string
					for i, item := range keyArr {
						if item == nil {
							continue
						}
						str, ok := item.(string)
						if !ok {
							return nil, fmt.Errorf("T1003: Object key must be a string, got %T", item)
						}
						if i == 0 {
							firstVal = str
						} else if str != firstVal {
							// Mixed values - cannot use as single key
							return nil, fmt.Errorf("T1003: Object key must be a string")
						}
					}
				}
			}
			// Use as literal field name
			return []string{keyName}, nil
		}
		// Fall through to evaluate normally (even if name contains spaces)
	}

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

	// Check if RHS is a number (index access)
	if node.RHS.Type == types.NodeNumber {
		// Direct index access like items[0]
		// Get array from collection
		arr, err := e.toArray(collection)
		if err != nil {
			return nil, err
		}

		indexFloat, ok := node.RHS.Value.(float64)
		if !ok {
			return nil, fmt.Errorf("invalid index type")
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

	// Check if collection is an array
	arr, isArray := collection.([]interface{})
	if !isArray {
		// If not an array, treat filter as conditional
		// Evaluate predicate in context of the object
		objCtx := evalCtx.NewChildContext(collection)
		match, err := e.evalNode(ctx, node.RHS, objCtx)
		if err != nil {
			return nil, err
		}

		// If predicate is true, return the object; otherwise nil
		if e.isTruthy(match) {
			return collection, nil
		}
		return nil, nil
	}

	// Otherwise treat as array filter predicate
	result := make([]interface{}, 0)
	for _, item := range arr {
		// Create context with item as data
		itemCtx := evalCtx.NewChildContext(item)

		// Evaluate filter expression
		match, err := e.evalNode(ctx, node.RHS, itemCtx)
		if err != nil {
			return nil, err
		}

		// Check if matches (truthy value)
		if e.isTruthy(match) {
			result = append(result, item)
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
	// Evaluate condition
	condition, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// Check if condition is truthy
	if e.isTruthy(condition) {
		// Evaluate then branch
		return e.evalNode(ctx, node.RHS, evalCtx)
	}

	// Evaluate else branch
	if len(node.Expressions) > 0 && node.Expressions[0] != nil {
		return e.evalNode(ctx, node.Expressions[0], evalCtx)
	}

	return nil, nil
}

// evalFunction evaluates a function call.
func (e *Evaluator) evalFunction(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	funcName := node.Value.(string)

	// Get function definition
	fnDef, ok := GetFunction(funcName)
	if !ok {
		return nil, fmt.Errorf("unknown function: %s", funcName)
	}

	// Evaluate arguments
	args := make([]interface{}, 0, len(node.Arguments))
	for _, argNode := range node.Arguments {
		arg, err := e.evalNode(ctx, argNode, evalCtx)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}

	// Validate argument count
	if len(args) < fnDef.MinArgs {
		return nil, types.NewError(types.ErrArgumentCountMismatch,
			fmt.Sprintf("function %s requires at least %d arguments, got %d", funcName, fnDef.MinArgs, len(args)), -1)
	}
	if fnDef.MaxArgs != -1 && len(args) > fnDef.MaxArgs {
		return nil, types.NewError(types.ErrArgumentCountMismatch,
			fmt.Sprintf("function %s accepts at most %d arguments, got %d", funcName, fnDef.MaxArgs, len(args)), -1)
	}

	// Call function
	return fnDef.Impl(ctx, e, evalCtx, args)
}

// evalLambda creates a lambda function value.
func (e *Evaluator) evalLambda(node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Extract parameter names from Arguments field
	params := make([]string, 0, len(node.Arguments))
	for _, argNode := range node.Arguments {
		if argNode.Type == types.NodeVariable {
			// Parameter is a variable like $x
			paramName := argNode.Value.(string)
			params = append(params, paramName)
		}
	}

	// Create lambda with closure over current context
	lambda := &Lambda{
		Params: params,
		Body:   node.RHS, // Body is in RHS
		Ctx:    evalCtx.Clone(),
	}

	return lambda, nil
}

// evalBind evaluates an assignment expression.
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

	// Convert to numbers
	start, err := e.toNumber(startVal)
	if err != nil {
		return nil, fmt.Errorf("range start must be a number: %v", err)
	}

	end, err := e.toNumber(endVal)
	if err != nil {
		return nil, fmt.Errorf("range end must be a number: %v", err)
	}

	// Generate range
	result := make([]interface{}, 0)
	if start <= end {
		for i := int(start); i <= int(end); i++ {
			result = append(result, float64(i))
		}
	} else {
		for i := int(start); i >= int(end); i-- {
			result = append(result, float64(i))
		}
	}

	return result, nil
}

// evalApply evaluates an apply expression (~>).
func (e *Evaluator) evalApply(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Evaluate left side (the data)
	data, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// Evaluate right side (the function)
	fn, err := e.evalNode(ctx, node.RHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// If fn is a lambda, call it with data as argument
	if lambda, ok := fn.(*Lambda); ok {
		return e.callLambda(ctx, lambda, []interface{}{data})
	}

	return nil, fmt.Errorf("right side of ~> must be a function")
}

// evalSort evaluates a sort expression (^).
// Syntax: sequence^(sort-key-expression)
// Examples: items^($), data^(>price), results^(<count)
func (e *Evaluator) evalSort(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Evaluate the sequence to sort
	sequence, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// Convert to array
	items, err := e.toArray(sequence)
	if err != nil {
		return nil, fmt.Errorf("cannot sort non-array: %v", err)
	}

	if len(items) == 0 {
		return items, nil
	}

	// Parse the sort key expression to extract sort direction and key
	sortKeyExpr := node.RHS
	ascending := true
	keyExpr := sortKeyExpr

	// Check for sort direction modifiers: <$ (ascending), >$ (descending)
	if sortKeyExpr.Type == types.NodeUnary && sortKeyExpr.Value == "<" {
		ascending = true
		keyExpr = sortKeyExpr.LHS
	} else if sortKeyExpr.Type == types.NodeUnary && sortKeyExpr.Value == ">" {
		ascending = false
		keyExpr = sortKeyExpr.LHS
	}

	// Evaluate sort keys for all items
	type sortItem struct {
		value interface{}
		key   interface{}
	}

	sortItems := make([]sortItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			sortItems = append(sortItems, sortItem{value: item, key: nil})
			continue
		}

		itemCtx := evalCtx.NewChildContext(item)
		key, err := e.evalNode(ctx, keyExpr, itemCtx)
		if err != nil {
			return nil, err
		}

		sortItems = append(sortItems, sortItem{value: item, key: key})
	}

	// Sort using the keys
	sort.SliceStable(sortItems, func(i, j int) bool {
		ki := sortItems[i].key
		kj := sortItems[j].key

		// Nil values always go to the end
		if ki == nil && kj == nil {
			return false
		}
		if ki == nil {
			return false
		}
		if kj == nil {
			return true
		}

		// Compare keys
		cmp := compareValues(ki, kj)
		if ascending {
			return cmp < 0
		} else {
			return cmp > 0
		}
	})

	// Extract sorted values
	result := make([]interface{}, len(sortItems))
	for i, si := range sortItems {
		result[i] = si.value
	}

	return result, nil
}

// callLambda calls a lambda function with the given arguments.
func (e *Evaluator) callLambda(ctx context.Context, lambda *Lambda, args []interface{}) (interface{}, error) {
	// Validate argument count
	if len(args) != len(lambda.Params) {
		return nil, fmt.Errorf("lambda expects %d arguments, got %d", len(lambda.Params), len(args))
	}

	// Create new context with lambda's closure context as parent
	lambdaCtx := lambda.Ctx.Clone()

	// Bind parameters
	for i, param := range lambda.Params {
		lambdaCtx.SetBinding(param, args[i])
	}

	// Evaluate body
	return e.evalNode(ctx, lambda.Body, lambdaCtx)
}

// Helper functions

// isTruthy determines if a value is truthy.
func (e *Evaluator) isTruthy(value interface{}) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0
	case int:
		return v != 0
	case types.Null:
		return false
	case []interface{}:
		return len(v) > 0
	case map[string]interface{}:
		return len(v) > 0
	case *OrderedObject:
		return len(v.Values) > 0
	default:
		return true
	}
}

// toArray converts a value to an array.
func (e *Evaluator) toArray(value interface{}) ([]interface{}, error) {
	if value == nil {
		return []interface{}{}, nil
	}

	// Already an array
	if arr, ok := value.([]interface{}); ok {
		return arr, nil
	}

	// Single value becomes single-element array
	return []interface{}{value}, nil
}

// toNumber converts a value to a number.
func (e *Evaluator) toNumber(value interface{}) (float64, error) {
	// Handle undefined (nil) - return 0 but with error to signal undefined
	if value == nil {
		return 0, fmt.Errorf("undefined value")
	}

	switch v := value.(type) {
	case types.Null:
		return 0, fmt.Errorf("null value")
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case bool:
		// JSONata spec: true → 1, false → 0
		if v {
			return 1.0, nil
		}
		return 0.0, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to number", value)
	}
}

// tryNumber attempts to convert a value to number without error.
// Returns (value, true) if successful, (0, false) otherwise.
// NOTE: Does NOT convert bool to avoid issues with comparison operators.
// Bool should be handled explicitly in functions that need it (e.g., fnNumber, opEqual).
func (e *Evaluator) tryNumber(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	default:
		return 0, false
	}
}

// toString converts a value to a string.
func (e *Evaluator) toString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case types.Null:
		return "null"
	case string:
		return v
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return ""
		}
		return e.formatNumberForString(v)
	case int:
		return strconv.Itoa(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", value)
	}
}

// roundNumberForJSON rounds a float to 15 significant digits, matching JSONata.
func (e *Evaluator) roundNumberForJSON(v float64) float64 {
	str := strconv.FormatFloat(v, 'g', 15, 64)
	rounded, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return v
	}
	return rounded
}

// formatNumberForString formats numbers with JSONata's exponent rules.
func (e *Evaluator) formatNumberForString(v float64) string {
	rounded := e.roundNumberForJSON(v)
	abs := math.Abs(rounded)
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		str := strconv.FormatFloat(rounded, 'g', -1, 64)
		// Remove leading zero from exponent: 1e-07 → 1e-7
		str = strings.ReplaceAll(str, "e-0", "e-")
		str = strings.ReplaceAll(str, "e+0", "e+")
		str = strings.ReplaceAll(str, "E-0", "E-")
		str = strings.ReplaceAll(str, "E+0", "E+")
		return str
	}

	str := strconv.FormatFloat(rounded, 'f', 15, 64)

	// Handle floating-point artifacts: if we see patterns like ...9999... or ...0000...
	// these are likely precision errors. Common patterns:
	// 90.569999999999993 → should be 90.57
	// 245.789999999999992 → should be 245.79
	str = e.cleanFloatingPointArtifacts(str, rounded)

	str = strings.TrimRight(str, "0")
	str = strings.TrimRight(str, ".")
	if str == "" || str == "-0" {
		return "0"
	}
	return str
}

// cleanFloatingPointArtifacts removes floating-point representation errors.
// E.g., 90.569999999999993 → 90.57, 245.789999999999992 → 245.79
func (e *Evaluator) cleanFloatingPointArtifacts(str string, rounded float64) string {
	// Look for patterns of many repeated 9s or 0s
	// Pattern: find '9999' (4 or more 9s) or '0000' (4 or more 0s) as indicators of floating-point errors
	if idx := strings.Index(str, "9999"); idx >= 0 {
		// Try rounding to fewer decimal places
		parts := strings.Split(str, ".")
		if len(parts) == 2 {
			// Round up at the position before the 9s
			decimalPos := idx - len(parts[0]) - 1
			if decimalPos > 0 && decimalPos < len(parts[1]) {
				// Round to one less decimal place
				factor := math.Pow(10, float64(decimalPos))
				roundedUp := math.Round(rounded*factor) / factor
				return strconv.FormatFloat(roundedUp, 'f', decimalPos, 64)
			}
		}
	} else if idx := strings.Index(str, "0000"); idx >= 0 && idx > len(strings.Split(str, ".")[0]) {
		// For patterns like ...000001, truncate
		parts := strings.Split(str, ".")
		if len(parts) == 2 {
			decimalPos := idx - len(parts[0]) - 1
			if decimalPos > 0 && decimalPos < len(parts[1]) {
				factor := math.Pow(10, float64(decimalPos))
				roundedDown := math.Round(rounded*factor) / factor
				return strconv.FormatFloat(roundedDown, 'f', decimalPos, 64)
			}
		}
	}
	return str
}

// Arithmetic operators

func (e *Evaluator) opAdd(left, right interface{}) (interface{}, error) {
	l, err := e.toNumber(left)
	if err != nil {
		return nil, err
	}
	r, err := e.toNumber(right)
	if err != nil {
		return nil, err
	}
	return l + r, nil
}

func (e *Evaluator) opSubtract(left, right interface{}) (interface{}, error) {
	l, err := e.toNumber(left)
	if err != nil {
		return nil, err
	}
	r, err := e.toNumber(right)
	if err != nil {
		return nil, err
	}
	return l - r, nil
}

func (e *Evaluator) opMultiply(left, right interface{}) (interface{}, error) {
	l, err := e.toNumber(left)
	if err != nil {
		return nil, err
	}
	r, err := e.toNumber(right)
	if err != nil {
		return nil, err
	}
	return l * r, nil
}

func (e *Evaluator) opDivide(left, right interface{}) (interface{}, error) {
	l, err := e.toNumber(left)
	if err != nil {
		return nil, err
	}
	r, err := e.toNumber(right)
	if err != nil {
		return nil, err
	}
	if r == 0 {
		// Division by zero produces infinity
		return math.Inf(1), nil
	}
	return l / r, nil
}

func (e *Evaluator) opModulo(left, right interface{}) (interface{}, error) {
	l, err := e.toNumber(left)
	if err != nil {
		return nil, err
	}
	r, err := e.toNumber(right)
	if err != nil {
		return nil, err
	}
	if r == 0 {
		return nil, fmt.Errorf("modulo by zero")
	}
	return math.Mod(l, r), nil
}

func (e *Evaluator) opNegate(operand interface{}) (interface{}, error) {
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
	if evalCtx == nil || evalCtx.Parent() == nil {
		return nil, nil // No parent context
	}
	return evalCtx.Parent().Data(), nil
}

// compareValues compares two values and returns:
// -1 if left < right
//
//	0 if left == right
//	1 if left > right
//
// This is used for sorting operations.
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
