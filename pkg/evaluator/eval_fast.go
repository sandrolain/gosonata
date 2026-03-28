package evaluator

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tidwall/gjson"

	"github.com/sandrolain/gosonata/pkg/types"
)

// EvalFast attempts zero-copy fast-path evaluation of expr against raw JSON bytes.
//
// It returns (result, true, nil) when the expression's [types.FastPathInfo]
// indicates an eligible fast-path kind and evaluation succeeds.
// It returns (nil, false, nil) when no fast-path is available — the caller
// must fall back to full AST evaluation via [Evaluator.Eval].
//
// A non-nil error is returned only when the fast-path evaluation itself fails
// (e.g. malformed JSON inside a numeric conversion).
//
// This function never calls json.Unmarshal on the entire document.
func EvalFast(_ context.Context, fp *types.FastPathInfo, raw json.RawMessage) (any, bool, error) {
	if fp == nil {
		return nil, false, nil
	}

	r := gjson.GetBytes(raw, fp.GJSONPath)

	switch fp.Kind {
	case types.FastPathPurePath:
		if !r.Exists() {
			return nil, true, nil
		}
		return gjsonToAny(r), true, nil

	case types.FastPathComparison:
		result := evalComparison(r, fp.CompareOp, fp.CompareVal)
		return result, true, nil

	case types.FastPathFunc:
		result, err := evalFuncFast(fp.FuncKind, r, fp.FuncArg)
		return result, true, err
	}

	return nil, false, nil
}

// EvalFastFromResult is like [EvalFast] but accepts the pre-extracted
// gjson.Result directly. Used by StreamEvaluator to avoid re-extracting
// fields when gjson.GetManyBytes has already run a single-pass scan.
func EvalFastFromResult(fp *types.FastPathInfo, r gjson.Result) (any, bool, error) {
	if fp == nil {
		return nil, false, nil
	}

	switch fp.Kind {
	case types.FastPathPurePath:
		if !r.Exists() {
			return nil, true, nil
		}
		return gjsonToAny(r), true, nil

	case types.FastPathComparison:
		return evalComparison(r, fp.CompareOp, fp.CompareVal), true, nil

	case types.FastPathFunc:
		result, err := evalFuncFast(fp.FuncKind, r, fp.FuncArg)
		return result, true, err
	}

	return nil, false, nil
}

// gjsonToAny converts a gjson.Result to the Go representation used throughout
// the gosonata evaluator.
func gjsonToAny(r gjson.Result) any {
	switch r.Type {
	case gjson.Null:
		return types.NullValue
	case gjson.True:
		return true
	case gjson.False:
		return false
	case gjson.Number:
		return r.Float()
	case gjson.String:
		return r.String()
	case gjson.JSON:
		if r.IsArray() {
			var arr []any
			r.ForEach(func(_, v gjson.Result) bool {
				arr = append(arr, gjsonToAny(v))
				return true
			})
			return arr
		}
		// Object: decode to ordered map preserving insertion order.
		var m map[string]any
		_ = json.Unmarshal([]byte(r.Raw), &m)
		return m
	}
	return nil
}

// evalComparison evaluates "gjson_field op literal" and returns a bool.
// op must be "=" or "!=". literal is the raw JSON representation of the
// right-hand side (e.g. `"admin"`, `42`, `true`, `null`).
func evalComparison(r gjson.Result, op, literal string) any {
	var equal bool

	switch {
	case literal == "null":
		// JSONata null comparison: only null equals null.
		switch r.Type {
		case gjson.Null:
			equal = true
		default:
			equal = !r.Exists()
		}

	case literal == "true":
		equal = r.Type == gjson.True

	case literal == "false":
		equal = r.Type == gjson.False

	case len(literal) > 0 && literal[0] == '"':
		// String literal — strip JSON quotes.
		s, err := strconv.Unquote(literal)
		if err != nil {
			equal = false
		} else {
			equal = r.Type == gjson.String && r.String() == s
		}

	default:
		// Numeric literal.
		litF, err := strconv.ParseFloat(literal, 64)
		if err != nil {
			equal = false
		} else {
			equal = r.Type == gjson.Number && r.Float() == litF
		}
	}

	if op == "!=" {
		return !equal
	}
	return equal
}

// evalFuncFast applies a supported stdlib function to the result of a gjson
// field extraction. All operations run without json.Unmarshal on the document.
func evalFuncFast(kind types.FuncFastKind, r gjson.Result, arg string) (any, error) {
	switch kind {

	case types.FuncFastExists:
		// $exists returns true if the field is present, even when its value is null.
		// Only fields that are truly absent from the JSON return false.
		return r.Exists(), nil

	case types.FuncFastNot:
		return !jsonataTruthy(r), nil

	case types.FuncFastBoolean:
		return jsonataTruthy(r), nil

	case types.FuncFastString:
		if !r.Exists() {
			return nil, nil
		}
		switch r.Type {
		case gjson.String:
			return r.String(), nil
		case gjson.Number:
			f := r.Float()
			if f == math.Trunc(f) && !math.IsInf(f, 0) {
				return strconv.FormatInt(int64(f), 10), nil
			}
			return strconv.FormatFloat(f, 'f', -1, 64), nil
		case gjson.True:
			return "true", nil
		case gjson.False:
			return "false", nil
		case gjson.Null:
			return nil, nil
		default:
			// JSON object/array: marshal back to string
			return r.Raw, nil
		}

	case types.FuncFastNumber:
		if !r.Exists() {
			return nil, nil
		}
		switch r.Type {
		case gjson.Number:
			return r.Float(), nil
		case gjson.String:
			f, err := strconv.ParseFloat(strings.TrimSpace(r.String()), 64)
			if err != nil {
				return nil, nil
			}
			return f, nil
		case gjson.True:
			return float64(1), nil
		case gjson.False:
			return float64(0), nil
		default:
			return nil, nil
		}

	case types.FuncFastLowercase:
		if !r.Exists() || r.Type != gjson.String {
			return nil, nil
		}
		return strings.ToLower(r.String()), nil

	case types.FuncFastUppercase:
		if !r.Exists() || r.Type != gjson.String {
			return nil, nil
		}
		return strings.ToUpper(r.String()), nil

	case types.FuncFastTrim:
		if !r.Exists() || r.Type != gjson.String {
			return nil, nil
		}
		// JSONata $trim: collapse internal whitespace too, not just edges.
		return strings.Join(strings.Fields(r.String()), " "), nil

	case types.FuncFastLength:
		if !r.Exists() {
			return nil, nil
		}
		switch r.Type {
		case gjson.String:
			return float64(utf8.RuneCountInString(r.String())), nil
		case gjson.JSON:
			if r.IsArray() {
				count := float64(0)
				r.ForEach(func(_, _ gjson.Result) bool { count++; return true })
				return count, nil
			}
			return nil, nil
		default:
			return nil, nil
		}

	case types.FuncFastType:
		if !r.Exists() {
			return "undefined", nil
		}
		switch r.Type {
		case gjson.String:
			return "string", nil
		case gjson.Number:
			return "number", nil
		case gjson.True, gjson.False:
			return "boolean", nil
		case gjson.Null:
			return "null", nil
		case gjson.JSON:
			if r.IsArray() {
				return "array", nil
			}
			return "object", nil
		}
		return "undefined", nil

	case types.FuncFastContains:
		if !r.Exists() || r.Type != gjson.String {
			return false, nil
		}
		needle, err := strconv.Unquote(arg)
		if err != nil {
			needle = arg
		}
		return strings.Contains(r.String(), needle), nil

	case types.FuncFastKeys:
		if !r.Exists() || r.Type != gjson.JSON || r.IsArray() {
			return nil, nil
		}
		var keys []string
		r.ForEach(func(k, _ gjson.Result) bool {
			keys = append(keys, k.String())
			return true
		})
		// Sort to match full-evaluator behavior (regular maps have no guaranteed order).
		sort.Strings(keys)
		result := make([]any, len(keys))
		for i, k := range keys {
			result[i] = k
		}
		return result, nil

	case types.FuncFastDistinct:
		if !r.Exists() {
			return nil, nil
		}
		arr := r.Array()
		seen := make(map[string]bool, len(arr))
		result := make([]any, 0, len(arr))
		for _, item := range arr {
			key := item.Raw
			if !seen[key] {
				seen[key] = true
				result = append(result, gjsonToAny(item))
			}
		}
		if len(result) == 1 {
			return result[0], nil
		}
		return result, nil

	case types.FuncFastAbs:
		if !r.Exists() || r.Type != gjson.Number {
			return nil, nil
		}
		return math.Abs(r.Float()), nil

	case types.FuncFastFloor:
		if !r.Exists() || r.Type != gjson.Number {
			return nil, nil
		}
		return math.Floor(r.Float()), nil

	case types.FuncFastCeil:
		if !r.Exists() || r.Type != gjson.Number {
			return nil, nil
		}
		return math.Ceil(r.Float()), nil

	case types.FuncFastSqrt:
		if !r.Exists() || r.Type != gjson.Number {
			return nil, nil
		}
		return math.Sqrt(r.Float()), nil

	case types.FuncFastCount:
		if !r.Exists() {
			return float64(0), nil
		}
		if r.Type == gjson.JSON && r.IsArray() {
			count := float64(0)
			r.ForEach(func(_, _ gjson.Result) bool { count++; return true })
			return count, nil
		}
		// Non-array value: count is 1 (JSONata sequence semantics).
		return float64(1), nil

	case types.FuncFastReverse:
		if !r.Exists() {
			return nil, nil
		}
		arr := r.Array()
		result := make([]any, len(arr))
		for i, v := range arr {
			result[len(arr)-1-i] = gjsonToAny(v)
		}
		return result, nil

	case types.FuncFastSum:
		if !r.Exists() {
			return float64(0), nil
		}
		sum := float64(0)
		r.ForEach(func(_, v gjson.Result) bool {
			sum += v.Float()
			return true
		})
		return sum, nil

	case types.FuncFastMax:
		if !r.Exists() {
			return nil, nil
		}
		arr := r.Array()
		if len(arr) == 0 {
			return nil, nil
		}
		max := arr[0].Float()
		for _, v := range arr[1:] {
			if f := v.Float(); f > max {
				max = f
			}
		}
		return max, nil

	case types.FuncFastMin:
		if !r.Exists() {
			return nil, nil
		}
		arr := r.Array()
		if len(arr) == 0 {
			return nil, nil
		}
		min := arr[0].Float()
		for _, v := range arr[1:] {
			if f := v.Float(); f < min {
				min = f
			}
		}
		return min, nil

	case types.FuncFastAverage:
		if !r.Exists() {
			return nil, nil
		}
		arr := r.Array()
		if len(arr) == 0 {
			return nil, nil
		}
		sum := float64(0)
		for _, v := range arr {
			sum += v.Float()
		}
		return sum / float64(len(arr)), nil
	}

	return nil, nil
}

// jsonataTruthy implements the JSONata truthiness rules for a gjson.Result:
//   - false/null/undefined/""/0 → false
//   - everything else → true
func jsonataTruthy(r gjson.Result) bool {
	switch r.Type {
	case gjson.False, gjson.Null:
		return false
	case gjson.True:
		return true
	case gjson.Number:
		return r.Float() != 0
	case gjson.String:
		return r.String() != ""
	case gjson.JSON:
		if r.IsArray() {
			// Empty array → false; non-empty → true.
			hasItems := false
			r.ForEach(func(_, _ gjson.Result) bool { hasItems = true; return false })
			return hasItems
		}
		// Non-empty object → true.
		hasKeys := false
		r.ForEach(func(_, _ gjson.Result) bool { hasKeys = true; return false })
		return hasKeys
	default:
		// Not found / undefined.
		return false
	}
}
