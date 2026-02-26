// Package extutil provides shared helpers for the ext sub-packages.
package extutil

import (
	"fmt"

	"github.com/sandrolain/gosonata/pkg/evaluator"
)

// AsObjectMap converts a JSONata object value (either map[string]interface{}
// or *evaluator.OrderedObject) into a plain map. Key order is NOT preserved.
func AsObjectMap(v interface{}) (map[string]interface{}, error) {
	if v == nil {
		return nil, fmt.Errorf("argument must be an object, got nil")
	}
	switch o := v.(type) {
	case map[string]interface{}:
		return o, nil
	case *evaluator.OrderedObject:
		return o.Values, nil
	default:
		return nil, fmt.Errorf("argument must be an object, got %T", v)
	}
}

// AsObjectOrdered converts a JSONata object into an ordered (keys, values) pair.
// Use this when insertion/declaration order must be preserved (e.g. $values, $pairs).
func AsObjectOrdered(v interface{}) ([]string, map[string]interface{}, error) {
	if v == nil {
		return nil, nil, fmt.Errorf("argument must be an object, got nil")
	}
	switch o := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(o))
		for k := range o {
			keys = append(keys, k)
		}
		return keys, o, nil
	case *evaluator.OrderedObject:
		return o.Keys, o.Values, nil
	default:
		return nil, nil, fmt.Errorf("argument must be an object, got %T", v)
	}
}

// IsObject returns true if v is a map or an *evaluator.OrderedObject.
func IsObject(v interface{}) bool {
	if v == nil {
		return false
	}
	switch v.(type) {
	case map[string]interface{}, *evaluator.OrderedObject:
		return true
	default:
		return false
	}
}

// ObjectLen returns the number of keys in a JSONata object.
// Returns -1 if v is not an object.
func ObjectLen(v interface{}) int {
	switch o := v.(type) {
	case map[string]interface{}:
		return len(o)
	case *evaluator.OrderedObject:
		return len(o.Keys)
	default:
		return -1
	}
}
