package evaluator

import (
	"context"
	"fmt"
	"sort"

	"github.com/sandrolain/gosonata/pkg/types"
)

func (e *Evaluator) callHOFFn(ctx context.Context, evalCtx *EvalContext, fn interface{}, args []interface{}) (interface{}, error) {
	switch f := fn.(type) {
	case *Lambda:
		callArgs := args
		if len(f.Params) > 0 && len(f.Params) < len(args) {
			callArgs = args[:len(f.Params)]
		}
		return e.callLambda(ctx, f, callArgs)
	case *FunctionDef:
		// Trim to MaxArgs if specified
		callArgs := args
		if f.MaxArgs > 0 && len(callArgs) > f.MaxArgs {
			callArgs = callArgs[:f.MaxArgs]
		}
		// For context-accepting functions with no required args (e.g. $string, $trim),
		// drop extra HOF positional args (index, total) — pass only the item value.
		// This matches JSONata's reference arity-based HOF arg passing.
		if f.AcceptsContext && f.MinArgs == 0 && len(callArgs) > 1 {
			callArgs = callArgs[:1]
		}
		return f.Impl(ctx, e, evalCtx, callArgs)
	default:
		return nil, fmt.Errorf("expected a function, got %T", fn)
	}
}

func fnMap(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}
	if args[1] == nil {
		return nil, fmt.Errorf("second argument to $map must be a function")
	}

	result := make([]interface{}, 0, len(arr))
	for i, item := range arr {
		// OPT-14: use pooled HOF args frame to avoid a []interface{}{...} allocation
		// per iteration. Safe: callHOFFn only reads elements; it never stores the slice.
		f, hofArgs := acquireHOFArgs3(item, float64(i), arr)
		value, err := e.callHOFFn(ctx, evalCtx, args[1], hofArgs)
		releaseHOFArgs(f)
		if err != nil {
			return nil, err
		}
		// Exclude undefined (nil) results - JSONata sequence semantics
		if value != nil {
			result = append(result, value)
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

func fnFilter(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}
	if args[1] == nil {
		return nil, fmt.Errorf("second argument to $filter must be a function")
	}

	result := make([]interface{}, 0)
	for i, item := range arr {
		// OPT-14: pooled HOF args frame
		f, hofArgs := acquireHOFArgs3(item, float64(i), arr)
		value, err := e.callHOFFn(ctx, evalCtx, args[1], hofArgs)
		releaseHOFArgs(f)
		if err != nil {
			return nil, err
		}
		if e.isTruthy(value) {
			result = append(result, item)
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

func fnReduce(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		if len(args) >= 3 {
			return args[2], nil
		}
		return nil, nil
	}
	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}
	if args[1] == nil {
		return nil, fmt.Errorf("second argument to $reduce must be a function")
	}
	// D3050: callback must accept at least 2 args
	switch f := args[1].(type) {
	case *Lambda:
		if len(f.Params) < 2 {
			return nil, types.NewError(types.ErrReduceInsufficientArgs,
				"The second argument of reduce function must be a function with at least two arguments", -1)
		}
	case *FunctionDef:
		if f.MinArgs < 2 {
			return nil, types.NewError(types.ErrReduceInsufficientArgs,
				"The second argument of reduce function must be a function with at least two arguments", -1)
		}
	}

	if len(arr) == 0 {
		if len(args) >= 3 {
			return args[2], nil
		}
		return nil, nil
	}

	var accumulator interface{}
	startIdx := 0

	if len(args) >= 3 && args[2] != nil {
		accumulator = args[2]
	} else {
		accumulator = arr[0]
		startIdx = 1
	}

	for i := startIdx; i < len(arr); i++ {
		// OPT-14: pooled HOF args frame (4 elements: accumulator, current, index, array)
		f, hofArgs := acquireHOFArgs4(accumulator, arr[i], float64(i), arr)
		value, err := e.callHOFFn(ctx, evalCtx, args[1], hofArgs)
		releaseHOFArgs(f)
		if err != nil {
			return nil, err
		}
		accumulator = value
	}

	return accumulator, nil
}

// fnSingle finds the single element in an array matching an optional predicate.
// Throws D3138 if more than one element matches, D3139 if no element matches.

func fnSingle(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	var fn interface{}
	if len(args) >= 2 {
		fn = args[1]
	}

	hasFoundMatch := false
	var result interface{}

	for i, entry := range arr {
		positiveResult := true
		if fn != nil {
			// OPT-14: pooled HOF args frame
			hf, hofArgs := acquireHOFArgs3(entry, float64(i), arr)
			res, err := e.callHOFFn(ctx, evalCtx, fn, hofArgs)
			releaseHOFArgs(hf)
			if err != nil {
				return nil, err
			}
			positiveResult = e.isTruthy(res)
		}
		if positiveResult {
			if !hasFoundMatch {
				result = entry
				hasFoundMatch = true
			} else {
				return nil, types.NewError(types.ErrSingleMultipleMatches,
					"The $single() function expected exactly 1 matching result. Instead it matched more.", -1)
			}
		}
	}

	if !hasFoundMatch {
		return nil, types.NewError(types.ErrSingleNoMatch,
			"The $single() function expected exactly 1 matching result. Instead it matched 0.", -1)
	}

	return result, nil
}

func fnSort(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	if len(arr) == 0 {
		return nil, nil
	}

	// Make a copy to avoid modifying the original
	result := make([]interface{}, len(arr))
	copy(result, arr)

	if len(args) == 1 || args[1] == nil {
		// Default sort: all elements must be the same type (all numbers OR all strings)
		// Otherwise return D3070
		var sortErr error
		sort.SliceStable(result, func(i, j int) bool {
			if sortErr != nil {
				return false
			}
			ni, isNi := result[i].(float64)
			nj, isNj := result[j].(float64)
			si, isSi := result[i].(string)
			sj, isSj := result[j].(string)

			if isNi && isNj {
				return ni < nj
			}
			if isSi && isSj {
				return si < sj
			}
			// Mixed types or non-comparable types (objects, booleans, etc.)
			sortErr = types.NewError(types.ErrTypeMismatch, "D3070 $sort: mixed types in array", -1)
			return false
		})
		if sortErr != nil {
			return nil, sortErr
		}
	} else {
		// Custom sort with comparator function.
		// JSONata convention: fn($a, $b) returns true when $a > $b (a comes AFTER b).
		// Go sort convention: less(i,j) returns true when arr[i] comes BEFORE arr[j].
		// Logic: less(i,j) = true iff $a < $b, i.e. !fn($a,$b) && fn($b,$a)
		var sortErr error
		sort.SliceStable(result, func(i, j int) bool {
			if sortErr != nil {
				return false
			}
			callFn := func(a, b interface{}) (bool, error) {
				var value interface{}
				var err error
				switch fn := args[1].(type) {
				case *Lambda:
					value, err = e.callLambda(ctx, fn, []interface{}{a, b})
				case *FunctionDef:
					value, err = fn.Impl(ctx, e, evalCtx, []interface{}{a, b})
				default:
					return false, fmt.Errorf("second argument to $sort must be a function")
				}
				if err != nil {
					return false, err
				}
				return e.isTruthy(value), nil
			}
			// Check fn($a, $b): if true, a > b → a comes AFTER b → less = false
			fwd, err := callFn(result[i], result[j])
			if err != nil {
				sortErr = err
				return false
			}
			if fwd {
				return false // a > b: a comes after b
			}
			// Check fn($b, $a): if true, b > a → a comes BEFORE b → less = true
			bwd, err := callFn(result[j], result[i])
			if err != nil {
				sortErr = err
				return false
			}
			return bwd // a < b: a comes before b; if equal (both false) → stable
		})
		if sortErr != nil {
			return nil, sortErr
		}
	}

	return result, nil
}
