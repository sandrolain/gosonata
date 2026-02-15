package evaluator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

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
			"map":      {Name: "map", MinArgs: 2, MaxArgs: 2, Impl: fnMap},
			"filter":   {Name: "filter", MinArgs: 2, MaxArgs: 2, Impl: fnFilter},
			"reduce":   {Name: "reduce", MinArgs: 2, MaxArgs: 3, Impl: fnReduce},
			"sort":     {Name: "sort", MinArgs: 1, MaxArgs: 2, Impl: fnSort},
			"append":   {Name: "append", MinArgs: 2, MaxArgs: 2, Impl: fnAppend},
			"reverse":  {Name: "reverse", MinArgs: 1, MaxArgs: 1, Impl: fnReverse},
			"distinct": {Name: "distinct", MinArgs: 1, MaxArgs: 1, Impl: fnDistinct},
			"shuffle":  {Name: "shuffle", MinArgs: 1, MaxArgs: 1, Impl: fnShuffle},
			"zip":      {Name: "zip", MinArgs: 1, MaxArgs: -1, Impl: fnZip},

			// String functions
			"string":          {Name: "string", MinArgs: 0, MaxArgs: 2, Impl: fnString},
			"length":          {Name: "length", MinArgs: 1, MaxArgs: 1, Impl: fnLength},
			"substring":       {Name: "substring", MinArgs: 2, MaxArgs: 3, Impl: fnSubstring},
			"uppercase":       {Name: "uppercase", MinArgs: 1, MaxArgs: 1, Impl: fnUppercase},
			"lowercase":       {Name: "lowercase", MinArgs: 1, MaxArgs: 1, Impl: fnLowercase},
			"trim":            {Name: "trim", MinArgs: 1, MaxArgs: 1, Impl: fnTrim},
			"contains":        {Name: "contains", MinArgs: 2, MaxArgs: 2, Impl: fnContains},
			"split":           {Name: "split", MinArgs: 2, MaxArgs: 3, Impl: fnSplit},
			"join":            {Name: "join", MinArgs: 1, MaxArgs: 2, Impl: fnJoin},
			"pad":             {Name: "pad", MinArgs: 2, MaxArgs: 3, Impl: fnPad},
			"substringBefore": {Name: "substringBefore", MinArgs: 2, MaxArgs: 2, Impl: fnSubstringBefore},
			"substringAfter":  {Name: "substringAfter", MinArgs: 2, MaxArgs: 2, Impl: fnSubstringAfter},

			// Type functions
			"type":    {Name: "type", MinArgs: 1, MaxArgs: 1, Impl: fnType},
			"exists":  {Name: "exists", MinArgs: 1, MaxArgs: 1, Impl: fnExists},
			"number":  {Name: "number", MinArgs: 1, MaxArgs: 1, Impl: fnNumber},
			"boolean": {Name: "boolean", MinArgs: 1, MaxArgs: 1, Impl: fnBoolean},
			"not":     {Name: "not", MinArgs: 1, MaxArgs: 1, Impl: fnNot},

			// Math functions
			"abs":    {Name: "abs", MinArgs: 1, MaxArgs: 1, Impl: fnAbs},
			"floor":  {Name: "floor", MinArgs: 1, MaxArgs: 1, Impl: fnFloor},
			"ceil":   {Name: "ceil", MinArgs: 1, MaxArgs: 1, Impl: fnCeil},
			"round":  {Name: "round", MinArgs: 1, MaxArgs: 2, Impl: fnRound},
			"sqrt":   {Name: "sqrt", MinArgs: 1, MaxArgs: 1, Impl: fnSqrt},
			"power":  {Name: "power", MinArgs: 2, MaxArgs: 2, Impl: fnPower},
			"random": {Name: "random", MinArgs: 0, MaxArgs: 0, Impl: fnRandom},

			// Object functions
			"each":   {Name: "each", MinArgs: 2, MaxArgs: 2, Impl: fnEach},
			"keys":   {Name: "keys", MinArgs: 1, MaxArgs: 1, Impl: fnKeys},
			"lookup": {Name: "lookup", MinArgs: 2, MaxArgs: 2, Impl: fnLookup},
			"merge":  {Name: "merge", MinArgs: 1, MaxArgs: 1, Impl: fnMerge},
			"spread": {Name: "spread", MinArgs: 1, MaxArgs: 1, Impl: fnSpread},
			"error":  {Name: "error", MinArgs: 0, MaxArgs: 1, Impl: fnError},
			"assert": {Name: "assert", MinArgs: 1, MaxArgs: 2, Impl: fnAssert},

			// Regex functions
			"match":   {Name: "match", MinArgs: 2, MaxArgs: 3, Impl: fnMatch},
			"replace": {Name: "replace", MinArgs: 3, MaxArgs: 4, Impl: fnReplace},

			// Date/Time functions
			"now":        {Name: "now", MinArgs: 0, MaxArgs: 2, Impl: fnNow},
			"millis":     {Name: "millis", MinArgs: 0, MaxArgs: 0, Impl: fnMillis},
			"fromMillis": {Name: "fromMillis", MinArgs: 1, MaxArgs: 3, Impl: fnFromMillis},
			"toMillis":   {Name: "toMillis", MinArgs: 1, MaxArgs: 2, Impl: fnToMillis},

			// Encoding functions
			"base64encode":       {Name: "base64encode", MinArgs: 1, MaxArgs: 1, Impl: fnBase64Encode},
			"base64decode":       {Name: "base64decode", MinArgs: 1, MaxArgs: 1, Impl: fnBase64Decode},
			"encodeUrl":          {Name: "encodeUrl", MinArgs: 1, MaxArgs: 1, Impl: fnEncodeUrl},
			"decodeUrl":          {Name: "decodeUrl", MinArgs: 1, MaxArgs: 1, Impl: fnDecodeUrl},
			"encodeUrlComponent": {Name: "encodeUrlComponent", MinArgs: 1, MaxArgs: 1, Impl: fnEncodeUrlComponent},
			"decodeUrlComponent": {Name: "decodeUrlComponent", MinArgs: 1, MaxArgs: 1, Impl: fnDecodeUrlComponent},

			// Number formatting functions
			"formatNumber":  {Name: "formatNumber", MinArgs: 1, MaxArgs: 3, Impl: fnFormatNumber},
			"formatBase":    {Name: "formatBase", MinArgs: 1, MaxArgs: 2, Impl: fnFormatBase},
			"formatInteger": {Name: "formatInteger", MinArgs: 1, MaxArgs: 2, Impl: fnFormatInteger},
			"parseInteger":  {Name: "parseInteger", MinArgs: 1, MaxArgs: 2, Impl: fnParseInteger},
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

	lengthInt := int(length)
	if lengthInt <= 0 {
		return "", nil
	}

	endIdx := startIdx + lengthInt
	if endIdx > len(str) {
		endIdx = len(str)
	}

	return str[startIdx:endIdx], nil
}

func fnUppercase(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
	str := e.toString(args[0])
	return strings.ToUpper(str), nil
}

func fnLowercase(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
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

	// Check for JSONata null (types.Null) first
	if _, ok := value.(types.Null); ok {
		return "null", nil
	}

	// undefined (nil) also returns "null" per JSONata spec
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
func fnEach(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	obj := args[0]
	if obj == nil {
		return []interface{}{}, nil
	}

	lambda, ok := args[1].(*Lambda)
	if !ok {
		return nil, fmt.Errorf("second argument to $each must be a function")
	}

	// Validate lambda has exactly 2 parameters (value, key)
	if len(lambda.Params) != 2 {
		return nil, fmt.Errorf("$each requires a function with 2 parameters (value, key), got %d", len(lambda.Params))
	}

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
		// Call lambda with (value, key)
		lambdaResult, err := e.callLambda(ctx, lambda, []interface{}{value, key})
		if err != nil {
			return nil, err
		}
		result = append(result, lambdaResult)
	}

	return result, nil
}

// --- Math Functions (Extended) ---

// fnRandom returns a pseudo-random number between 0 (inclusive) and 1 (exclusive).
func fnRandom(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	return rand.Float64(), nil
}

// --- Object Functions ---

// fnKeys returns an array of keys from an object or array of objects.
// For arrays, returns a de-duplicated list of all keys from all items.
func fnKeys(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return []interface{}{}, nil
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
func fnError(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	message := "$error() function evaluated"
	if len(args) > 0 && args[0] != nil {
		message = fmt.Sprint(args[0])
	}
	return nil, fmt.Errorf("D3137: %s", message)
}

// fnAssert asserts a condition, throws error if false.
// Signature: $assert(condition [, message])
// The condition must be a boolean; null and numbers return T0410 error
func fnAssert(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("T0410: $assert() requires at least 1 argument")
	}

	// Validate that first argument is a boolean
	// null and numbers are not valid conditions
	if args[0] != nil {
		if _, ok := args[0].(bool); !ok {
			// Non-boolean values are not valid conditions
			return nil, fmt.Errorf("T0410: $assert() requires condition to be boolean")
		}
	} else {
		// null is not a valid condition
		return nil, fmt.Errorf("T0410: $assert() requires condition to be boolean")
	}

	// At this point, args[0] is a boolean
	condition := args[0].(bool)

	// Extract message
	message := "$assert() statement failed"
	if len(args) > 1 && args[1] != nil {
		message = fmt.Sprint(args[1])
	}

	if !condition {
		return nil, fmt.Errorf("D3141: %s", message)
	}
	return nil, nil
}

// --- Regex Functions ---

// fnMatch finds regex matches and returns array of match objects.
// Signature: $match(str, pattern [, limit])
func fnMatch(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	str, ok := args[0].(string)
	if !ok {
		str = fmt.Sprint(args[0])
	}

	// Get pattern (string or regex)
	var regexPattern *regexp.Regexp
	var err error

	switch pattern := args[1].(type) {
	case string:
		regexPattern, err = regexp.Compile(regexp.QuoteMeta(pattern))
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
	case *regexp.Regexp:
		regexPattern = pattern
	default:
		return nil, fmt.Errorf("pattern must be string or regex")
	}

	// Get limit if provided
	limit := -1
	if len(args) > 2 && args[2] != nil {
		limitNum, err := e.toNumber(args[2])
		if err != nil {
			return nil, err
		}
		limit = int(limitNum)
	}

	// Find all matches
	matches := regexPattern.FindAllStringSubmatchIndex(str, limit)
	if matches == nil {
		return []interface{}{}, nil
	}

	result := make([]interface{}, len(matches))
	for i, match := range matches {
		// match[0:2] is the full match start:end
		// match[2:] are capture groups
		matchStr := str[match[0]:match[1]]
		groups := make([]interface{}, 0)

		// Add capture groups
		for j := 1; j < len(match)/2; j++ {
			start := match[2*j]
			end := match[2*j+1]
			if start >= 0 && end >= 0 {
				groups = append(groups, str[start:end])
			} else {
				groups = append(groups, nil)
			}
		}

		matchObj := &OrderedObject{
			Keys: []string{"match", "index", "groups"},
			Values: map[string]interface{}{
				"match":  matchStr,
				"index":  float64(match[0]),
				"groups": groups,
			},
		}
		result[i] = matchObj
	}

	return result, nil
}

// fnReplace finds and replaces using regex or string pattern.
// Signature: $replace(str, pattern, replacement [, limit])
func fnReplace(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// undefined inputs return undefined
	if args[0] == nil {
		return nil, nil
	}

	str, ok := args[0].(string)
	if !ok {
		str = fmt.Sprint(args[0])
	}

	replacement := fmt.Sprint(args[2])

	// Get limit if provided
	limit := -1 // -1 means unlimited
	if len(args) > 3 && args[3] != nil {
		limitNum, err := e.toNumber(args[3])
		if err != nil {
			return nil, err
		}
		limit = int(limitNum)
		// Validate limit is not negative (except -1 which means unlimited)
		if limit < 0 && limit != -1 {
			return nil, fmt.Errorf("D3011: limit must be non-negative")
		}
	}

	// Get pattern (string or regex)
	var result string
	switch pattern := args[1].(type) {
	case string:
		// Validate pattern is not empty
		if pattern == "" {
			return nil, fmt.Errorf("D3010: pattern cannot be empty")
		}

		// Simple string replacement
		if limit < 0 {
			result = strings.ReplaceAll(str, pattern, replacement)
		} else {
			result = strings.Replace(str, pattern, replacement, limit)
		}

	case *regexp.Regexp:
		// Validate pattern is not empty
		if pattern.String() == "" {
			return nil, fmt.Errorf("D3010: pattern cannot be empty")
		}

		// Regex replacement
		if limit < 0 {
			result = pattern.ReplaceAllString(str, replacement)
		} else {
			// Replace only first 'limit' occurrences
			matches := pattern.FindAllStringIndex(str, limit)
			if len(matches) == 0 {
				result = str
			} else {
				var buf bytes.Buffer
				lastEnd := 0
				for _, match := range matches {
					buf.WriteString(str[lastEnd:match[0]])
					buf.WriteString(replacement)
					lastEnd = match[1]
				}
				buf.WriteString(str[lastEnd:])
				result = buf.String()
			}
		}

	default:
		return nil, fmt.Errorf("pattern must be string or regex")
	}

	return result, nil
}

// --- Date/Time Functions ---

var nowTime time.Time
var nowCalculated bool

// fnNow returns current timestamp in ISO 8601 format.
// Signature: $now([picture [, timezone]])
func fnNow(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// Cache the current time for all evaluations in this context
	if !nowCalculated {
		nowTime = time.Now()
		nowCalculated = true
	}

	// Simple ISO 8601 format if no picture provided
	if len(args) == 0 {
		return nowTime.UTC().Format(time.RFC3339Nano), nil
	}

	// Note: Full XPath datetime formatting is complex and not implemented
	// Return simple ISO format for now
	return nowTime.UTC().Format(time.RFC3339Nano), nil
}

// fnMillis returns milliseconds since Unix epoch.
func fnMillis(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// Use same time as $now for consistency
	if !nowCalculated {
		nowTime = time.Now()
		nowCalculated = true
	}
	return float64(nowTime.UnixMilli()), nil
}

// fnFromMillis converts milliseconds since epoch to ISO 8601 string.
// Signature: $fromMillis(number [, picture [, timezone]])
func fnFromMillis(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// undefined inputs return undefined
	if args[0] == nil {
		return nil, nil
	}

	millis, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}

	timestamp := time.Unix(0, int64(millis)*1000000).UTC()

	// Simple ISO 8601 format if no picture provided
	if len(args) < 2 {
		return timestamp.Format(time.RFC3339Nano), nil
	}

	// Note: Full XPath datetime formatting is complex and not implemented
	// Return simple ISO format for now
	return timestamp.Format(time.RFC3339Nano), nil
}

// fnToMillis converts ISO 8601 timestamp to milliseconds since epoch.
// Signature: $toMillis(timestamp [, picture])
func fnToMillis(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// undefined inputs return undefined
	if args[0] == nil {
		return nil, nil
	}

	timestamp, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("D3110: timestamp must be a string, got %T", args[0])
	}

	// If picture format is provided, use custom parsing
	if len(args) == 2 && args[1] != nil {
		picture, ok := args[1].(string)
		if !ok {
			return nil, fmt.Errorf("picture format must be a string")
		}
		return parseTimestampWithPicture(timestamp, picture)
	}

	// Normalize timezone offset: convert +0000 to +00:00
	normalized := normalizeTimezoneOffset(timestamp)

	// Try parsing ISO 8601 formats
	layouts := []string{
		time.RFC3339Nano,                     // 2006-01-02T15:04:05.999999999Z07:00
		time.RFC3339,                         // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04:05.999999999Z0700", // with numeric timezone
		"2006-01-02T15:04:05Z0700",
		"2006-01-02T15:04:05.999999999", // without timezone
		"2006-01-02T15:04:05",
		"2006-01-02", // date only
		"2006-01",    // year-month only
		"2006",       // year only
	}

	var t time.Time
	var err error
	for _, layout := range layouts {
		t, err = time.Parse(layout, normalized)
		if err == nil {
			return float64(t.UnixMilli()), nil
		}
	}

	return nil, fmt.Errorf("D3110: cannot parse timestamp: %s", timestamp)
}

// normalizeTimezoneOffset converts timezone offsets like +0000 to +00:00
func normalizeTimezoneOffset(timestamp string) string {
	// Match timezone offset at the end: +0000 or -0000
	re := regexp.MustCompile(`([+-])(\d{2})(\d{2})$`)
	if re.MatchString(timestamp) {
		return re.ReplaceAllString(timestamp, `$1$2:$3`)
	}
	return timestamp
}

// parseTimestampWithPicture parses a timestamp using a picture format string.
// Picture format uses markers like [Y0001] for year, [M01] for month, etc.
// This is a simplified implementation supporting only the patterns in the test suite.
func parseTimestampWithPicture(timestamp, picture string) (interface{}, error) {
	// Parse picture format to extract component patterns
	// Supported patterns:
	// [Y0001], [Y0000], [Y,*-4] - year (4 digits)
	// [M01], [M00] - month (2 digits)
	// [D01], [D00] - day (2 digits)
	// [H00] - hour (2 digits)
	// [m00] - minute (2 digits)
	// [s00] - second (2 digits)

	// Build regex pattern and component extractors
	type component struct {
		name    string
		pattern string
		digits  int
	}

	var components []component
	pattern := picture

	// Replace picture markers with regex groups
	replacements := []struct {
		markers []string
		comp    component
	}{
		{[]string{"[Y0001]", "[Y0000]", "[Y,*-4]"}, component{"year", `(\d{4})`, 4}},
		{[]string{"[M01]", "[M00]"}, component{"month", `(\d{2})`, 2}},
		{[]string{"[D01]", "[D00]"}, component{"day", `(\d{2})`, 2}},
		{[]string{"[H00]"}, component{"hour", `(\d{2})`, 2}},
		{[]string{"[m00]"}, component{"minute", `(\d{2})`, 2}},
		{[]string{"[s00]"}, component{"second", `(\d{2})`, 2}},
	}

	for _, repl := range replacements {
		for _, marker := range repl.markers {
			if strings.Contains(pattern, marker) {
				components = append(components, repl.comp)
				pattern = strings.Replace(pattern, marker, repl.comp.pattern, 1)
				break
			}
		}
	}

	// Compile and match
	re, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		return nil, fmt.Errorf("invalid picture format: %s", picture)
	}

	matches := re.FindStringSubmatch(timestamp)
	if matches == nil {
		return nil, fmt.Errorf("D3110: cannot parse timestamp with picture format: %s", timestamp)
	}

	// Extract components
	values := make(map[string]int)
	for i, comp := range components {
		val, _ := strconv.Atoi(matches[i+1])
		values[comp.name] = val
	}

	// Default missing components
	year := values["year"]
	if year == 0 {
		year = time.Now().UTC().Year()
	}
	month := values["month"]
	if month == 0 {
		month = 1
	}
	day := values["day"]
	if day == 0 {
		day = 1
	}
	hour := values["hour"]
	minute := values["minute"]
	second := values["second"]

	// Create time and convert to milliseconds
	t := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
	return float64(t.UnixMilli()), nil
}

// --- Encoding Functions (Fase 5.3) ---

// fnBase64Encode encodes a string to base64.
// Signature: $base64encode(string)
func fnBase64Encode(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	encoded := base64.StdEncoding.EncodeToString([]byte(str))
	return encoded, nil
}

// fnBase64Decode decodes a base64 string.
// Signature: $base64decode(string)
func fnBase64Decode(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	decoded, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, fmt.Errorf("D3137: invalid base64 string: %w", err)
	}
	return string(decoded), nil
}

// fnEncodeUrl encodes a URL string.
// Signature: $encodeUrl(string)
// Encodes the full URL (path and query string).
func fnEncodeUrl(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	// Parse and re-encode to handle special characters
	u, err := url.Parse(str)
	if err != nil {
		// If parsing fails, do basic escaping
		return url.PathEscape(str), nil
	}
	return u.String(), nil
}

// fnDecodeUrl decodes a URL string.
// Signature: $decodeUrl(string)
func fnDecodeUrl(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	decoded, err := url.PathUnescape(str)
	if err != nil {
		return nil, fmt.Errorf("D3137: invalid URL encoding: %w", err)
	}
	return decoded, nil
}

// fnEncodeUrlComponent encodes a URL component (query parameter value).
// Signature: $encodeUrlComponent(string)
func fnEncodeUrlComponent(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	return url.QueryEscape(str), nil
}

// fnDecodeUrlComponent decodes a URL component.
// Signature: $decodeUrlComponent(string)
func fnDecodeUrlComponent(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	decoded, err := url.QueryUnescape(str)
	if err != nil {
		return nil, fmt.Errorf("D3137: invalid URL component encoding: %w", err)
	}
	return decoded, nil
}

// --- Number Formatting Functions (Fase 5.3) ---

// fnFormatNumber formats a number with optional picture string and decimal format.
// Signature: $formatNumber(number [, picture [, options]])
// Simplified implementation without full XPath picture string support.
func fnFormatNumber(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}

	// Check for non-finite values
	if math.IsInf(num, 0) || math.IsNaN(num) {
		return nil, fmt.Errorf("D3061: cannot format non-finite number")
	}

	// Default formatting
	if len(args) == 1 {
		return e.formatNumberForString(num), nil
	}

	// Picture string formatting (simplified)
	picture := e.toString(args[1])

	// Parse options if provided
	decimalSep := "."
	groupingSep := ","
	if len(args) > 2 && args[2] != nil {
		if opts, ok := args[2].(map[string]interface{}); ok {
			if ds, ok := opts["decimal-separator"].(string); ok {
				decimalSep = ds
			}
			if gs, ok := opts["grouping-separator"].(string); ok {
				groupingSep = gs
			}
		}
	}

	// Simple picture string parsing
	// Format: "#,###.##" style patterns
	formatted := formatNumberWithPicture(num, picture, decimalSep, groupingSep)
	return formatted, nil
}

// formatNumberWithPicture formats a number using a simple picture string.
func formatNumberWithPicture(num float64, picture, decimalSep, groupingSep string) string {
	// Extract decimal places from picture
	decimalPlaces := 0
	if idx := strings.IndexAny(picture, "."+decimalSep); idx >= 0 {
		decimalPlaces = len(picture) - idx - 1
	}

	// Format with specified decimal places
	formatStr := fmt.Sprintf("%%.%df", decimalPlaces)
	formatted := fmt.Sprintf(formatStr, num)

	// Replace decimal separator if needed
	if decimalSep != "." {
		formatted = strings.Replace(formatted, ".", decimalSep, 1)
	}

	// Add grouping separator for thousands
	if groupingSep != "" && (strings.Contains(picture, ",") || strings.Contains(picture, groupingSep)) {
		parts := strings.Split(formatted, decimalSep)
		intPart := parts[0]

		// Handle negative numbers
		negative := false
		if strings.HasPrefix(intPart, "-") {
			negative = true
			intPart = intPart[1:]
		}

		// Add grouping from right to left
		if len(intPart) > 3 {
			var result []rune
			for i, c := range intPart {
				if i > 0 && (len(intPart)-i)%3 == 0 {
					for _, ch := range groupingSep {
						result = append(result, ch)
					}
				}
				result = append(result, c)
			}
			intPart = string(result)
		}

		if negative {
			intPart = "-" + intPart
		}

		if len(parts) > 1 {
			formatted = intPart + decimalSep + parts[1]
		} else {
			formatted = intPart
		}
	}

	return formatted
}

// fnFormatBase formats a number in a different base (2-36).
// Signature: $formatBase(number [, radix])
func fnFormatBase(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}

	// Check for non-finite values
	if math.IsInf(num, 0) || math.IsNaN(num) {
		return nil, fmt.Errorf("D3061: cannot format non-finite number")
	}

	// Default radix is 10
	radix := 10
	if len(args) > 1 && args[1] != nil {
		radixNum, err := e.toNumber(args[1])
		if err != nil {
			return nil, err
		}
		radix = int(radixNum)
		if radix < 2 || radix > 36 {
			return nil, fmt.Errorf("D3100: radix must be between 2 and 36")
		}
	}

	// Convert to integer and format in specified base
	intNum := int64(num)
	return strconv.FormatInt(intNum, radix), nil
}

// fnFormatInteger formats an integer with optional picture string.
// Signature: $formatInteger(number [, picture])
// Simplified implementation supporting basic Roman numerals and ordinals.
func fnFormatInteger(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}

	// Check for non-finite values
	if math.IsInf(num, 0) || math.IsNaN(num) {
		return nil, fmt.Errorf("D3061: cannot format non-finite number")
	}

	intNum := int(num)

	// Default formatting
	if len(args) == 1 {
		return fmt.Sprintf("%d", intNum), nil
	}

	// Picture string formatting
	picture := e.toString(args[1])

	switch picture {
	case "i": // Roman numerals lowercase
		return strings.ToLower(toRomanNumeral(intNum)), nil
	case "I": // Roman numerals uppercase
		return toRomanNumeral(intNum), nil
	case "w": // Words lowercase
		return strings.ToLower(numberToWords(intNum)), nil
	case "W": // Words uppercase
		return numberToWords(intNum), nil
	case "Ww": // Words title case
		return strings.Title(strings.ToLower(numberToWords(intNum))), nil
	default:
		// Default to decimal
		return fmt.Sprintf("%d", intNum), nil
	}
}

// toRomanNumeral converts an integer to Roman numeral representation.
func toRomanNumeral(num int) string {
	if num <= 0 || num >= 4000 {
		return fmt.Sprintf("%d", num) // Outside roman numeral range
	}

	val := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	sym := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}

	var result strings.Builder
	for i := 0; i < len(val); i++ {
		for num >= val[i] {
			result.WriteString(sym[i])
			num -= val[i]
		}
	}

	return result.String()
}

// numberToWords converts an integer to English words (simplified).
func numberToWords(num int) string {
	// Simplified implementation for common numbers
	if num == 0 {
		return "zero"
	}

	if num < 0 {
		return "minus " + numberToWords(-num)
	}

	ones := []string{"", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	teens := []string{"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	tens := []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}

	if num < 10 {
		return ones[num]
	}

	if num < 20 {
		return teens[num-10]
	}

	if num < 100 {
		return tens[num/10] + hyphenIfNeeded(num%10) + ones[num%10]
	}

	if num < 1000 {
		result := ones[num/100] + " hundred"
		if num%100 != 0 {
			result += " " + numberToWords(num%100)
		}
		return result
	}

	if num < 1000000 {
		result := numberToWords(num/1000) + " thousand"
		if num%1000 != 0 {
			result += " " + numberToWords(num%1000)
		}
		return result
	}

	// For larger numbers, just return the decimal representation
	return fmt.Sprintf("%d", num)
}

func hyphenIfNeeded(n int) string {
	if n > 0 {
		return "-"
	}
	return ""
}

// fnParseInteger parses a string to an integer with optional radix.
// Signature: $parseInteger(string [, radix])
func fnParseInteger(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := strings.TrimSpace(e.toString(args[0]))

	// Default radix is 10
	radix := 10
	if len(args) > 1 && args[1] != nil {
		radixNum, err := e.toNumber(args[1])
		if err != nil {
			return nil, err
		}
		radix = int(radixNum)
		if radix < 2 || radix > 36 {
			return nil, fmt.Errorf("D3100: radix must be between 2 and 36")
		}
	}

	// Parse integer
	num, err := strconv.ParseInt(str, radix, 64)
	if err != nil {
		return nil, fmt.Errorf("D3137: cannot parse '%s' as integer", str)
	}

	return float64(num), nil
}

// --- Enhanced Array Functions (Fase 5.2) ---

// fnDistinct removes duplicate values from an array.
// Signature: $distinct(array)
func fnDistinct(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	// Use a map to track seen values
	// Note: This uses string representation for comparison, which may not be perfect
	// for complex objects but works for primitive types
	seen := make(map[string]bool)
	result := make([]interface{}, 0)

	for _, item := range arr {
		// Serialize item to string for comparison
		key := fmt.Sprintf("%v", item)
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// fnShuffle randomly shuffles an array.
// Signature: $shuffle(array)
func fnShuffle(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	// Make a copy to avoid modifying the original
	result := make([]interface{}, len(arr))
	copy(result, arr)

	// Fisher-Yates shuffle
	rand.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})

	return result, nil
}

// fnZip convolves multiple arrays into an array of tuples.
// Signature: $zip(array1, array2, ...)
// Returns array of arrays, where each sub-array contains one element from each input array.
func fnZip(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return []interface{}{}, nil
	}

	// Handle undefined
	if args[0] == nil {
		return nil, nil
	}

	// Convert all args to arrays
	arrays := make([][]interface{}, len(args))
	maxLen := 0

	for i, arg := range args {
		if arg == nil {
			arrays[i] = []interface{}{}
			continue
		}

		arr, err := e.toArray(arg)
		if err != nil {
			return nil, err
		}
		arrays[i] = arr
		if len(arr) > maxLen {
			maxLen = len(arr)
		}
	}

	// Zip arrays together
	result := make([]interface{}, maxLen)
	for i := 0; i < maxLen; i++ {
		tuple := make([]interface{}, len(arrays))
		for j, arr := range arrays {
			if i < len(arr) {
				tuple[j] = arr[i]
			} else {
				tuple[j] = nil
			}
		}
		result[i] = tuple
	}

	return result, nil
}

// --- Enhanced String Functions (Fase 5.2) ---

// fnPad pads a string to a target width.
// Signature: $pad(str, width [, char])
// Pads on the right by default, negative width pads on the left.
func fnPad(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])

	width, err := e.toNumber(args[1])
	if err != nil {
		return nil, err
	}
	targetWidth := int(width)

	// Default pad character is space
	padChar := " "
	if len(args) > 2 && args[2] != nil {
		padChar = e.toString(args[2])
		if len(padChar) == 0 {
			padChar = " "
		}
	}

	// Determine padding direction
	leftPad := targetWidth < 0
	if leftPad {
		targetWidth = -targetWidth
	}

	// Calculate padding needed
	strLen := len(str)
	if strLen >= targetWidth {
		return str, nil
	}

	padCount := targetWidth - strLen
	padding := strings.Repeat(padChar, padCount)

	if leftPad {
		return padding + str, nil
	}
	return str + padding, nil
}

// fnSubstringBefore returns the substring before the first occurrence of a separator.
// Signature: $substringBefore(str, separator)
func fnSubstringBefore(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	separator := e.toString(args[1])

	// If separator is empty, return empty string
	if separator == "" {
		return "", nil
	}

	idx := strings.Index(str, separator)
	if idx < 0 {
		return "", nil
	}

	return str[:idx], nil
}

// fnSubstringAfter returns the substring after the first occurrence of a separator.
// Signature: $substringAfter(str, separator)
func fnSubstringAfter(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	separator := e.toString(args[1])

	// If separator is empty, return the original string
	if separator == "" {
		return str, nil
	}

	idx := strings.Index(str, separator)
	if idx < 0 {
		return "", nil
	}

	return str[idx+len(separator):], nil
}
