package evaluator

import (
	"context"
	"fmt"
	"math"
	"math/rand"
)

func fnAbs(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}
	return math.Abs(num), nil
}

func fnFloor(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}
	return math.Floor(num), nil
}

func fnCeil(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}
	return math.Ceil(num), nil
}

// roundBankers implements banker's rounding (round half to even)
// This matches JSONata's rounding behavior

func roundBankers(num float64, decimals int) float64 {
	if math.IsNaN(num) || math.IsInf(num, 0) {
		return num
	}

	shift := math.Pow(10, float64(decimals))
	shifted := num * shift

	// Get the integer and fractional parts
	floor := math.Floor(shifted)
	frac := shifted - floor

	// Check if we're exactly at 0.5
	if math.Abs(frac-0.5) < 1e-10 {
		// Round to nearest even
		if int64(floor)%2 == 0 {
			return floor / shift
		}
		return (floor + 1) / shift
	}

	// For other cases, use standard rounding (round half away from zero)
	return math.Round(shifted) / shift
}

func fnRound(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}

	if len(args) == 1 {
		return roundBankers(num, 0), nil
	}

	if args[1] == nil {
		return nil, nil
	}
	precision, err := e.toNumber(args[1])
	if err != nil {
		return nil, err
	}

	decimals := int(precision)
	return roundBankers(num, decimals), nil
}

func fnSqrt(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}
	result := math.Sqrt(num)
	if math.IsNaN(result) {
		return nil, fmt.Errorf("D3060: Sqrt function: out of domain (num=%v)", num)
	}
	return result, nil
}

func fnPower(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil || args[1] == nil {
		return nil, nil
	}
	base, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}

	exponent, err := e.toNumber(args[1])
	if err != nil {
		return nil, err
	}

	result := math.Pow(base, exponent)

	// Check for domain errors (NaN or Inf)
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return nil, fmt.Errorf("D3061: Power function: out of domain (base=%v, exponent=%v)", base, exponent)
	}

	return result, nil
}

// --- Object Functions ---

// fnEach returns an array containing the results of calling a function on each key-value pair of an object.
// Signature: $each(object, function)
// The function is invoked with two arguments: the property value and the property name.
// Returns results in key order.

func fnRandom(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	return rand.Float64(), nil
}

// --- Object Functions ---

// fnKeys returns an array of keys from an object or array of objects.
// For arrays, returns a de-duplicated list of all keys from all items.
