package evaluator

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/sandrolain/gosonata/pkg/types"
)

func fnAppend(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// If second argument is undefined, return first as-is
	if args[1] == nil {
		return args[0], nil
	}

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
	// Handle undefined
	if args[0] == nil {
		return nil, nil
	}

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

func fnDistinct(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	arr, err := e.toArray(args[0])
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	result := make([]interface{}, 0)

	for _, item := range arr {
		key := distinctCanonicalKey(item)
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

// distinctCanonicalKey produces a canonical string representation of a JSON value
// suitable for equality comparison in $distinct. Object keys are sorted to ensure
// that two objects with the same content but different insertion order compare equal.
func distinctCanonicalKey(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "N" // undefined/nil
	case types.Null:
		return "n" // JSON null
	case bool:
		if val {
			return "bt"
		}
		return "bf"
	case float64:
		// Fast-path: evita json.Marshal usando strconv, zero allocazioni aggiuntive
		return "f" + strconv.FormatFloat(val, 'f', -1, 64)
	case string:
		// Fast-path: il prefisso "s" garantisce unicità di tipo; il valore grezzo è canonico
		return "s" + val
	case *OrderedObject:
		// Sort keys for canonical comparison
		keys := make([]string, len(val.Keys))
		copy(keys, val.Keys)
		sort.Strings(keys)
		var buf strings.Builder
		buf.WriteString("o{")
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			// strconv.Quote produce la stessa forma quoted+escaped di json.Marshal per le stringhe
			buf.WriteString(strconv.Quote(k))
			buf.WriteByte(':')
			buf.WriteString(distinctCanonicalKey(val.Values[k]))
		}
		buf.WriteByte('}')
		return buf.String()
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var buf strings.Builder
		buf.WriteString("o{")
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(strconv.Quote(k))
			buf.WriteByte(':')
			buf.WriteString(distinctCanonicalKey(val[k]))
		}
		buf.WriteByte('}')
		return buf.String()
	case []interface{}:
		var buf strings.Builder
		buf.WriteString("a[")
		for i, item := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(distinctCanonicalKey(item))
		}
		buf.WriteByte(']')
		return buf.String()
	default:
		return fmt.Sprintf("%T:%v", val, val)
	}
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

	// If any argument is undefined, return empty array
	for _, arg := range args {
		if arg == nil {
			return []interface{}{}, nil
		}
	}

	// Convert all args to arrays
	arrays := make([][]interface{}, len(args))
	minLen := -1

	for i, arg := range args {
		arr, err := e.toArray(arg)
		if err != nil {
			return nil, err
		}
		arrays[i] = arr
		// Track minimum length
		if minLen == -1 || len(arr) < minLen {
			minLen = len(arr)
		}
	}

	// If any array is empty, return empty array
	if minLen == 0 {
		return []interface{}{}, nil
	}

	// Zip arrays together, stopping at shortest array length
	result := make([]interface{}, minLen)
	for i := 0; i < minLen; i++ {
		tuple := make([]interface{}, len(arrays))
		for j, arr := range arrays {
			tuple[j] = arr[i]
		}
		result[i] = tuple
	}

	return result, nil
}

// --- Enhanced String Functions (Fase 5.2) ---

// fnPad pads a string to a target width.
// Signature: $pad(str, width [, char])
// Pads on the right by default, negative width pads on the left.
