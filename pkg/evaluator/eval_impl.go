package evaluator

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sandrolain/gosonata/pkg/types"
)

// evalNode evaluates an AST node in the given context.
// recurseDepthKey is used to store recursion depth in context.Context.
type recurseDepthKey struct{}

// getRecurseDepth returns the current lambda recursion depth from a context.Context.
func getRecurseDepth(ctx context.Context) int {
	if d, ok := ctx.Value(recurseDepthKey{}).(int); ok {
		return d
	}
	return 0
}

// withRecurseDepth returns a context.Context with incremented recursion depth.
func withRecurseDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, recurseDepthKey{}, depth)
}

func (e *Evaluator) evalNode(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Check recursion depth
	if getRecurseDepth(ctx) > e.opts.MaxDepth {
		return nil, types.NewError(types.ErrUndefinedVariable, "maximum recursion depth exceeded", -1)
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
	case types.NodeDescendant:
		return e.evalDescendent(ctx, node, evalCtx)
	case types.NodeWildcard:
		return e.evalWildcard(ctx, node, evalCtx)
	case types.NodeRegex:
		return e.evalRegex(node)
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
	case types.NodePartial:
		return e.evalPartial(ctx, node, evalCtx)
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

// evalRegex evaluates a regex literal.
func (e *Evaluator) evalRegex(node *types.ASTNode) (interface{}, error) {
	pattern, ok := node.Value.(string)
	if !ok {
		return nil, fmt.Errorf("invalid regex pattern type")
	}

	// Compile the regex pattern (already converted to Go format by lexer)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	return re, nil
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
			} else if subArr, ok := item.([]interface{}); ok {
				// Nested array: recurse into it
				subCtx := evalCtx.NewChildContext(subArr)
				if value, err := e.evalNameString(name, subCtx); err == nil && value != nil {
					if subArrVal, isArr := value.([]interface{}); isArr {
						result = append(result, subArrVal...)
					} else {
						result = append(result, value)
					}
				}
			}
		}
		if len(result) == 0 {
			return nil, nil
		}
		if len(result) == 1 {
			return result[0], nil
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
		data := evalCtx.Data()
		return data, nil
	}

	// $$ refers to root context
	if varName == "$" {
		if evalCtx.Root() != nil {
			return evalCtx.Root().Data(), nil
		}
		// Fallback: if no root, return current context (shouldn't happen)
		return evalCtx.Data(), nil
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
			} else if node.RHS.Type == types.NodeFunction && node.RHS.LHS != nil && node.RHS.LHS.Type == types.NodeLambda {
				// Special case: lambda call in path context
				// The item should be injected as the first argument
				value, err = e.evalFunctionWithContextInjection(ctx, node.RHS, itemCtx, item)
			} else {
				value, err = e.evalNode(ctx, node.RHS, itemCtx)
			}
			if err != nil {
				return nil, err
			}

			// Flatten: if value is an array, append its elements
			// UNLESS the RHS is an array constructor [expression], which preserves structure
			// Otherwise append the value itself (if not nil)
			if value != nil {
				// If RHS is an explicit array constructor, don't flatten the result
				if subArr, isArr := value.([]interface{}); isArr && node.RHS.Type != types.NodeArray {
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
func isComplexType(v interface{}) bool {
	if v == nil {
		return false
	}
	val := reflect.ValueOf(v)
	kind := val.Kind()
	return kind == reflect.Map || kind == reflect.Slice || kind == reflect.Array
}

// deepEqual performs deep equality comparison between two values.
// Handles maps, slices, and primitive types.
func deepEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Use reflection for all comparisons
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)

	// Different types are not equal
	if aVal.Type() != bVal.Type() {
		return false
	}

	switch aVal.Kind() {
	case reflect.Map:
		if aVal.Len() != bVal.Len() {
			return false
		}
		for _, key := range aVal.MapKeys() {
			aElem := aVal.MapIndex(key)
			bElem := bVal.MapIndex(key)
			if !bElem.IsValid() || !deepEqual(aElem.Interface(), bElem.Interface()) {
				return false
			}
		}
		return true

	case reflect.Slice, reflect.Array:
		if aVal.Len() != bVal.Len() {
			return false
		}
		for i := 0; i < aVal.Len(); i++ {
			if !deepEqual(aVal.Index(i).Interface(), bVal.Index(i).Interface()) {
				return false
			}
		}
		return true

	case reflect.String:
		return aVal.String() == bVal.String()

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return aVal.Int() == bVal.Int()

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return aVal.Uint() == bVal.Uint()

	case reflect.Float32, reflect.Float64:
		return aVal.Float() == bVal.Float()

	case reflect.Bool:
		return aVal.Bool() == bVal.Bool()

	default:
		// For other types, use DeepEqual from reflect package
		return reflect.DeepEqual(a, b)
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
	// Check if this is a lambda/variable call (LHS contains lambda or variable) or built-in function call (Value contains name)
	if node.LHS != nil {
		// Lambda or variable call: evaluate first, then call it
		callableValue, err := e.evalNode(ctx, node.LHS, evalCtx)
		if err != nil {
			return nil, err
		}

		// Check what we got
		switch fn := callableValue.(type) {
		case *Lambda:
			// User-defined lambda
			// Evaluate arguments
			args := make([]interface{}, 0, len(node.Arguments))
			for _, argNode := range node.Arguments {
				arg, err := e.evalNode(ctx, argNode, evalCtx)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
			}

			// Call lambda
			return e.callLambda(ctx, fn, args)

		case *FunctionDef:
			// Built-in function (from variable like $not)
			// Evaluate arguments
			args := make([]interface{}, 0, len(node.Arguments))
			for _, argNode := range node.Arguments {
				arg, err := e.evalNode(ctx, argNode, evalCtx)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
			}

			// If function accepts context and we have fewer args than required, prepend context
			if fn.AcceptsContext && len(args) < fn.MinArgs {
				contextData := evalCtx.Data()
				args = append([]interface{}{contextData}, args...)
			}

			// Validate argument count
			if len(args) < fn.MinArgs {
				return nil, types.NewError(types.ErrArgumentCountMismatch,
					fmt.Sprintf("function requires at least %d arguments, got %d", fn.MinArgs, len(args)), -1)
			}
			if fn.MaxArgs != -1 && len(args) > fn.MaxArgs {
				return nil, types.NewError(types.ErrArgumentCountMismatch,
					fmt.Sprintf("function accepts at most %d arguments, got %d", fn.MaxArgs, len(args)), -1)
			}

			// Call function
			return fn.Impl(ctx, e, evalCtx, args)

		default:
			return nil, fmt.Errorf("expected lambda or function, got %T", callableValue)
		}
	}

	// Built-in function call
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

	// If function accepts context and we have fewer args than required, prepend context
	if fnDef.AcceptsContext && len(args) < fnDef.MinArgs {
		contextData := evalCtx.Data()
		args = append([]interface{}{contextData}, args...)
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

// evalFunctionWithContextInjection evaluates a lambda call with optional context injection.
// This is used when a lambda is called in a path context (e.g., Age.function($x,$y){...}(arg))
// The contextValue is prepended to the arguments ONLY if the lambda needs more arguments.
func (e *Evaluator) evalFunctionWithContextInjection(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext, contextValue interface{}) (interface{}, error) {
	// node.LHS should be a lambda
	if node.LHS == nil || node.LHS.Type != types.NodeLambda {
		return nil, fmt.Errorf("expected lambda in function call with context injection")
	}

	// Evaluate lambda
	lambdaValue, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	lambda, ok := lambdaValue.(*Lambda)
	if !ok {
		return nil, fmt.Errorf("expected lambda function, got %T", lambdaValue)
	}

	// Evaluate explicit arguments
	explicitArgs := make([]interface{}, 0, len(node.Arguments))
	for _, argNode := range node.Arguments {
		arg, err := e.evalNode(ctx, argNode, evalCtx)
		if err != nil {
			return nil, err
		}
		explicitArgs = append(explicitArgs, arg)
	}

	// Determine if we need to inject context
	// Inject context value as first argument ONLY if we have fewer args than params
	var args []interface{}
	if len(explicitArgs) < len(lambda.Params) {
		// Need context injection
		args = make([]interface{}, 0, len(explicitArgs)+1)
		args = append(args, contextValue)
		args = append(args, explicitArgs...)
	} else {
		// Already have enough args, use them as-is
		args = explicitArgs
	}

	// Call lambda with (possibly injected) context
	return e.callLambda(ctx, lambda, args)
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

	// Parse signature if present
	var sig *Signature
	if node.Signature != "" {
		parsedSig, err := ParseSignature(node.Signature)
		if err != nil {
			// Return S0401 error for invalid signature
			return nil, err
		}
		sig = parsedSig
	}

	// Create new context with lambda's closure context as parent.
	// We store evalCtx directly (not cloned) so that the lambda can see
	// bindings added AFTER lambda creation in the same block scope (enables recursion).
	// callLambda() creates its own clone of this context at call time.
	lambda := &Lambda{
		Params:    params,
		Body:      node.RHS, // Body is in RHS
		Ctx:       evalCtx,
		Signature: sig,
	}

	return lambda, nil
}

// evalPartial creates a partial application lambda.
// When a function is called with placeholder arguments (?), it returns a new
// lambda that accepts values for those placeholders.
func (e *Evaluator) evalPartial(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Count placeholders and build parameter list
	placeholderCount := 0
	for _, arg := range node.Arguments {
		if arg.Type == types.NodePlaceholder {
			placeholderCount++
		}
	}

	if placeholderCount == 0 {
		// No placeholders - should not happen, but treat as regular function call
		return e.evalFunction(ctx, node, evalCtx)
	}

	// Check if partial application is allowed
	// It's only allowed when calling through a variable/lambda (node.LHS != nil)
	// Direct function calls (node.Value is string) are not allowed
	if node.LHS == nil && node.Value != nil {
		// Direct function call with placeholder
		funcName, ok := node.Value.(string)
		if !ok {
			return nil, types.NewError("T1007", "partial application can only be applied to a function", node.Position)
		}

		// Check if function exists
		if _, exists := GetFunction(funcName); !exists {
			return nil, types.NewError("T1008", fmt.Sprintf("attempted partial application of unknown function: %s", funcName), node.Position)
		}

		// Function exists but partial application is not supported for direct calls
		return nil, types.NewError("T1007", "partial application can only be applied to a function", node.Position)
	}

	// When LHS is set, evaluate it to check if it's callable
	if node.LHS != nil {
		lhsVal, err := e.evalNode(ctx, node.LHS, evalCtx)
		if err != nil {
			return nil, err
		}
		switch lhsVal.(type) {
		case *Lambda, *FunctionDef:
			// OK, callable
		default:
			return nil, types.NewError("T1007", "partial application can only be applied to a function", node.Position)
		}
	}

	// Create parameter names for the lambda ($1, $2, $3, ...)
	params := make([]string, placeholderCount)
	for i := 0; i < placeholderCount; i++ {
		params[i] = fmt.Sprintf("%d", i+1)
	}

	// Build the body: a function call with placeholders replaced by variables
	bodyNode := types.NewASTNode(types.NodeFunction, node.Position)
	bodyNode.Value = node.Value
	bodyNode.LHS = node.LHS
	bodyNode.Arguments = make([]*types.ASTNode, len(node.Arguments))

	placeholderIndex := 0
	for i, arg := range node.Arguments {
		if arg.Type == types.NodePlaceholder {
			// Replace placeholder with variable reference
			varNode := types.NewASTNode(types.NodeVariable, arg.Position)
			varNode.Value = params[placeholderIndex]
			bodyNode.Arguments[i] = varNode
			placeholderIndex++
		} else {
			// Keep non-placeholder arguments as-is
			bodyNode.Arguments[i] = arg
		}
	}

	// Create lambda
	lambda := &Lambda{
		Params: params,
		Body:   bodyNode,
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
func (e *Evaluator) evalApply(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Evaluate left side (the data to pipe)
	data, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// Special case: if data is a function, check for function composition
	isDataFunction := false
	switch data.(type) {
	case *Lambda, *FunctionDef:
		isDataFunction = true
	}

	// If data is a function and RHS evaluates to a function, create composed function
	if isDataFunction {
		// Evaluate RHS to check if it's also a function
		var rhsFunc interface{}
		if node.RHS.Type == types.NodeVariable || node.RHS.Type == types.NodePartial {
			// Variable or partial application that should resolve to a function
			rhsFunc, err = e.evalNode(ctx, node.RHS, evalCtx)
			if err != nil {
				return nil, err
			}
		} else if node.RHS.Type == types.NodeFunction && node.RHS.LHS != nil {
			// Function call through variable
			rhsFunc, err = e.evalNode(ctx, node.RHS, evalCtx)
			if err != nil {
				return nil, err
			}
		}

		// Check if RHS is a function
		if rhsFunc != nil {
			switch rhsFunc.(type) {
			case *Lambda, *FunctionDef:
				// Create function composition: f ~> g creates λx.g(f(x))
				return e.createComposition(data, rhsFunc, evalCtx), nil
			}
		}
	}

	// Check if RHS is a NodeFilter wrapping a function call (e.g., $map($fn)[])
	// In this case, inject data into the inner function call, then apply the filter
	if node.RHS.Type == types.NodeFilter && node.RHS.LHS != nil && node.RHS.LHS.Type == types.NodeFunction {
		innerFnNode := node.RHS.LHS
		filterNode := node.RHS

		// Evaluate the inner function call with data prepended
		var innerResult interface{}
		if innerFnNode.LHS != nil {
			// Variable/lambda call - evaluate the callable and call with data prepended
			callableValue, err := e.evalNode(ctx, innerFnNode.LHS, evalCtx)
			if err != nil {
				return nil, err
			}
			args := make([]interface{}, 0, len(innerFnNode.Arguments)+1)
			args = append(args, data)
			for _, argNode := range innerFnNode.Arguments {
				arg, err := e.evalNode(ctx, argNode, evalCtx)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
			}
			switch fn := callableValue.(type) {
			case *Lambda:
				innerResult, err = e.callLambda(ctx, fn, args)
			case *FunctionDef:
				if len(args) < fn.MinArgs {
					return nil, types.NewError(types.ErrArgumentCountMismatch,
						fmt.Sprintf("function requires at least %d arguments, got %d", fn.MinArgs, len(args)), -1)
				}
				innerResult, err = fn.Impl(ctx, e, evalCtx, args)
			default:
				return nil, fmt.Errorf("expected lambda or function, got %T", callableValue)
			}
			if err != nil {
				return nil, err
			}
		} else if innerFnNode.Value != nil {
			// Named function call
			funcName := innerFnNode.Value.(string)
			fnDef, ok := GetFunction(funcName)
			if !ok {
				return nil, fmt.Errorf("unknown function: %s", funcName)
			}
			args := make([]interface{}, 0, len(innerFnNode.Arguments)+1)
			args = append(args, data)
			for _, argNode := range innerFnNode.Arguments {
				arg, err := e.evalNode(ctx, argNode, evalCtx)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
			}
			if len(args) < fnDef.MinArgs {
				return nil, types.NewError(types.ErrArgumentCountMismatch,
					fmt.Sprintf("function requires at least %d arguments, got %d", fnDef.MinArgs, len(args)), -1)
			}
			innerResult, err = fnDef.Impl(ctx, e, evalCtx, args)
			if err != nil {
				return nil, err
			}
		}

		// Now apply the filter/keep-array operation to the inner result
		// We DON'T call evalFilter(filterNode) because that would re-evaluate filterNode.LHS.
		// Instead, apply the filter directly to innerResult.
		if filterNode.RHS == nil {
			// Empty filter [] means "return as array" (KeepArray)
			arr, err := e.toArray(innerResult)
			if err != nil {
				return nil, err
			}
			if len(arr) == 0 {
				return nil, nil
			}
			return arr, nil
		}
		// Non-empty filter: apply predicate to innerResult
		innerArr, err := e.toArray(innerResult)
		if err != nil {
			return nil, err
		}
		// Apply filter predicate similar to evalFilter but using innerArr directly
		var filterResult []interface{}
		for i, item := range innerArr {
			itemCtx := evalCtx.NewChildContext(item)
			itemCtx.SetBinding("", item) // Set $ to item
			// Evaluate filter predicate
			predVal, err := e.evalNode(ctx, filterNode.RHS, itemCtx)
			if err != nil {
				return nil, err
			}
			// Check if predicate is a number (index)
			if idx, ok := predVal.(float64); ok {
				if int(idx) == i {
					filterResult = append(filterResult, item)
				}
			} else if e.isTruthy(predVal) {
				filterResult = append(filterResult, item)
			}
		}
		if len(filterResult) == 0 {
			return nil, nil
		}
		if len(filterResult) == 1 {
			return filterResult[0], nil
		}
		return filterResult, nil
	}

	// Check if RHS is a function call
	if node.RHS.Type == types.NodeFunction {
		// It's a function call - inject data as first argument
		fnNode := node.RHS

		// If it's a built-in function call (Value contains name)
		if fnNode.Value != nil {
			funcName := fnNode.Value.(string)
			fnDef, ok := GetFunction(funcName)
			if !ok {
				return nil, fmt.Errorf("unknown function: %s", funcName)
			}

			// Evaluate existing arguments
			args := make([]interface{}, 0, len(fnNode.Arguments)+1)
			args = append(args, data) // Prepend piped data

			for _, argNode := range fnNode.Arguments {
				arg, err := e.evalNode(ctx, argNode, evalCtx)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
			}

			// Validate argument count
			if len(args) < fnDef.MinArgs {
				return nil, types.NewError(types.ErrArgumentCountMismatch,
					fmt.Sprintf("function requires at least %d arguments, got %d", fnDef.MinArgs, len(args)), -1)
			}
			if fnDef.MaxArgs != -1 && len(args) > fnDef.MaxArgs {
				return nil, types.NewError(types.ErrArgumentCountMismatch,
					fmt.Sprintf("function accepts at most %d arguments, got %d", fnDef.MaxArgs, len(args)), -1)
			}

			// Call the function
			return fnDef.Impl(ctx, e, evalCtx, args)
		}

		// If it's a lambda/variable function call (LHS contains callable)
		if fnNode.LHS != nil {
			callableValue, err := e.evalNode(ctx, fnNode.LHS, evalCtx)
			if err != nil {
				return nil, err
			}

			// Evaluate arguments
			args := make([]interface{}, 0, len(fnNode.Arguments)+1)
			args = append(args, data) // Prepend piped data

			for _, argNode := range fnNode.Arguments {
				arg, err := e.evalNode(ctx, argNode, evalCtx)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
			}

			// Call based on type
			switch fn := callableValue.(type) {
			case *Lambda:
				return e.callLambda(ctx, fn, args)
			case *FunctionDef:
				return fn.Impl(ctx, e, evalCtx, args)
			default:
				return nil, fmt.Errorf("expected lambda or function, got %T", callableValue)
			}
		}
	}

	// RHS is not a function call - evaluate it and expect a lambda or regex
	fn, err := e.evalNode(ctx, node.RHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// If fn is a regex, apply it to data as a match test
	if regex, ok := fn.(*regexp.Regexp); ok {
		// Convert data to string
		str, ok := data.(string)
		if !ok {
			str = fmt.Sprint(data)
		}
		return regex.MatchString(str), nil
	}

	// If fn is a lambda, call it with data as argument
	if lambda, ok := fn.(*Lambda); ok {
		return e.callLambda(ctx, lambda, []interface{}{data})
	}

	// If fn is a function definition, call it
	if fnDef, ok := fn.(*FunctionDef); ok {
		return fnDef.Impl(ctx, e, evalCtx, []interface{}{data})
	}

	return nil, types.NewError(types.ErrInvokeNonFunction, "right side of ~> must be a function", -1)
}

// createComposition creates a composed function from two functions.
// composition(f, g) returns λx.g(f(x))
func (e *Evaluator) createComposition(leftFn, rightFn interface{}, evalCtx *EvalContext) *Lambda {
	// Create a lambda that accepts one parameter and applies both functions
	bodyNode := types.NewASTNode(types.NodeFunction, 0)

	// The body calls rightFn with the result of calling leftFn
	// First, call leftFn with the parameter
	leftCallNode := types.NewASTNode(types.NodeFunction, 0)
	leftCallNode.LHS = &types.ASTNode{
		Type:  types.NodeVariable,
		Value: "leftFn",
	}
	leftCallNode.Arguments = []*types.ASTNode{
		{
			Type:  types.NodeVariable,
			Value: "1", // Parameter name
		},
	}

	// Then call rightFn with the result
	bodyNode.LHS = &types.ASTNode{
		Type:  types.NodeVariable,
		Value: "rightFn",
	}
	bodyNode.Arguments = []*types.ASTNode{leftCallNode}

	// Create context with both functions bound
	composedCtx := evalCtx.Clone()
	composedCtx.SetBinding("leftFn", leftFn)
	composedCtx.SetBinding("rightFn", rightFn)

	return &Lambda{
		Params: []string{"1"},
		Body:   bodyNode,
		Ctx:    composedCtx,
	}
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

		itemCtx := evalCtx.NewChildContext(item)
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

// callLambda calls a lambda function with the given arguments.
func (e *Evaluator) callLambda(ctx context.Context, lambda *Lambda, args []interface{}) (interface{}, error) {
	// Check for undefined arguments - if any argument is undefined (nil),
	// the result is undefined per JSONata spec
	for _, arg := range args {
		if arg == nil {
			return nil, nil // undefined propagates
		}
	}

	// Validate signature if present
	if lambda.Signature != nil {
		// Count required parameters (non-optional)
		requiredCount := 0
		for _, param := range lambda.Signature.Params {
			if !param.Optional {
				requiredCount++
			}
		}

		// Check argument count (must be between required and total params)
		if len(args) < requiredCount || len(args) > len(lambda.Signature.Params) {
			if requiredCount == len(lambda.Signature.Params) {
				return nil, fmt.Errorf("lambda expects %d arguments, got %d", len(lambda.Signature.Params), len(args))
			} else {
				return nil, fmt.Errorf("lambda expects %d-%d arguments, got %d", requiredCount, len(lambda.Signature.Params), len(args))
			}
		}

		// Apply auto-wrapping and validate each argument
		for i := range args {
			param := lambda.Signature.Params[i]

			// Auto-wrap: if parameter expects array but arg is not array, wrap it
			if param.Type == TypeArray {
				if _, isArray := args[i].([]interface{}); !isArray {
					args[i] = []interface{}{args[i]}
				}
			}

			// Validate argument against parameter type
			if err := param.ValidateArgument(args[i]); err != nil {
				return nil, err
			}
		}
	} else {
		// No signature - validate argument count: allow fewer args (missing ones default to nil)
		if len(args) > len(lambda.Params) {
			return nil, fmt.Errorf("lambda expects %d arguments, got %d", len(lambda.Params), len(args))
		}
	}

	// Create new context with lambda's closure context as parent.
	// Clone (not CloneDeeper) - recursion depth is tracked via context.Context key.
	lambdaCtx := lambda.Ctx.Clone()

	// Increment recursion depth in context.Context to catch infinite recursion.
	ctx = withRecurseDepth(ctx, getRecurseDepth(ctx)+1)

	// Bind parameters
	for i, param := range lambda.Params {
		if i < len(args) {
			lambdaCtx.SetBinding(param, args[i])
		}
		// Optional parameters without args remain unbound
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

// isTruthyBoolean implements the $boolean() function semantics:
// - Functions are always false
// - Arrays are true only if they contain at least one truthy element (recursively)
// - All other rules same as isTruthy
func (e *Evaluator) isTruthyBoolean(value interface{}) bool {
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
		// Array is true only if at least one element is truthy (recursively)
		for _, item := range v {
			if e.isTruthyBoolean(item) {
				return true
			}
		}
		return false
	case map[string]interface{}:
		return len(v) > 0
	case *OrderedObject:
		return len(v.Values) > 0
	case *Lambda, *FunctionDef:
		// Functions are always falsy in $boolean() context
		return false
	default:
		return true
	}
}

// isTruthyForDefault determines if a value is truthy for the default operator (?:).
// This has special semantics: arrays are truthy only if they contain at least one truthy value,
// and functions are considered falsy.
func (e *Evaluator) isTruthyForDefault(value interface{}) bool {
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
		// Array is truthy only if it contains at least one truthy element (recursively)
		for _, item := range v {
			if e.isTruthyForDefault(item) {
				return true
			}
		}
		return false
	case map[string]interface{}:
		return len(v) > 0
	case *OrderedObject:
		return len(v.Values) > 0
	case *Lambda:
		// Functions are falsy for the default operator
		return false
	default:
		// Other types (including functions) are considered falsy
		return false
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
		// Arrays, objects, and other types: serialize as JSON
		b, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(b)
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

// requireNumericOperand validates that a value is a numeric type for arithmetic operations.
// Returns T2001 error for non-numeric types (bool, string, object, etc.).
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
