// Package extobject provides extended object functions for GoSonata beyond the
// official JSONata spec.
package extobject

import (
	"context"
	"fmt"

	"github.com/sandrolain/gosonata/pkg/ext/extutil"
	"github.com/sandrolain/gosonata/pkg/functions"
)

// All returns all extended object function definitions (simple, no HOF).
func All() []functions.CustomFunctionDef {
	return []functions.CustomFunctionDef{
		Values(),
		Pairs(),
		FromPairs(),
		Pick(),
		Omit(),
		DeepMerge(),
		Invert(),
		Size(),
		Rename(),
	}
}

// AllAdvanced returns advanced (HOF) extended object function definitions.
func AllAdvanced() []functions.AdvancedCustomFunctionDef {
	return []functions.AdvancedCustomFunctionDef{
		MapValues(),
		MapKeys(),
	}
}

// AllEntries returns all object function definitions (simple + advanced) as
// [functions.FunctionEntry], suitable for spreading into [gosonata.WithFunctions].
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

// Values returns the definition for $values(object).
// Returns the values of the object as an array.
func Values() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "values",
		Signature: "<o:a>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			keys, vals, err := extutil.AsObjectOrdered(args[0])
			if err != nil {
				return nil, fmt.Errorf("$values: %w", err)
			}
			if len(keys) == 0 {
				return nil, nil
			}
			result := make([]interface{}, 0, len(keys))
			for _, k := range keys {
				result = append(result, vals[k])
			}
			return result, nil
		},
	}
}

// Pairs returns the definition for $pairs(object).
// Returns [[key, value], ...] for each key in the object.
func Pairs() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "pairs",
		Signature: "<o:a>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			keys, vals, err := extutil.AsObjectOrdered(args[0])
			if err != nil {
				return nil, fmt.Errorf("$pairs: %w", err)
			}
			if len(keys) == 0 {
				return nil, nil
			}
			result := make([]interface{}, 0, len(keys))
			for _, k := range keys {
				result = append(result, []interface{}{k, vals[k]})
			}
			return result, nil
		},
	}
}

// FromPairs returns the definition for $fromPairs(array).
// Converts [[key, value], ...] into an object.
func FromPairs() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "fromPairs",
		Signature: "<a:o>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			arr, ok := args[0].([]interface{})
			if !ok {
				return nil, fmt.Errorf("$fromPairs: argument must be an array")
			}
			result := make(map[string]interface{}, len(arr))
			for i, item := range arr {
				pair, ok := item.([]interface{})
				if !ok || len(pair) < 2 {
					return nil, fmt.Errorf("$fromPairs: element %d must be a [key, value] pair", i)
				}
				key, ok := pair[0].(string)
				if !ok {
					return nil, fmt.Errorf("$fromPairs: key at element %d must be a string", i)
				}
				result[key] = pair[1]
			}
			return result, nil
		},
	}
}

// Pick returns the definition for $pick(object, keys).
// Returns a new object containing only the specified keys.
func Pick() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "pick",
		Signature: "<o-a<s>:o>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			obj, err := extutil.AsObjectMap(args[0])
			if err != nil {
				return nil, fmt.Errorf("$pick: %w", err)
			}
			keysRaw, ok := args[1].([]interface{})
			if !ok {
				return nil, fmt.Errorf("$pick: second argument must be an array of strings")
			}
			result := make(map[string]interface{})
			for _, kr := range keysRaw {
				k, ok := kr.(string)
				if !ok {
					continue
				}
				if v, exists := obj[k]; exists {
					result[k] = v
				}
			}
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

// Omit returns the definition for $omit(object, keys).
// Returns a new object excluding the specified keys.
func Omit() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "omit",
		Signature: "<o-a<s>:o>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			obj, err := extutil.AsObjectMap(args[0])
			if err != nil {
				return nil, fmt.Errorf("$omit: %w", err)
			}
			keysRaw, ok := args[1].([]interface{})
			if !ok {
				return nil, fmt.Errorf("$omit: second argument must be an array of strings")
			}
			skip := make(map[string]bool, len(keysRaw))
			for _, kr := range keysRaw {
				if k, ok := kr.(string); ok {
					skip[k] = true
				}
			}
			result := make(map[string]interface{})
			for k, v := range obj {
				if !skip[k] {
					result[k] = v
				}
			}
			if len(result) == 0 {
				return nil, nil
			}
			return result, nil
		},
	}
}

// DeepMerge returns the definition for $deepMerge(array<object>).
// Recursively merges objects; later objects override earlier ones.
func DeepMerge() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "deepMerge",
		Signature: "<a<o>:o>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			arr, ok := args[0].([]interface{})
			if !ok {
				return nil, fmt.Errorf("$deepMerge: argument must be an array of objects")
			}
			result := make(map[string]interface{})
			for _, item := range arr {
				obj, err := extutil.AsObjectMap(item)
				if err != nil {
					return nil, fmt.Errorf("$deepMerge: all elements must be objects")
				}
				deepMergeInto(result, obj)
			}
			return result, nil
		},
	}
}

func deepMergeInto(dst, src map[string]interface{}) {
	for k, srcVal := range src {
		if srcMap, ok := srcVal.(map[string]interface{}); ok {
			if dstMap, ok := dst[k].(map[string]interface{}); ok {
				// Both are objects: recurse
				merged := make(map[string]interface{})
				for dk, dv := range dstMap {
					merged[dk] = dv
				}
				deepMergeInto(merged, srcMap)
				dst[k] = merged
				continue
			}
		}
		dst[k] = srcVal
	}
}

// Invert returns the definition for $invert(object).
// Swaps keys and values; values are converted to strings.
func Invert() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "invert",
		Signature: "<o:o>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			obj, err := extutil.AsObjectMap(args[0])
			if err != nil {
				return nil, fmt.Errorf("$invert: %w", err)
			}
			result := make(map[string]interface{}, len(obj))
			for k, v := range obj {
				result[fmt.Sprint(v)] = k
			}
			return result, nil
		},
	}
}

// Size returns the definition for $size(object).
// Returns the number of keys in the object.
func Size() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "size",
		Signature: "<o:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			obj, err := extutil.AsObjectMap(args[0])
			if err != nil {
				return nil, fmt.Errorf("$size: %w", err)
			}
			return float64(len(obj)), nil
		},
	}
}

// Rename returns the definition for $rename(object, mapping).
// Renames keys according to the mapping object.
func Rename() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "rename",
		Signature: "<o-o:o>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			obj, err := extutil.AsObjectMap(args[0])
			if err != nil {
				return nil, fmt.Errorf("$rename: %w", err)
			}
			mapping, err := extutil.AsObjectMap(args[1])
			if err != nil {
				return nil, fmt.Errorf("$rename: second argument must be an object")
			}
			result := make(map[string]interface{}, len(obj))
			for k, v := range obj {
				newKey := k
				if mapped, ok := mapping[k]; ok {
					if s, ok := mapped.(string); ok {
						newKey = s
					}
				}
				result[newKey] = v
			}
			return result, nil
		},
	}
}

// ── Advanced (HOF) functions ────────────────────────────────────────────────

// MapValues returns the AdvancedCustomFunctionDef for $mapValues(object, fn).
// fn(value, key) is called for each value; returns a new object with transformed values.
func MapValues() functions.AdvancedCustomFunctionDef {
	return functions.AdvancedCustomFunctionDef{
		Name:      "mapValues",
		Signature: "",
		Fn: func(ctx context.Context, caller functions.Caller, args ...interface{}) (interface{}, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("$mapValues: requires 2 arguments")
			}
			obj, err := extutil.AsObjectMap(args[0])
			if err != nil {
				return nil, fmt.Errorf("$mapValues: %w", err)
			}
			fn := args[1]
			result := make(map[string]interface{}, len(obj))
			for k, v := range obj {
				newVal, err := caller.Call(ctx, fn, v, k)
				if err != nil {
					return nil, fmt.Errorf("$mapValues: %w", err)
				}
				result[k] = newVal
			}
			return result, nil
		},
	}
}

// MapKeys returns the AdvancedCustomFunctionDef for $mapKeys(object, fn).
// fn(key, value) is called for each key; returns a new object with transformed keys.
func MapKeys() functions.AdvancedCustomFunctionDef {
	return functions.AdvancedCustomFunctionDef{
		Name:      "mapKeys",
		Signature: "",
		Fn: func(ctx context.Context, caller functions.Caller, args ...interface{}) (interface{}, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("$mapKeys: requires 2 arguments")
			}
			obj, err := extutil.AsObjectMap(args[0])
			if err != nil {
				return nil, fmt.Errorf("$mapKeys: %w", err)
			}
			fn := args[1]
			result := make(map[string]interface{}, len(obj))
			for k, v := range obj {
				newKeyRaw, err := caller.Call(ctx, fn, k, v)
				if err != nil {
					return nil, fmt.Errorf("$mapKeys: %w", err)
				}
				newKey, ok := newKeyRaw.(string)
				if !ok {
					newKey = fmt.Sprint(newKeyRaw)
				}
				result[newKey] = v
			}
			return result, nil
		},
	}
}
