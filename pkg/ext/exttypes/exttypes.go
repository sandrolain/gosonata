// Package exttypes provides type predicate and control functions for GoSonata
// beyond the official JSONata spec.
package exttypes

import (
	"context"
	"fmt"

	"github.com/sandrolain/gosonata/pkg/ext/extutil"
	"github.com/sandrolain/gosonata/pkg/functions"
	"github.com/sandrolain/gosonata/pkg/types"
)

// All returns all extended type/control function definitions.
func All() []functions.CustomFunctionDef {
	return []functions.CustomFunctionDef{
		IsString(),
		IsNumber(),
		IsBoolean(),
		IsArray(),
		IsObject(),
		IsNull(),
		IsFunction(),
		IsUndefined(),
		IsEmpty(),
		Default(),
		Identity(),
	}
}

// AllEntries returns all type-predicate function definitions as [functions.FunctionEntry],
// suitable for spreading into [gosonata.WithFunctions].
func AllEntries() []functions.FunctionEntry {
	all := All()
	out := make([]functions.FunctionEntry, len(all))
	for i, f := range all {
		out[i] = f
	}
	return out
}

// IsString returns the definition for $isString(v).
func IsString() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "isString",
		Signature: "<x:b>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			_, ok := args[0].(string)
			return ok, nil
		},
	}
}

// IsNumber returns the definition for $isNumber(v).
func IsNumber() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "isNumber",
		Signature: "<x:b>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			switch args[0].(type) {
			case float64, int, int64:
				return true, nil
			default:
				return false, nil
			}
		},
	}
}

// IsBoolean returns the definition for $isBoolean(v).
func IsBoolean() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "isBoolean",
		Signature: "<x:b>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			_, ok := args[0].(bool)
			return ok, nil
		},
	}
}

// IsArray returns the definition for $isArray(v).
func IsArray() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "isArray",
		Signature: "<x:b>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			_, ok := args[0].([]interface{})
			return ok, nil
		},
	}
}

// IsObject returns the definition for $isObject(v).
func IsObject() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "isObject",
		Signature: "<x:b>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			return extutil.IsObject(args[0]), nil
		},
	}
}

// IsNull returns the definition for $isNull(v).
// Returns true for JSON null (nil in Go).
func IsNull() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "isNull",
		Signature: "<x:b>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			if args[0] == nil {
				return true, nil
			}
			_, isNull := args[0].(types.Null)
			return isNull, nil
		},
	}
}

// IsFunction returns the definition for $isFunction(v).
// Returns true if the value is a callable (lambda or built-in).
func IsFunction() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "isFunction",
		Signature: "<x:b>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			if args[0] == nil {
				return false, nil
			}
			// Functions are represented by internal types not visible here.
			// Use a type-name based check as approximation.
			typeName := fmt.Sprintf("%T", args[0])
			return typeName == "*evaluator.Lambda" ||
				typeName == "*evaluator.FunctionDef" ||
				typeName == "*evaluator.Composition", nil
		},
	}
}

// IsUndefined returns the definition for $isUndefined(v).
// Returns true if the value is nil / undefined.
func IsUndefined() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "isUndefined",
		Signature: "<x:b>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			return args[0] == nil, nil
		},
	}
}

// IsEmpty returns the definition for $isEmpty(v).
// Returns true for "", nil, [], and {}.
func IsEmpty() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "isEmpty",
		Signature: "<x:b>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			switch v := args[0].(type) {
			case nil:
				return true, nil
			case string:
				return v == "", nil
			case []interface{}:
				return len(v) == 0, nil
			case map[string]interface{}:
				return len(v) == 0, nil
			default:
				if n := extutil.ObjectLen(args[0]); n >= 0 {
					return n == 0, nil
				}
				return false, nil
			}
		},
	}
}

// Default returns the definition for $default(value, defaultValue).
// Returns value if it is not nil/undefined, otherwise defaultValue.
func Default() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "default",
		Signature: "<x-x:x>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			if args[0] != nil {
				return args[0], nil
			}
			if len(args) >= 2 {
				return args[1], nil
			}
			return nil, nil
		},
	}
}

// Identity returns the definition for $identity(x).
// Returns its argument unchanged.
func Identity() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "identity",
		Signature: "<x:x>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			if len(args) == 0 {
				return nil, nil
			}
			return args[0], nil
		},
	}
}
