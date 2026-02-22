package evaluator

import (
	"context"
	"fmt"
	"sort"
)

func fnEach(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	obj := args[0]
	if obj == nil {
		return []interface{}{}, nil
	}

	fnArg := args[1]

	var keys []string
	var values map[string]interface{}

	// Handle OrderedObject to preserve key order
	if orderedObj, ok := obj.(*OrderedObject); ok {
		keys = orderedObj.Keys
		values = orderedObj.Values
	} else if mapObj, ok := obj.(map[string]interface{}); ok {
		// For regular maps, sort keys for consistent ordering
		keys = make([]string, 0, len(mapObj))
		for k := range mapObj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		values = mapObj
	} else {
		return nil, fmt.Errorf("first argument to $each must be an object")
	}

	result := make([]interface{}, 0, len(keys))
	for _, key := range keys {
		value := values[key]
		var itemResult interface{}
		var err error

		switch fn := fnArg.(type) {
		case *Lambda:
			// Pass args based on how many params the lambda expects
			var callArgs []interface{}
			switch len(fn.Params) {
			case 1:
				callArgs = []interface{}{value}
			case 2:
				callArgs = []interface{}{value, key}
			default: // 3+
				callArgs = []interface{}{value, key, obj}
			}
			itemResult, err = e.callLambda(ctx, fn, callArgs)
		case *FunctionDef:
			// Call with (value, key) respecting function's max args
			var callArgs []interface{}
			if fn.MaxArgs == 1 {
				callArgs = []interface{}{value}
			} else if fn.MaxArgs < 0 || fn.MaxArgs >= 3 {
				callArgs = []interface{}{value, key, obj}
			} else {
				callArgs = []interface{}{value, key}
			}
			itemResult, err = fn.Impl(ctx, e, evalCtx, callArgs)
		default:
			return nil, fmt.Errorf("second argument to $each must be a function")
		}

		if err != nil {
			return nil, err
		}
		// Skip undefined results
		if itemResult != nil {
			result = append(result, itemResult)
		}
	}

	return result, nil
}

// fnSift filters an object's key-value pairs using a predicate function.
// Signature: $sift(obj, function($v, $k?, $o?) → boolean)
// Returns a new object with only the key-value pairs where the function returns true.

func fnSift(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	obj := args[0]
	if obj == nil {
		return nil, nil
	}

	// If obj is an array, map sift over each element (path context mapping)
	if arr, ok := obj.([]interface{}); ok {
		results := make([]interface{}, 0, len(arr))
		for _, elem := range arr {
			if elem == nil {
				continue
			}
			res, err := fnSift(ctx, e, evalCtx, []interface{}{elem, args[1]})
			if err != nil {
				return nil, err
			}
			if res != nil {
				results = append(results, res)
			}
		}
		if len(results) == 0 {
			return nil, nil
		}
		return results, nil
	}

	fnArg := args[1]

	var keys []string
	var values map[string]interface{}

	// Handle OrderedObject to preserve key order
	if orderedObj, ok := obj.(*OrderedObject); ok {
		keys = orderedObj.Keys
		values = orderedObj.Values
	} else if mapObj, ok := obj.(map[string]interface{}); ok {
		keys = make([]string, 0, len(mapObj))
		for k := range mapObj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		values = mapObj
	} else {
		// Non-object: return nil (undefined) when used in path context over mixed arrays
		return nil, nil
	}

	// Build result as OrderedObject to preserve order
	resultObj := &OrderedObject{
		Keys:   make([]string, 0),
		Values: make(map[string]interface{}),
	}

	for _, key := range keys {
		value := values[key]
		var include interface{}
		var err error

		switch fn := fnArg.(type) {
		case *Lambda:
			var callArgs []interface{}
			switch len(fn.Params) {
			case 1:
				callArgs = []interface{}{value}
			case 2:
				callArgs = []interface{}{value, key}
			default: // 3+
				callArgs = []interface{}{value, key, obj}
			}
			include, err = e.callLambda(ctx, fn, callArgs)
		case *FunctionDef:
			var callArgs []interface{}
			if fn.MaxArgs == 1 {
				callArgs = []interface{}{value}
			} else if fn.MaxArgs < 0 || fn.MaxArgs >= 3 {
				callArgs = []interface{}{value, key, obj}
			} else {
				callArgs = []interface{}{value, key}
			}
			include, err = fn.Impl(ctx, e, evalCtx, callArgs)
		default:
			return nil, fmt.Errorf("second argument to $sift must be a function")
		}

		if err != nil {
			return nil, err
		}

		if e.isTruthy(include) {
			resultObj.Keys = append(resultObj.Keys, key)
			resultObj.Values[key] = value
		}
	}

	if len(resultObj.Keys) == 0 {
		return nil, nil
	}

	return resultObj, nil
}

// fnRandom returns a pseudo-random number between 0 (inclusive) and 1 (exclusive).

func fnKeys(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return []interface{}{}, nil
	}

	// Handle undefined
	if args[0] == nil {
		return nil, nil
	}

	arg := args[0]
	result := make([]interface{}, 0)

	switch v := arg.(type) {
	case []interface{}:
		// Merge the keys of all items in the array, preserving order of first appearance
		seen := make(map[string]bool)
		var keys []string
		for _, item := range v {
			if allkeys, err := fnKeys(ctx, e, evalCtx, []interface{}{item}); err != nil {
				return nil, err
			} else if allkeys != nil {
				if arr, ok := allkeys.([]interface{}); ok {
					for _, key := range arr {
						if keyStr, ok := key.(string); ok {
							if !seen[keyStr] {
								seen[keyStr] = true
								keys = append(keys, keyStr)
							}
						}
					}
				}
			}
		}
		for _, key := range keys {
			result = append(result, key)
		}
	case *OrderedObject:
		for _, k := range v.Keys {
			result = append(result, k)
		}
	case map[string]interface{}:
		for key := range v {
			result = append(result, key)
		}
	}

	if len(result) == 0 {
		return nil, nil
	}

	// Unwrap singleton arrays per JSONata semantics
	if len(result) == 1 {
		return result[0], nil
	}

	return result, nil
}

// fnLookup returns the value associated with a key in an object.
// If the object is an array of objects, searches all and returns all matching values.

func fnLookup(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	keyStr, ok := args[1].(string)
	if !ok {
		keyStr = fmt.Sprint(args[1])
	}

	if args[0] == nil {
		return nil, nil
	}

	// Handle single object
	if orderedObj, ok := args[0].(*OrderedObject); ok {
		if val, found := orderedObj.Get(keyStr); found {
			return val, nil
		}
		return nil, nil
	}

	if mapObj, ok := args[0].(map[string]interface{}); ok {
		if val, found := mapObj[keyStr]; found {
			return val, nil
		}
		return nil, nil
	}

	// Handle array of objects
	if arr, ok := args[0].([]interface{}); ok {
		results := make([]interface{}, 0)
		for _, item := range arr {
			if orderedObj, ok := item.(*OrderedObject); ok {
				if val, found := orderedObj.Get(keyStr); found {
					results = append(results, val)
				}
			} else if mapObj, ok := item.(map[string]interface{}); ok {
				if val, found := mapObj[keyStr]; found {
					results = append(results, val)
				}
			}
		}
		if len(results) == 0 {
			return nil, nil
		}
		if len(results) == 1 {
			return results[0], nil
		}
		return results, nil
	}

	return nil, nil
}

// fnMerge merges an array of objects into a single object.

func fnMerge(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// undefined inputs return undefined
	if args[0] == nil {
		return nil, nil
	}

	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	result := &OrderedObject{
		Keys:   make([]string, 0),
		Values: make(map[string]interface{}),
	}

	for _, item := range arr {
		if orderedObj, ok := item.(*OrderedObject); ok {
			for _, k := range orderedObj.Keys {
				if _, exists := result.Values[k]; !exists {
					result.Keys = append(result.Keys, k)
				}
				result.Values[k] = orderedObj.Values[k]
			}
		} else if mapObj, ok := item.(map[string]interface{}); ok {
			for k, v := range mapObj {
				if _, exists := result.Values[k]; !exists {
					result.Keys = append(result.Keys, k)
				}
				result.Values[k] = v
			}
		} else {
			return nil, fmt.Errorf("cannot merge non-object item")
		}
	}

	return result, nil
}

// fnSpread splits object/array into array of single key/value pair objects.
// For non-array non-object values (including lambdas), returns the value as-is.

func fnSpread(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}

	return fnSpreadRecursive(ctx, e, evalCtx, args[0])
}

// fnSpreadRecursive is the recursive implementation of spread

func fnSpreadRecursive(ctx context.Context, e *Evaluator, evalCtx *EvalContext, arg interface{}) (interface{}, error) {
	result := make([]interface{}, 0)

	switch v := arg.(type) {
	case []interface{}:
		// spread all items in the array
		for _, item := range v {
			spreadItem, err := fnSpreadRecursive(ctx, e, evalCtx, item)
			if err != nil {
				return nil, err
			}
			if arr, ok := spreadItem.([]interface{}); ok {
				result = append(result, arr...)
			} else if spreadItem != nil {
				result = append(result, spreadItem)
			}
		}
	case *OrderedObject:
		// Create single-key objects for each property
		for _, k := range v.Keys {
			item := map[string]interface{}{
				k: v.Values[k],
			}
			result = append(result, item)
		}
	case map[string]interface{}:
		// Create single-key objects for each property
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			item := map[string]interface{}{
				k: v[k],
			}
			result = append(result, item)
		}
	default:
		// For non-array, non-object values (including lambdas), return as-is
		return arg, nil
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// fnError throws an error with optional message.
// Signature: $error([message])
