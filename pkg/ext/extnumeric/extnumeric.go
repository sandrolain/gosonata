// Package extnumeric provides extended numeric functions for GoSonata beyond
// the official JSONata spec.
package extnumeric

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/sandrolain/gosonata/pkg/functions"
)

// All returns all extended numeric function definitions.
func All() []functions.CustomFunctionDef {
	return []functions.CustomFunctionDef{
		Log(),
		Sign(),
		Trunc(),
		Clamp(),
		Sin(),
		Cos(),
		Tan(),
		Asin(),
		Acos(),
		Atan(),
		Atan2(),
		Pi(),
		E(),
		Median(),
		Variance(),
		Stddev(),
		Percentile(),
		Mode(),
	}
}

// AllEntries returns all numeric function definitions as [functions.FunctionEntry],
// suitable for spreading into [gosonata.WithFunctions].
func AllEntries() []functions.FunctionEntry {
	all := All()
	out := make([]functions.FunctionEntry, len(all))
	for i, f := range all {
		out[i] = f
	}
	return out
}

// Log returns the definition for $log(n [, base]).
// Without base, returns the natural logarithm.
func Log() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "log",
		Signature: "<n<n>?:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			n, err := toFloat(args[0])
			if err != nil {
				return nil, fmt.Errorf("$log: %w", err)
			}
			if n <= 0 {
				return nil, fmt.Errorf("$log: argument must be positive")
			}
			if len(args) >= 2 && args[1] != nil {
				base, err := toFloat(args[1])
				if err != nil {
					return nil, fmt.Errorf("$log: %w", err)
				}
				if base <= 0 || base == 1 {
					return nil, fmt.Errorf("$log: base must be positive and not 1")
				}
				return math.Log(n) / math.Log(base), nil
			}
			return math.Log(n), nil
		},
	}
}

// Sign returns the definition for $sign(n).
// Returns -1, 0, or 1.
func Sign() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "sign",
		Signature: "<n:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			n, err := toFloat(args[0])
			if err != nil {
				return nil, fmt.Errorf("$sign: %w", err)
			}
			switch {
			case n < 0:
				return float64(-1), nil
			case n > 0:
				return float64(1), nil
			default:
				return float64(0), nil
			}
		},
	}
}

// Trunc returns the definition for $trunc(n).
// Truncates toward zero.
func Trunc() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "trunc",
		Signature: "<n:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			n, err := toFloat(args[0])
			if err != nil {
				return nil, fmt.Errorf("$trunc: %w", err)
			}
			return math.Trunc(n), nil
		},
	}
}

// Clamp returns the definition for $clamp(n, min, max).
func Clamp() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "clamp",
		Signature: "<n-n-n:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			n, err := toFloat(args[0])
			if err != nil {
				return nil, fmt.Errorf("$clamp: %w", err)
			}
			min, err := toFloat(args[1])
			if err != nil {
				return nil, fmt.Errorf("$clamp: %w", err)
			}
			max, err := toFloat(args[2])
			if err != nil {
				return nil, fmt.Errorf("$clamp: %w", err)
			}
			if n < min {
				return min, nil
			}
			if n > max {
				return max, nil
			}
			return n, nil
		},
	}
}

// Sin returns the definition for $sin(n).
func Sin() functions.CustomFunctionDef {
	return mathFunc1("sin", math.Sin)
}

// Cos returns the definition for $cos(n).
func Cos() functions.CustomFunctionDef {
	return mathFunc1("cos", math.Cos)
}

// Tan returns the definition for $tan(n).
func Tan() functions.CustomFunctionDef {
	return mathFunc1("tan", math.Tan)
}

// Asin returns the definition for $asin(n).
func Asin() functions.CustomFunctionDef {
	return mathFunc1("asin", math.Asin)
}

// Acos returns the definition for $acos(n).
func Acos() functions.CustomFunctionDef {
	return mathFunc1("acos", math.Acos)
}

// Atan returns the definition for $atan(n).
func Atan() functions.CustomFunctionDef {
	return mathFunc1("atan", math.Atan)
}

// Atan2 returns the definition for $atan2(y, x).
func Atan2() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "atan2",
		Signature: "<n-n:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			y, err := toFloat(args[0])
			if err != nil {
				return nil, fmt.Errorf("$atan2: %w", err)
			}
			x, err := toFloat(args[1])
			if err != nil {
				return nil, fmt.Errorf("$atan2: %w", err)
			}
			return math.Atan2(y, x), nil
		},
	}
}

// Pi returns the definition for $pi().
func Pi() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "pi",
		Signature: "<:n>",
		Fn: func(_ context.Context, _ ...interface{}) (interface{}, error) {
			return math.Pi, nil
		},
	}
}

// E returns the definition for $e().
func E() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "e",
		Signature: "<:n>",
		Fn: func(_ context.Context, _ ...interface{}) (interface{}, error) {
			return math.E, nil
		},
	}
}

// Median returns the definition for $median(array).
func Median() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "median",
		Signature: "<a<n>:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			nums, err := toFloatSlice(args[0])
			if err != nil {
				return nil, fmt.Errorf("$median: %w", err)
			}
			if len(nums) == 0 {
				return nil, nil
			}
			sorted := make([]float64, len(nums))
			copy(sorted, nums)
			sort.Float64s(sorted)
			mid := len(sorted) / 2
			if len(sorted)%2 == 0 {
				return (sorted[mid-1] + sorted[mid]) / 2, nil
			}
			return sorted[mid], nil
		},
	}
}

// Variance returns the definition for $variance(array).
func Variance() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "variance",
		Signature: "<a<n>:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			nums, err := toFloatSlice(args[0])
			if err != nil {
				return nil, fmt.Errorf("$variance: %w", err)
			}
			return calcVariance(nums)
		},
	}
}

// Stddev returns the definition for $stddev(array).
func Stddev() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "stddev",
		Signature: "<a<n>:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			nums, err := toFloatSlice(args[0])
			if err != nil {
				return nil, fmt.Errorf("$stddev: %w", err)
			}
			v, err := calcVariance(nums)
			if err != nil || v == nil {
				return v, err
			}
			return math.Sqrt(v.(float64)), nil
		},
	}
}

// Percentile returns the definition for $percentile(array, p).
// p is in range [0, 100].
func Percentile() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "percentile",
		Signature: "<a<n>-n:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			nums, err := toFloatSlice(args[0])
			if err != nil {
				return nil, fmt.Errorf("$percentile: %w", err)
			}
			p, err := toFloat(args[1])
			if err != nil {
				return nil, fmt.Errorf("$percentile: %w", err)
			}
			if p < 0 || p > 100 {
				return nil, fmt.Errorf("$percentile: p must be between 0 and 100")
			}
			if len(nums) == 0 {
				return nil, nil
			}
			sorted := make([]float64, len(nums))
			copy(sorted, nums)
			sort.Float64s(sorted)
			idx := p / 100 * float64(len(sorted)-1)
			lo := int(math.Floor(idx))
			hi := int(math.Ceil(idx))
			if lo == hi {
				return sorted[lo], nil
			}
			frac := idx - float64(lo)
			return sorted[lo]*(1-frac) + sorted[hi]*frac, nil
		},
	}
}

// Mode returns the definition for $mode(array).
// Returns the most frequent value; if multiple values have the same
// frequency, returns all of them as an array.
func Mode() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "mode",
		Signature: "<a<n>:x>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			nums, err := toFloatSlice(args[0])
			if err != nil {
				return nil, fmt.Errorf("$mode: %w", err)
			}
			if len(nums) == 0 {
				return nil, nil
			}
			counts := make(map[float64]int)
			for _, n := range nums {
				counts[n]++
			}
			maxCount := 0
			for _, c := range counts {
				if c > maxCount {
					maxCount = c
				}
			}
			var modes []interface{}
			for _, n := range nums {
				if counts[n] == maxCount {
					found := false
					for _, m := range modes {
						if m.(float64) == n {
							found = true
							break
						}
					}
					if !found {
						modes = append(modes, n)
					}
				}
			}
			if len(modes) == 1 {
				return modes[0], nil
			}
			return modes, nil
		},
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func mathFunc1(name string, fn func(float64) float64) functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      name,
		Signature: "<n:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			n, err := toFloat(args[0])
			if err != nil {
				return nil, fmt.Errorf("$%s: %w", name, err)
			}
			return fn(n), nil
		},
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

func toFloatSlice(v interface{}) ([]float64, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected an array, got %T", v)
	}
	nums := make([]float64, len(arr))
	for i, item := range arr {
		n, err := toFloat(item)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
		nums[i] = n
	}
	return nums, nil
}

func calcVariance(nums []float64) (interface{}, error) {
	if len(nums) == 0 {
		return nil, nil
	}
	sum := 0.0
	for _, n := range nums {
		sum += n
	}
	mean := sum / float64(len(nums))
	variance := 0.0
	for _, n := range nums {
		diff := n - mean
		variance += diff * diff
	}
	variance /= float64(len(nums))
	return variance, nil
}
