package evaluator

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/sandrolain/gosonata/pkg/types"
)

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
	if len(args) > 1 && args[1] != nil {
		if p, ok := args[1].(bool); ok {
			prettify = p
		} else {
			return nil, types.NewError(types.ErrArgumentCountMismatch, "The second argument of the $string function must be Boolean", -1)
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
	// undefined returns undefined
	if args[0] == nil {
		return nil, nil
	}

	// $length accepts only strings
	v, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("T0410: $length() argument must be a string")
	}
	// Count Unicode characters (runes), not bytes
	return float64(utf8.RuneCountInString(v)), nil
}

func fnSubstring(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// undefined returns undefined
	if args[0] == nil {
		return nil, nil
	}

	// Validate first argument is a string
	str, ok := args[0].(string)
	if !ok {
		return nil, types.NewError(types.ErrArgumentCountMismatch, "Argument 1 of function 'substring' must be a string", -1)
	}

	start, err := e.toNumber(args[1])
	if err != nil {
		return nil, err
	}

	// Convert to runes to handle Unicode correctly
	runes := []rune(str)
	startIdx := int(start)
	strLen := len(runes)

	// Handle negative indices (count from end)
	if startIdx < 0 {
		startIdx = strLen + startIdx
		if startIdx < 0 {
			startIdx = 0
		}
	}
	if startIdx > strLen {
		return "", nil
	}

	if len(args) == 2 {
		return string(runes[startIdx:]), nil
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
	if endIdx > strLen {
		endIdx = strLen
	}

	return string(runes[startIdx:endIdx]), nil
}

func fnUppercase(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
	// Validate argument is a string
	str, ok := args[0].(string)
	if !ok {
		return nil, types.NewError(types.ErrArgumentCountMismatch, "Argument 1 of function 'uppercase' must be a string", -1)
	}
	return strings.ToUpper(str), nil
}

func fnLowercase(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
	// Validate argument is a string
	str, ok := args[0].(string)
	if !ok {
		return nil, types.NewError(types.ErrArgumentCountMismatch, "Argument 1 of function 'lowercase' must be a string", -1)
	}
	return strings.ToLower(str), nil
}

func fnTrim(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// Handle no arguments
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	// Trim leading/trailing whitespace
	str = strings.TrimSpace(str)
	// Normalize internal whitespace (collapse multiple spaces to single)
	str = regexp.MustCompile(`\s+`).ReplaceAllString(str, " ")
	return str, nil
}

func fnContains(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// Handle undefined
	if args[0] == nil || args[1] == nil {
		return nil, nil
	}

	// Type checking: first argument must be a string
	str, ok := args[0].(string)
	if !ok {
		return nil, types.NewError("T0410", "Argument 1 of function 'contains' must be a string", -1)
	}

	// Second argument can be a string or regex
	switch pattern := args[1].(type) {
	case string:
		return strings.Contains(str, pattern), nil
	case *regexp.Regexp:
		return pattern.MatchString(str), nil
	default:
		return nil, types.NewError("T0410", "Argument 2 of function 'contains' must be a string or regex", -1)
	}
}

func fnSplit(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// Undefined input → undefined
	if args[0] == nil {
		return nil, nil
	}

	str, ok := args[0].(string)
	if !ok {
		return nil, types.NewError(types.ErrArgumentCountMismatch, "The first argument of the function '$split' must be a string", -1)
	}

	// Check for limit argument type validation
	limit := -1
	if len(args) >= 3 && args[2] != nil {
		// Limit must be a number, not a string
		switch v := args[2].(type) {
		case float64:
			limit = int(v)
		case int:
			limit = v
		default:
			return nil, types.NewError(types.ErrArgumentCountMismatch, "The third argument of the function '$split' must be a number", -1)
		}
		// Negative limit → D3020 error
		if limit < 0 {
			return nil, types.NewError("D3020", "Third argument of $split cannot be negative", -1)
		}
		// limit = 0 → empty array
		if limit == 0 {
			return []interface{}{}, nil
		}
	}

	// Check if separator is a regex or string
	var parts []string

	switch sep := args[1].(type) {
	case *regexp.Regexp:
		if limit > 0 {
			// Split all, then truncate
			allParts := sep.Split(str, -1)
			if len(allParts) > limit {
				allParts = allParts[:limit]
			}
			parts = allParts
		} else {
			parts = sep.Split(str, -1)
		}
	case string:
		if limit > 0 {
			// Split all, then truncate to limit
			allParts := strings.Split(str, sep)
			if len(allParts) > limit {
				allParts = allParts[:limit]
			}
			parts = allParts
		} else {
			parts = strings.Split(str, sep)
		}
	default:
		// Separator must be a string or regex
		return nil, types.NewError(types.ErrArgumentCountMismatch, "The second argument of the function '$split' must be a string or regex", -1)
	}

	result := make([]interface{}, len(parts))
	for i, p := range parts {
		result[i] = p
	}

	return result, nil
}

func fnJoin(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// undefined input → undefined
	if args[0] == nil {
		return nil, nil
	}

	// If first argument is a string, return it directly (like single-element join)
	if str, ok := args[0].(string); ok {
		return str, nil
	}

	// First argument must be an array
	arr, ok := args[0].([]interface{})
	if !ok {
		return nil, types.NewError("T0412", "The argument of the function '$join' is not an array", -1)
	}

	// Separator must be a string (if provided)
	separator := ""
	if len(args) == 2 && args[1] != nil {
		sep, ok := args[1].(string)
		if !ok {
			return nil, types.NewError(types.ErrArgumentCountMismatch, "The second argument of the function '$join' is not a string", -1)
		}
		separator = sep
	}

	// All array elements must be strings
	strs := make([]string, len(arr))
	for i, v := range arr {
		s, ok := v.(string)
		if !ok {
			return nil, types.NewError("T0412", "The argument of the function '$join' is not an array of strings", -1)
		}
		strs[i] = s
	}

	return strings.Join(strs, separator), nil
}

// --- Type Functions ---

func fnPad(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	strRunes := []rune(str)

	width, err := e.toNumber(args[1])
	if err != nil {
		return nil, err
	}
	targetWidth := int(width)

	// Default pad character is space
	padRunes := []rune{' '}
	if len(args) > 2 && args[2] != nil {
		padStr := e.toString(args[2])
		if len([]rune(padStr)) > 0 {
			padRunes = []rune(padStr)
		}
	}

	// Determine padding direction
	leftPad := targetWidth < 0
	if leftPad {
		targetWidth = -targetWidth
	}

	// Calculate padding needed (using rune count for Unicode correctness)
	strLen := len(strRunes)
	if strLen >= targetWidth {
		return str, nil
	}

	padCount := targetWidth - strLen

	// Build padding by cycling through pad runes
	padding := make([]rune, padCount)
	for i := 0; i < padCount; i++ {
		padding[i] = padRunes[i%len(padRunes)]
	}

	if leftPad {
		return string(padding) + string(strRunes), nil
	}
	return string(strRunes) + string(padding), nil
}

// fnSubstringBefore returns the substring before the first occurrence of a separator.
// Signature: $substringBefore(str, separator)

func fnSubstringBefore(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	// Validate first argument is a string
	str, ok := args[0].(string)
	if !ok {
		return nil, types.NewError(types.ErrArgumentCountMismatch, "Argument 1 of function 'substringBefore' must be a string", -1)
	}

	// Validate second argument is a string
	separator, ok := args[1].(string)
	if !ok {
		return nil, types.NewError(types.ErrArgumentCountMismatch, "Argument 2 of function 'substringBefore' must be a string", -1)
	}

	// If separator is empty, return empty string
	if separator == "" {
		return "", nil
	}

	idx := strings.Index(str, separator)
	if idx < 0 {
		// Separator not found, return the original string
		return str, nil
	}

	return str[:idx], nil
}

// fnSubstringAfter returns the substring after the first occurrence of a separator.
// Signature: $substringAfter(str, separator)

func fnSubstringAfter(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	// Validate first argument is a string
	str, ok := args[0].(string)
	if !ok {
		return nil, types.NewError(types.ErrArgumentCountMismatch, "Argument 1 of function 'substringAfter' must be a string", -1)
	}

	// Validate second argument is a string
	separator, ok := args[1].(string)
	if !ok {
		return nil, types.NewError(types.ErrArgumentCountMismatch, "Argument 2 of function 'substringAfter' must be a string", -1)
	}

	// If separator is empty, return the original string
	if separator == "" {
		return str, nil
	}

	idx := strings.Index(str, separator)
	if idx < 0 {
		// Separator not found, return the original string
		return str, nil
	}

	return str[idx+len(separator):], nil
}

// fnEval evaluates a JSONata expression string in the current context.
// $eval(expr[, bindings])
