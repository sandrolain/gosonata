// Package extarray provides extended array functions for GoSonata beyond the
// official JSONata spec.
package extarray

import (
	"context"
	"fmt"
	"math"

	"github.com/sandrolain/gosonata/pkg/functions"
)

// All returns all extended array function definitions (simple, no HOF).
func All() []functions.CustomFunctionDef {
	return []functions.CustomFunctionDef{
		First(),
		Last(),
		Take(),
		Skip(),
		Slice(),
		Flatten(),
		Chunk(),
		Union(),
		Intersection(),
		Difference(),
		SymmetricDifference(),
		Range(),
		ZipLongest(),
		Window(),
	}
}

// AllAdvanced returns advanced (HOF) extended array function definitions.
// These require a Caller to invoke lambda arguments.
func AllAdvanced() []functions.AdvancedCustomFunctionDef {
	return []functions.AdvancedCustomFunctionDef{
		GroupBy(),
		CountBy(),
		SumBy(),
		MinBy(),
		MaxBy(),
		Accumulate(),
	}
}

// AllEntries returns all array function definitions (simple + advanced) as
// [functions.FunctionEntry], suitable for spreading into [gosonata.WithFunctions]:
//
//	gosonata.WithFunctions(extarray.AllEntries()...)
func AllEntries() []functions.FunctionEntry {
	simple := All()
	adv := AllAdvanced()
	out := make([]functions.FunctionEntry, 0, len(simple)+len(adv))
	for _, f := range simple {
		out = append(out, f)
	}
	for _, f := range adv {
		out = append(out, f)
	}
	return out
}

// First returns the definition for $first(array).
func First() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "first",
		Signature: "<a:x>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$first: %w", err)
			}
			if len(arr) == 0 {
				return nil, nil
			}
			return arr[0], nil
		},
	}
}

// Last returns the definition for $last(array).
func Last() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "last",
		Signature: "<a:x>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$last: %w", err)
			}
			if len(arr) == 0 {
				return nil, nil
			}
			return arr[len(arr)-1], nil
		},
	}
}

// Take returns the definition for $take(array, n).
func Take() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "take",
		Signature: "<a-n:a>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$take: %w", err)
			}
			n, ok := toInt(args[1])
			if !ok {
				return nil, fmt.Errorf("$take: second argument must be a number")
			}
			if n < 0 {
				n = 0
			}
			if n > len(arr) {
				n = len(arr)
			}
			result := arr[:n]
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

// Skip returns the definition for $skip(array, n).
func Skip() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "skip",
		Signature: "<a-n:a>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$skip: %w", err)
			}
			n, ok := toInt(args[1])
			if !ok {
				return nil, fmt.Errorf("$skip: second argument must be a number")
			}
			if n < 0 {
				n = 0
			}
			if n > len(arr) {
				n = len(arr)
			}
			result := arr[n:]
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

// Slice returns the definition for $slice(array, start [, end]).
// start/end are 0-based; negative values count from the end.
func Slice() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "slice",
		Signature: "<a-n<n>?:a>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$slice: %w", err)
			}
			n := len(arr)
			start, ok := toInt(args[1])
			if !ok {
				return nil, fmt.Errorf("$slice: start must be a number")
			}
			start = normaliseIndex(start, n)
			end := n
			if len(args) >= 3 && args[2] != nil {
				e, ok := toInt(args[2])
				if !ok {
					return nil, fmt.Errorf("$slice: end must be a number")
				}
				end = normaliseIndex(e, n)
			}
			if start >= end {
				return nil, nil
			}
			result := arr[start:end]
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

// Flatten returns the definition for $flatten(array [, depth]).
// Without depth (or depth=-1), flattens completely.
func Flatten() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "flatten",
		Signature: "<a<n>?:a>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$flatten: %w", err)
			}
			depth := -1 // unlimited
			if len(args) >= 2 && args[1] != nil {
				d, ok := toInt(args[1])
				if !ok {
					return nil, fmt.Errorf("$flatten: depth must be a number")
				}
				depth = d
			}
			result := flattenArray(arr, depth)
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

func flattenArray(arr []interface{}, depth int) []interface{} {
	var result []interface{}
	for _, item := range arr {
		if inner, ok := item.([]interface{}); ok && depth != 0 {
			nextDepth := depth - 1
			if depth < 0 {
				nextDepth = depth // keep unlimited
			}
			result = append(result, flattenArray(inner, nextDepth)...)
		} else {
			result = append(result, item)
		}
	}
	return result
}

// Chunk returns the definition for $chunk(array, size).
func Chunk() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "chunk",
		Signature: "<a-n:a>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$chunk: %w", err)
			}
			size, ok := toInt(args[1])
			if !ok || size <= 0 {
				return nil, fmt.Errorf("$chunk: size must be a positive integer")
			}
			var chunks []interface{}
			for i := 0; i < len(arr); i += size {
				end := i + size
				if end > len(arr) {
					end = len(arr)
				}
				chunks = append(chunks, arr[i:end])
			}
			if len(chunks) == 0 {
				return nil, nil
			}
			return chunks, nil
		},
	}
}

// Union returns the definition for $union(arr1, arr2).
// Returns a deduplicated array containing all elements from both arrays.
func Union() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "union",
		Signature: "<a-a:a>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			a1, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$union: %w", err)
			}
			a2, err := toArray(args[1])
			if err != nil {
				return nil, fmt.Errorf("$union: %w", err)
			}
			seen := make(map[interface{}]bool)
			var result []interface{}
			for _, item := range append(a1, a2...) {
				key := fmt.Sprint(item)
				if !seen[key] {
					seen[key] = true
					result = append(result, item)
				}
			}
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

// Intersection returns the definition for $intersection(arr1, arr2).
func Intersection() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "intersection",
		Signature: "<a-a:a>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			a1, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$intersection: %w", err)
			}
			a2, err := toArray(args[1])
			if err != nil {
				return nil, fmt.Errorf("$intersection: %w", err)
			}
			set := make(map[string]bool)
			for _, item := range a2 {
				set[fmt.Sprint(item)] = true
			}
			var result []interface{}
			seen := make(map[string]bool)
			for _, item := range a1 {
				key := fmt.Sprint(item)
				if set[key] && !seen[key] {
					seen[key] = true
					result = append(result, item)
				}
			}
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

// Difference returns the definition for $difference(arr1, arr2).
// Elements in arr1 but not in arr2.
func Difference() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "difference",
		Signature: "<a-a:a>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			a1, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$difference: %w", err)
			}
			a2, err := toArray(args[1])
			if err != nil {
				return nil, fmt.Errorf("$difference: %w", err)
			}
			set := make(map[string]bool)
			for _, item := range a2 {
				set[fmt.Sprint(item)] = true
			}
			var result []interface{}
			seen := make(map[string]bool)
			for _, item := range a1 {
				key := fmt.Sprint(item)
				if !set[key] && !seen[key] {
					seen[key] = true
					result = append(result, item)
				}
			}
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

// SymmetricDifference returns the definition for $symmetricDifference(arr1, arr2).
// Elements in either arr1 or arr2 but not both.
func SymmetricDifference() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "symmetricDifference",
		Signature: "<a-a:a>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			a1, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$symmetricDifference: %w", err)
			}
			a2, err := toArray(args[1])
			if err != nil {
				return nil, fmt.Errorf("$symmetricDifference: %w", err)
			}
			set1 := make(map[string]bool)
			for _, item := range a1 {
				set1[fmt.Sprint(item)] = true
			}
			set2 := make(map[string]bool)
			for _, item := range a2 {
				set2[fmt.Sprint(item)] = true
			}
			seen := make(map[string]bool)
			var result []interface{}
			for _, item := range append(a1, a2...) {
				key := fmt.Sprint(item)
				if !seen[key] && (set1[key] != set2[key]) {
					seen[key] = true
					result = append(result, item)
				}
			}
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

// Range returns the definition for $range(start, end [, step]).
// Supports float steps. end is exclusive.
func Range() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "range",
		Signature: "<n-n<n>?:a<n>>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			start, err1 := toFloat(args[0])
			end, err2 := toFloat(args[1])
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("$range: start and end must be numbers")
			}
			step := 1.0
			if len(args) >= 3 && args[2] != nil {
				s, err := toFloat(args[2])
				if err != nil {
					return nil, fmt.Errorf("$range: step must be a number")
				}
				if s == 0 {
					return nil, fmt.Errorf("$range: step must not be zero")
				}
				step = s
			}
			var result []interface{}
			const maxItems = 100000
			for i := 0; ; i++ {
				v := start + float64(i)*step
				if step > 0 && v > end {
					break
				}
				if step < 0 && v < end {
					break
				}
				if i >= maxItems {
					return nil, fmt.Errorf("$range: would produce more than %d items", maxItems)
				}
				// Round to avoid floating-point accumulation errors
				v = math.Round(v*1e10) / 1e10
				result = append(result, v)
			}
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

// ZipLongest returns the definition for $zipLongest(arr1, arr2 [, fill]).
// Zips two arrays; shorter array is padded with fill (default nil).
func ZipLongest() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "zipLongest",
		Signature: "<a-a<x>?:a>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			a1, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$zipLongest: %w", err)
			}
			a2, err := toArray(args[1])
			if err != nil {
				return nil, fmt.Errorf("$zipLongest: %w", err)
			}
			var fill interface{}
			if len(args) >= 3 {
				fill = args[2]
			}
			length := len(a1)
			if len(a2) > length {
				length = len(a2)
			}
			result := make([]interface{}, length)
			for i := 0; i < length; i++ {
				v1 := fill
				v2 := fill
				if i < len(a1) {
					v1 = a1[i]
				}
				if i < len(a2) {
					v2 = a2[i]
				}
				result[i] = []interface{}{v1, v2}
			}
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

// Window returns the definition for $window(array, size, step).
// Returns a sliding window view over the array.
func Window() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "window",
		Signature: "<a-n-n:a>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$window: %w", err)
			}
			size, ok1 := toInt(args[1])
			step, ok2 := toInt(args[2])
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("$window: size and step must be numbers")
			}
			if size <= 0 || step <= 0 {
				return nil, fmt.Errorf("$window: size and step must be positive")
			}
			var result []interface{}
			for i := 0; i+size <= len(arr); i += step {
				result = append(result, arr[i:i+size])
			}
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

// ── Advanced (HOF) functions ────────────────────────────────────────────────

// GroupBy returns the AdvancedCustomFunctionDef for $groupBy(array, fn).
// fn(item) should return the group key.
func GroupBy() functions.AdvancedCustomFunctionDef {
	return functions.AdvancedCustomFunctionDef{
		Name:      "groupBy",
		Signature: "",
		Fn: func(ctx context.Context, caller functions.Caller, args ...interface{}) (interface{}, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("$groupBy: requires 2 arguments")
			}
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$groupBy: %w", err)
			}
			fn := args[1]
			keys := []string{}
			groups := make(map[string][]interface{})
			for _, item := range arr {
				keyRaw, err := caller.Call(ctx, fn, item)
				if err != nil {
					return nil, fmt.Errorf("$groupBy: %w", err)
				}
				key := fmt.Sprint(keyRaw)
				if _, exists := groups[key]; !exists {
					keys = append(keys, key)
					groups[key] = []interface{}{}
				}
				groups[key] = append(groups[key], item)
			}
			result := make(map[string]interface{}, len(groups))
			for k, v := range groups {
				_ = k
				_ = keys
				result[k] = v
			}
			return result, nil
		},
	}
}

// CountBy returns the AdvancedCustomFunctionDef for $countBy(array, fn).
// fn(item) returns a key; result is an object with counts per key.
func CountBy() functions.AdvancedCustomFunctionDef {
	return functions.AdvancedCustomFunctionDef{
		Name:      "countBy",
		Signature: "",
		Fn: func(ctx context.Context, caller functions.Caller, args ...interface{}) (interface{}, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("$countBy: requires 2 arguments")
			}
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$countBy: %w", err)
			}
			fn := args[1]
			result := make(map[string]interface{})
			for _, item := range arr {
				keyRaw, err := caller.Call(ctx, fn, item)
				if err != nil {
					return nil, fmt.Errorf("$countBy: %w", err)
				}
				key := fmt.Sprint(keyRaw)
				if cur, ok := result[key]; ok {
					result[key] = cur.(float64) + 1
				} else {
					result[key] = float64(1)
				}
			}
			return result, nil
		},
	}
}

// SumBy returns the AdvancedCustomFunctionDef for $sumBy(array, fn).
// fn(item) returns a number; result is the sum of all fn results.
func SumBy() functions.AdvancedCustomFunctionDef {
	return functions.AdvancedCustomFunctionDef{
		Name:      "sumBy",
		Signature: "",
		Fn: func(ctx context.Context, caller functions.Caller, args ...interface{}) (interface{}, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("$sumBy: requires 2 arguments")
			}
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$sumBy: %w", err)
			}
			fn := args[1]
			sum := 0.0
			for _, item := range arr {
				v, err := caller.Call(ctx, fn, item)
				if err != nil {
					return nil, fmt.Errorf("$sumBy: %w", err)
				}
				n, err := toFloat(v)
				if err != nil {
					return nil, fmt.Errorf("$sumBy: fn must return a number: %w", err)
				}
				sum += n
			}
			return sum, nil
		},
	}
}

// MinBy returns the AdvancedCustomFunctionDef for $minBy(array, fn).
// fn(item) returns a number; result is the item with the minimum fn result.
func MinBy() functions.AdvancedCustomFunctionDef {
	return functions.AdvancedCustomFunctionDef{
		Name:      "minBy",
		Signature: "",
		Fn: func(ctx context.Context, caller functions.Caller, args ...interface{}) (interface{}, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("$minBy: requires 2 arguments")
			}
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$minBy: %w", err)
			}
			fn := args[1]
			if len(arr) == 0 {
				return nil, nil
			}
			minVal := math.Inf(1)
			var minItem interface{}
			for _, item := range arr {
				v, err := caller.Call(ctx, fn, item)
				if err != nil {
					return nil, fmt.Errorf("$minBy: %w", err)
				}
				n, err := toFloat(v)
				if err != nil {
					return nil, fmt.Errorf("$minBy: fn must return a number: %w", err)
				}
				if n < minVal {
					minVal = n
					minItem = item
				}
			}
			return minItem, nil
		},
	}
}

// MaxBy returns the AdvancedCustomFunctionDef for $maxBy(array, fn).
// fn(item) returns a number; result is the item with the maximum fn result.
func MaxBy() functions.AdvancedCustomFunctionDef {
	return functions.AdvancedCustomFunctionDef{
		Name:      "maxBy",
		Signature: "",
		Fn: func(ctx context.Context, caller functions.Caller, args ...interface{}) (interface{}, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("$maxBy: requires 2 arguments")
			}
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$maxBy: %w", err)
			}
			fn := args[1]
			if len(arr) == 0 {
				return nil, nil
			}
			maxVal := math.Inf(-1)
			var maxItem interface{}
			for _, item := range arr {
				v, err := caller.Call(ctx, fn, item)
				if err != nil {
					return nil, fmt.Errorf("$maxBy: %w", err)
				}
				n, err := toFloat(v)
				if err != nil {
					return nil, fmt.Errorf("$maxBy: fn must return a number: %w", err)
				}
				if n > maxVal {
					maxVal = n
					maxItem = item
				}
			}
			return maxItem, nil
		},
	}
}

// Accumulate returns the AdvancedCustomFunctionDef for $accumulate(array, fn, init).
// Like $reduce but returns all intermediate values (scan/prefix).
func Accumulate() functions.AdvancedCustomFunctionDef {
	return functions.AdvancedCustomFunctionDef{
		Name:      "accumulate",
		Signature: "",
		Fn: func(ctx context.Context, caller functions.Caller, args ...interface{}) (interface{}, error) {
			if len(args) < 3 {
				return nil, fmt.Errorf("$accumulate: requires 3 arguments (array, fn, init)")
			}
			arr, err := toArray(args[0])
			if err != nil {
				return nil, fmt.Errorf("$accumulate: %w", err)
			}
			fn := args[1]
			acc := args[2]
			result := []interface{}{acc}
			for _, item := range arr {
				next, err := caller.Call(ctx, fn, acc, item)
				if err != nil {
					return nil, fmt.Errorf("$accumulate: %w", err)
				}
				acc = next
				result = append(result, acc)
			}
			return result, nil
		},
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func toArray(v interface{}) ([]interface{}, error) {
	if v == nil {
		return nil, nil
	}
	switch a := v.(type) {
	case []interface{}:
		return a, nil
	default:
		// Wrap single value as array
		return []interface{}{v}, nil
	}
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}

func toFloat(v interface{}) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("expected a number, got %T", v)
	}
}

func normaliseIndex(idx, length int) int {
	if idx < 0 {
		idx = length + idx
	}
	if idx < 0 {
		idx = 0
	}
	if idx > length {
		idx = length
	}
	return idx
}
