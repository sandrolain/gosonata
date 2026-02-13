package evaluator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/sandrolain/gosonata/pkg/types"
)

// FunctionDef defines a built-in function.
type FunctionDef struct {
	Name    string
	MinArgs int
	MaxArgs int // -1 for unlimited
	Impl    FunctionImpl
}

// FunctionImpl is the implementation of a function.
type FunctionImpl func(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error)

// Lambda represents a lambda function.
type Lambda struct {
	Params []string
	Body   *types.ASTNode
	Ctx    *EvalContext // Closure context
}

// OrderedObject preserves insertion order for JSON stringification.
type OrderedObject struct {
	Keys   []string
	Values map[string]interface{}
}

// Get retrieves a value by key.
func (o *OrderedObject) Get(key string) (interface{}, bool) {
	value, ok := o.Values[key]
	return value, ok
}

// MarshalJSON preserves key order during marshaling.
func (o *OrderedObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range o.Keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyBytes, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buf.Write(keyBytes)
		buf.WriteByte(':')
		valueBytes, err := json.Marshal(o.Values[key])
		if err != nil {
			return nil, err
		}
		buf.Write(valueBytes)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

var (
	builtinFunctions     map[string]*FunctionDef
	builtinFunctionsOnce sync.Once
)

// initBuiltinFunctions initializes the built-in function registry.
func initBuiltinFunctions() {
	builtinFunctionsOnce.Do(func() {
		builtinFunctions = map[string]*FunctionDef{
			// Aggregation functions
			"sum":     {Name: "sum", MinArgs: 1, MaxArgs: 1, Impl: fnSum},
			"count":   {Name: "count", MinArgs: 1, MaxArgs: 1, Impl: fnCount},
			"average": {Name: "average", MinArgs: 1, MaxArgs: 1, Impl: fnAverage},
			"min":     {Name: "min", MinArgs: 1, MaxArgs: 1, Impl: fnMin},
			"max":     {Name: "max", MinArgs: 1, MaxArgs: 1, Impl: fnMax},

			// Array functions
			"map":     {Name: "map", MinArgs: 2, MaxArgs: 2, Impl: fnMap},
			"filter":  {Name: "filter", MinArgs: 2, MaxArgs: 2, Impl: fnFilter},
			"reduce":  {Name: "reduce", MinArgs: 2, MaxArgs: 3, Impl: fnReduce},
			"sort":    {Name: "sort", MinArgs: 1, MaxArgs: 2, Impl: fnSort},
			"append":  {Name: "append", MinArgs: 2, MaxArgs: 2, Impl: fnAppend},
			"reverse": {Name: "reverse", MinArgs: 1, MaxArgs: 1, Impl: fnReverse},

			// String functions
			"string":    {Name: "string", MinArgs: 0, MaxArgs: 2, Impl: fnString},
			"length":    {Name: "length", MinArgs: 1, MaxArgs: 1, Impl: fnLength},
			"substring": {Name: "substring", MinArgs: 2, MaxArgs: 3, Impl: fnSubstring},
			"uppercase": {Name: "uppercase", MinArgs: 1, MaxArgs: 1, Impl: fnUppercase},
			"lowercase": {Name: "lowercase", MinArgs: 1, MaxArgs: 1, Impl: fnLowercase},
			"trim":      {Name: "trim", MinArgs: 1, MaxArgs: 1, Impl: fnTrim},
			"contains":  {Name: "contains", MinArgs: 2, MaxArgs: 2, Impl: fnContains},
			"split":     {Name: "split", MinArgs: 2, MaxArgs: 3, Impl: fnSplit},
			"join":      {Name: "join", MinArgs: 1, MaxArgs: 2, Impl: fnJoin},

			// Type functions
			"type":    {Name: "type", MinArgs: 1, MaxArgs: 1, Impl: fnType},
			"exists":  {Name: "exists", MinArgs: 1, MaxArgs: 1, Impl: fnExists},
			"number":  {Name: "number", MinArgs: 1, MaxArgs: 1, Impl: fnNumber},
			"boolean": {Name: "boolean", MinArgs: 1, MaxArgs: 1, Impl: fnBoolean},
			"not":     {Name: "not", MinArgs: 1, MaxArgs: 1, Impl: fnNot},

			// Math functions
			"abs":   {Name: "abs", MinArgs: 1, MaxArgs: 1, Impl: fnAbs},
			"floor": {Name: "floor", MinArgs: 1, MaxArgs: 1, Impl: fnFloor},
			"ceil":  {Name: "ceil", MinArgs: 1, MaxArgs: 1, Impl: fnCeil},
			"round": {Name: "round", MinArgs: 1, MaxArgs: 2, Impl: fnRound},
			"sqrt":  {Name: "sqrt", MinArgs: 1, MaxArgs: 1, Impl: fnSqrt},
			"power": {Name: "power", MinArgs: 2, MaxArgs: 2, Impl: fnPower},
		}
	})
}

// GetFunction retrieves a built-in function by name.
func GetFunction(name string) (*FunctionDef, bool) {
	initBuiltinFunctions()
	fn, ok := builtinFunctions[name]
	return fn, ok
}

// --- Aggregation Functions ---

func fnSum(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
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

func fnMap(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	lambda, ok := args[1].(*Lambda)
	if !ok {
		return nil, fmt.Errorf("second argument to $map must be a function")
	}

	result := make([]interface{}, 0, len(arr))
	for _, item := range arr {
		value, err := e.callLambda(ctx, lambda, []interface{}{item})
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}

	return result, nil
}

func fnFilter(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	lambda, ok := args[1].(*Lambda)
	if !ok {
		return nil, fmt.Errorf("second argument to $filter must be a function")
	}

	result := make([]interface{}, 0)
	for _, item := range arr {
		value, err := e.callLambda(ctx, lambda, []interface{}{item})
		if err != nil {
			return nil, err
		}
		if e.isTruthy(value) {
			result = append(result, item)
		}
	}

	return result, nil
}

func fnReduce(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	lambda, ok := args[1].(*Lambda)
	if !ok {
		return nil, fmt.Errorf("second argument to $reduce must be a function")
	}

	if len(arr) == 0 {
		if len(args) == 3 {
			return args[2], nil
		}
		return nil, nil
	}

	var accumulator interface{}
	startIdx := 0

	if len(args) == 3 {
		accumulator = args[2]
	} else {
		accumulator = arr[0]
		startIdx = 1
	}

	for i := startIdx; i < len(arr); i++ {
		value, err := e.callLambda(ctx, lambda, []interface{}{accumulator, arr[i]})
		if err != nil {
			return nil, err
		}
		accumulator = value
	}

	return accumulator, nil
}

func fnSort(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	// Make a copy to avoid modifying the original
	result := make([]interface{}, len(arr))
	copy(result, arr)

	if len(args) == 1 {
		// Default sort: numeric or string
		sort.SliceStable(result, func(i, j int) bool {
			// Try numeric comparison first
			ni, oki := result[i].(float64)
			nj, okj := result[j].(float64)
			if oki && okj {
				return ni < nj
			}

			// Fall back to string comparison
			si := e.toString(result[i])
			sj := e.toString(result[j])
			return si < sj
		})
	} else {
		// Custom sort with lambda
		lambda, ok := args[1].(*Lambda)
		if !ok {
			return nil, fmt.Errorf("second argument to $sort must be a function")
		}

		sort.SliceStable(result, func(i, j int) bool {
			value, err := e.callLambda(ctx, lambda, []interface{}{result[i], result[j]})
			if err != nil {
				return false
			}
			// Truthy means i < j
			return e.isTruthy(value)
		})
	}

	return result, nil
}

func fnAppend(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	arr1, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	arr2, err := e.toArray(args[1])
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, 0, len(arr1)+len(arr2))
	result = append(result, arr1...)
	result = append(result, arr2...)

	return result, nil
}

func fnReverse(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(arr))
	for i := 0; i < len(arr); i++ {
		result[i] = arr[len(arr)-1-i]
	}

	return result, nil
}

// --- String Functions ---

func fnString(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	var value interface{}
	if len(args) == 0 {
		value = evalCtx.Data()
	} else {
		value = args[0]
	}

	// undefined returns undefined
	if value == nil {
		return nil, nil
	}
	if _, ok := value.(types.Null); ok {
		return "null", nil
	}

	// Check for prettify parameter (second arg)
	prettify := false
	if len(args) > 1 {
		if p, ok := args[1].(bool); ok {
			prettify = p
		} else {
			// Second argument provided but not boolean - error
			return nil, types.NewError(types.ErrArgumentCountMismatch,
				"function $string: second argument must be boolean", -1)
		}
	}

	// For simple types, use toString
	switch v := value.(type) {
	case string:
		return v, nil
	case float64:
		// Check for non-finite values - direct infinity is D3001 (arithmetic error)
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return nil, types.NewError(types.ErrSerializeNonFinite,
				"value cannot be represented as a JSON number", -1)
		}
		return e.toString(value), nil
	case int, bool:
		return e.toString(value), nil
	case *Lambda, *FunctionDef:
		return "", nil
	case map[string]interface{}, []interface{}, *OrderedObject:
		processed, err := preprocessForStringify(e, value)
		if err != nil {
			return nil, err
		}

		// Check for non-finite values in processed data - D1001 (JSON serialization error)
		if containsNonFinite(processed) {
			return nil, types.NewError(types.ErrNumberTooLarge,
				"value cannot be represented as a JSON number", -1)
		}

		// For objects and arrays, use JSON marshaling
		var bytes []byte
		if prettify {
			bytes, err = json.MarshalIndent(processed, "", "  ")
		} else {
			bytes, err = json.Marshal(processed)
		}
		if err != nil {
			return nil, err
		}
		return string(bytes), nil
	default:
		// Fallback to toString
		return e.toString(value), nil
	}
}

// containsNonFinite recursively checks if a value contains non-finite numbers (Inf, NaN)
func containsNonFinite(value interface{}) bool {
	switch v := value.(type) {
	case float64:
		return math.IsInf(v, 0) || math.IsNaN(v)
	case map[string]interface{}:
		for _, item := range v {
			if containsNonFinite(item) {
				return true
			}
		}
		return false
	case []interface{}:
		for _, item := range v {
			if containsNonFinite(item) {
				return true
			}
		}
		return false
	case *OrderedObject:
		for _, item := range v.Values {
			if containsNonFinite(item) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func preprocessForStringify(e *Evaluator, value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case types.Null:
		return nil, nil
	case float64:
		return e.roundNumberForJSON(v), nil
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, item := range v {
			if isFunctionValue(item) {
				result[key] = ""
				continue
			}
			processed, err := preprocessForStringify(e, item)
			if err != nil {
				return nil, err
			}
			result[key] = processed
		}
		return result, nil
	case *OrderedObject:
		result := &OrderedObject{
			Keys:   make([]string, 0, len(v.Keys)),
			Values: make(map[string]interface{}, len(v.Values)),
		}
		for _, key := range v.Keys {
			item, _ := v.Values[key]
			result.Keys = append(result.Keys, key)
			if isFunctionValue(item) {
				result.Values[key] = ""
				continue
			}
			processed, err := preprocessForStringify(e, item)
			if err != nil {
				return nil, err
			}
			result.Values[key] = processed
		}
		return result, nil
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			if isFunctionValue(item) {
				result[i] = ""
				continue
			}
			processed, err := preprocessForStringify(e, item)
			if err != nil {
				return nil, err
			}
			result[i] = processed
		}
		return result, nil
	default:
		if isFunctionValue(value) {
			return "", nil
		}
		return value, nil
	}
}

func isFunctionValue(value interface{}) bool {
	switch value.(type) {
	case *Lambda, *FunctionDef:
		return true
	default:
		return false
	}
}

func fnLength(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if str, ok := args[0].(string); ok {
		return float64(len(str)), nil
	}

	if arr, err := e.toArray(args[0]); err == nil {
		return float64(len(arr)), nil
	}

	return 0.0, nil
}

func fnSubstring(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	str := e.toString(args[0])

	start, err := e.toNumber(args[1])
	if err != nil {
		return nil, err
	}

	startIdx := int(start)
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > len(str) {
		return "", nil
	}

	if len(args) == 2 {
		return str[startIdx:], nil
	}

	length, err := e.toNumber(args[2])
	if err != nil {
		return nil, err
	}

	endIdx := startIdx + int(length)
	if endIdx > len(str) {
		endIdx = len(str)
	}

	return str[startIdx:endIdx], nil
}

func fnUppercase(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	str := e.toString(args[0])
	return strings.ToUpper(str), nil
}

func fnLowercase(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	str := e.toString(args[0])
	return strings.ToLower(str), nil
}

func fnTrim(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	str := e.toString(args[0])
	return strings.TrimSpace(str), nil
}

func fnContains(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	str := e.toString(args[0])
	pattern := e.toString(args[1])
	return strings.Contains(str, pattern), nil
}

func fnSplit(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	str := e.toString(args[0])
	separator := e.toString(args[1])

	var parts []string
	if len(args) == 3 {
		limit, err := e.toNumber(args[2])
		if err != nil {
			return nil, err
		}
		parts = strings.SplitN(str, separator, int(limit))
	} else {
		parts = strings.Split(str, separator)
	}

	result := make([]interface{}, len(parts))
	for i, p := range parts {
		result[i] = p
	}

	return result, nil
}

func fnJoin(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	separator := ""
	if len(args) == 2 {
		separator = e.toString(args[1])
	}

	strs := make([]string, len(arr))
	for i, v := range arr {
		strs[i] = e.toString(v)
	}

	return strings.Join(strs, separator), nil
}

// --- Type Functions ---

func fnType(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	value := args[0]

	if value == nil {
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
	return e.isTruthy(args[0]), nil
}

func fnNot(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// Special case: not(undefined) → undefined (per JSONata spec)
	if args[0] == nil {
		return nil, nil
	}
	return !e.isTruthy(args[0]), nil
}

// --- Math Functions ---

func fnAbs(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}
	return math.Abs(num), nil
}

func fnFloor(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}
	return math.Floor(num), nil
}

func fnCeil(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}
	return math.Ceil(num), nil
}

func fnRound(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}

	if len(args) == 1 {
		return math.Round(num), nil
	}

	precision, err := e.toNumber(args[1])
	if err != nil {
		return nil, err
	}

	multiplier := math.Pow(10, precision)
	return math.Round(num*multiplier) / multiplier, nil
}

func fnSqrt(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}
	return math.Sqrt(num), nil
}

func fnPower(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	base, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}

	exponent, err := e.toNumber(args[1])
	if err != nil {
		return nil, err
	}

	return math.Pow(base, exponent), nil
}
