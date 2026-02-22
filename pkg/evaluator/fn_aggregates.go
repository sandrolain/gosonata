package evaluator

import (
	"context"

	"github.com/sandrolain/gosonata/pkg/types"
)

func fnSum(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	sum := 0.0
	for _, v := range arr {
		num, err := e.toNumber(v)
		if err != nil {
			return nil, err
		}
		sum += num
	}

	return sum, nil
}

func fnCount(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return 0.0, nil
	}

	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	return float64(len(arr)), nil
}

func fnAverage(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	if len(arr) == 0 {
		return nil, nil
	}

	// Type checking: all elements must be numbers
	for _, v := range arr {
		if _, ok := v.(float64); !ok {
			return nil, types.NewError("T0412", "Argument of function 'average' must be an array of numbers", -1)
		}
	}

	sum := 0.0
	for _, v := range arr {
		num, err := e.toNumber(v)
		if err != nil {
			return nil, err
		}
		sum += num
	}

	return sum / float64(len(arr)), nil
}

func fnMin(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	if len(arr) == 0 {
		return nil, nil
	}

	// Type checking: all elements must be numbers
	for _, v := range arr {
		if _, ok := v.(float64); !ok {
			return nil, types.NewError("T0412", "Argument of function 'min' must be an array of numbers", -1)
		}
	}

	min, err := e.toNumber(arr[0])
	if err != nil {
		return nil, err
	}

	for i := 1; i < len(arr); i++ {
		num, err := e.toNumber(arr[i])
		if err != nil {
			return nil, err
		}
		if num < min {
			min = num
		}
	}

	return min, nil
}

func fnMax(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	if len(arr) == 0 {
		return nil, nil
	}

	// Type checking: all elements must be numbers
	for _, v := range arr {
		if _, ok := v.(float64); !ok {
			return nil, types.NewError("T0412", "Argument of function 'max' must be an array of numbers", -1)
		}
	}

	max, err := e.toNumber(arr[0])
	if err != nil {
		return nil, err
	}

	for i := 1; i < len(arr); i++ {
		num, err := e.toNumber(arr[i])
		if err != nil {
			return nil, err
		}
		if num > max {
			max = num
		}
	}

	return max, nil
}

// --- Array Functions ---

// callHOFFn calls a HOF function (Lambda or FunctionDef) with the provided args.
// For Lambda: trims args to match the number of lambda params.
// For FunctionDef: passes all args, trimming to MaxArgs if needed.
// For built-in functions that accept context (AcceptsContext + MinArgs=0),
// only the first (value) arg is passed — extra HOF positional args (index, total) are dropped.
