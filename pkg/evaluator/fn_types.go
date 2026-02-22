package evaluator

import (
	"context"
	"strconv"
	"strings"

	"github.com/sandrolain/gosonata/pkg/types"
)

func fnType(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	value := args[0]

	// undefined (nil) returns undefined (not "null")
	if value == nil {
		return nil, nil
	}

	// Check for JSONata null (types.Null) - returns "null"
	if _, ok := value.(types.Null); ok {
		return "null", nil
	}

	switch value.(type) {
	case string:
		return "string", nil
	case float64:
		return "number", nil
	case bool:
		return "boolean", nil
	case []interface{}:
		return "array", nil
	case map[string]interface{}:
		return "object", nil
	case *OrderedObject:
		return "object", nil
	case *Lambda:
		return "function", nil
	default:
		return "unknown", nil
	}
}

func fnExists(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	return args[0] != nil, nil
}

func fnNumber(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// undefined inputs return undefined
	if args[0] == nil {
		return nil, nil
	}
	if str, ok := args[0].(string); ok {
		if num, err := strconv.ParseFloat(str, 64); err == nil {
			return num, nil
		}
		if strings.HasPrefix(str, "0x") || strings.HasPrefix(str, "0X") {
			if num, err := strconv.ParseInt(str[2:], 16, 64); err == nil {
				return float64(num), nil
			}
		}
		if strings.HasPrefix(str, "0o") || strings.HasPrefix(str, "0O") {
			if num, err := strconv.ParseInt(str[2:], 8, 64); err == nil {
				return float64(num), nil
			}
		}
		if strings.HasPrefix(str, "0b") || strings.HasPrefix(str, "0B") {
			if num, err := strconv.ParseInt(str[2:], 2, 64); err == nil {
				return float64(num), nil
			}
		}
	}

	return e.toNumber(args[0])
}

func fnBoolean(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// Per JSONata spec for $boolean():
	// - undefined → undefined
	// - functions → false
	// - arrays → true only if at least one truthy element (recursively)
	if args[0] == nil {
		return nil, nil // undefined → undefined
	}
	return e.isTruthyBoolean(args[0]), nil
}

func fnNot(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// Special case: not(undefined) → undefined (per JSONata spec)
	if args[0] == nil {
		return nil, nil
	}
	return !e.isTruthy(args[0]), nil
}

// --- Math Functions ---
