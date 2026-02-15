package loader

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
)

// CompareResults compares actual result with expected result
func CompareResults(actual, expected interface{}, metadata TestCase) (bool, string) {
	// Handle undefined result
	if metadata.UndefinedResult {
		if actual == nil {
			return true, ""
		}
		return false, fmt.Sprintf("expected undefined, got %v", actual)
	}

	// Handle error expectations
	if metadata.Error != nil {
		return false, "expected error but got result"
	}

	// Deep equality with special handling
	if metadata.Unordered {
		return compareUnordered(actual, expected)
	}

	return compareOrdered(actual, expected)
}

// compareOrdered compares results with order-sensitive equality
func compareOrdered(actual, expected interface{}) (bool, string) {
	if !deepEqual(actual, expected) {
		return false, fmt.Sprintf(
			"result mismatch\n  Expected: %#v\n  Got:      %#v",
			expected, actual)
	}
	return true, ""
}

// compareUnordered compares results ignoring array/object order
func compareUnordered(actual, expected interface{}) (bool, string) {
	if !deepEqualUnordered(actual, expected) {
		return false, fmt.Sprintf(
			"result mismatch (unordered)\n  Expected: %#v\n  Got:      %#v",
			expected, actual)
	}
	return true, ""
}

// deepEqual performs deep equality with type coercion for numbers
func deepEqual(a, b interface{}) bool {
	// Nil handling
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Try standard reflection equality first
	if reflect.DeepEqual(a, b) {
		return true
	}

	// Handle numeric type coercion (Go float64 vs JavaScript number)
	if aNum, ok := toNumber(a); ok {
		if bNum, ok := toNumber(b); ok {
			return numbersClose(aNum, bNum)
		}
	}

	// Handle string comparison
	aStr, aIsStr := a.(string)
	bStr, bIsStr := b.(string)
	if aIsStr && bIsStr {
		return aStr == bStr
	}

	// Handle boolean comparison
	aBool, aIsBool := a.(bool)
	bBool, bIsBool := b.(bool)
	if aIsBool && bIsBool {
		return aBool == bBool
	}

	// Handle array/slice comparison
	aArr, aIsArr := a.([]interface{})
	bArr, bIsArr := b.([]interface{})
	if aIsArr && bIsArr {
		if len(aArr) != len(bArr) {
			return false
		}
		for i := range aArr {
			if !deepEqual(aArr[i], bArr[i]) {
				return false
			}
		}
		return true
	}

	// Handle map/object comparison
	aMap, aIsMap := a.(map[string]interface{})
	bMap, bIsMap := b.(map[string]interface{})
	if aIsMap && bIsMap {
		if len(aMap) != len(bMap) {
			return false
		}
		for k, v := range aMap {
			bv, ok := bMap[k]
			if !ok {
				return false
			}
			if !deepEqual(v, bv) {
				return false
			}
		}
		return true
	}

	// Handle OrderedObject (from GoSonata evaluator) - check by string type name
	aTypeName := reflect.TypeOf(a).String()
	bTypeName := reflect.TypeOf(b).String()

	aIsOrderedObject := strings.Contains(aTypeName, "OrderedObject")
	bIsOrderedObject := strings.Contains(bTypeName, "OrderedObject")

	// OrderedObject vs map comparison
	if aIsOrderedObject && !bIsOrderedObject {
		bMap, bIsMap := b.(map[string]interface{})
		if bIsMap {
			aVal := reflect.ValueOf(a)
			if aVal.Kind() == reflect.Ptr {
				aVal = aVal.Elem()
			}
			aValuesField := aVal.FieldByName("Values")
			if aValuesField.IsValid() {
				aMapIface := aValuesField.Interface()
				if aMap, ok := aMapIface.(map[string]interface{}); ok {
					return deepEqual(aMap, bMap)
				}
			}
		}
	}

	// map vs OrderedObject comparison
	if !aIsOrderedObject && bIsOrderedObject {
		aMap, aIsMap := a.(map[string]interface{})
		if aIsMap {
			bVal := reflect.ValueOf(b)
			if bVal.Kind() == reflect.Ptr {
				bVal = bVal.Elem()
			}
			bValuesField := bVal.FieldByName("Values")
			if bValuesField.IsValid() {
				bMapIface := bValuesField.Interface()
				if bMap, ok := bMapIface.(map[string]interface{}); ok {
					return deepEqual(aMap, bMap)
				}
			}
		}
	}

	// Both are OrderedObjects
	if aIsOrderedObject && bIsOrderedObject {
		// Both are OrderedObjects, compare using reflection
		aVal := reflect.ValueOf(a)
		bVal := reflect.ValueOf(b)

		// Dereference pointers if needed
		if aVal.Kind() == reflect.Ptr {
			aVal = aVal.Elem()
		}
		if bVal.Kind() == reflect.Ptr {
			bVal = bVal.Elem()
		}

		aValuesField := aVal.FieldByName("Values")
		bValuesField := bVal.FieldByName("Values")

		if aValuesField.IsValid() && bValuesField.IsValid() {
			aMapIface := aValuesField.Interface()
			bMapIface := bValuesField.Interface()

			aMap, aOk := aMapIface.(map[string]interface{})
			bMap, bOk := bMapIface.(map[string]interface{})

			if aOk && bOk {
				if len(aMap) != len(bMap) {
					return false
				}
				for k, v := range aMap {
					bv, ok := bMap[k]
					if !ok {
						return false
					}
					if !deepEqual(v, bv) {
						return false
					}
				}
				return true
			}
		}
	}

	return false
}

// deepEqualUnordered is like deepEqual but ignores array/object order
func deepEqualUnordered(a, b interface{}) bool {
	if deepEqual(a, b) {
		return true
	}

	// Try comparing as unordered collections
	aArr, aIsArr := a.([]interface{})
	bArr, bIsArr := b.([]interface{})
	if aIsArr && bIsArr {
		return unorderedArraysEqual(aArr, bArr)
	}

	return false
}

// toNumber tries to convert value to float64
func toNumber(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		var f float64
		if err := json.Unmarshal([]byte(val), &f); err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// numbersClose checks if two numbers are close (handles float precision)
func numbersClose(a, b float64) bool {
	// Direct equality
	if a == b {
		return true
	}

	// Check for NaN
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	if math.IsNaN(a) || math.IsNaN(b) {
		return false
	}

	// Check relative tolerance
	const epsilon = 1e-10
	if a == 0 || b == 0 {
		return math.Abs(a-b) < epsilon
	}

	relErr := math.Abs((a - b) / b)
	return relErr < epsilon
}

// unorderedArraysEqual compares arrays ignoring order
func unorderedArraysEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	// Create sorted copies and compare
	aCopy := make([]interface{}, len(a))
	bCopy := make([]interface{}, len(b))
	copy(aCopy, a)
	copy(bCopy, b)

	// Sort by string representation
	sortByString(aCopy)
	sortByString(bCopy)

	return reflect.DeepEqual(aCopy, bCopy)
}

// sortByString sorts slice by string representation
func sortByString(s []interface{}) {
	sort.Slice(s, func(i, j int) bool {
		return fmt.Sprintf("%v", s[i]) < fmt.Sprintf("%v", s[j])
	})
}
