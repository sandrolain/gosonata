package evaluator

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sandrolain/gosonata/pkg/types"
)

func (e *Evaluator) isTruthy(value interface{}) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0
	case int:
		return v != 0
	case types.Null:
		return false
	case []interface{}:
		return len(v) > 0
	case map[string]interface{}:
		return len(v) > 0
	case *OrderedObject:
		return len(v.Values) > 0
	default:
		return true
	}
}

// isTruthyBoolean implements the $boolean() function semantics:
// - Functions are always false
// - Arrays are true only if they contain at least one truthy element (recursively)
// - All other rules same as isTruthy

func (e *Evaluator) isTruthyBoolean(value interface{}) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0
	case int:
		return v != 0
	case types.Null:
		return false
	case []interface{}:
		// Array is true only if at least one element is truthy (recursively)
		for _, item := range v {
			if e.isTruthyBoolean(item) {
				return true
			}
		}
		return false
	case map[string]interface{}:
		return len(v) > 0
	case *OrderedObject:
		return len(v.Values) > 0
	case *Lambda, *FunctionDef:
		// Functions are always falsy in $boolean() context
		return false
	default:
		return true
	}
}

// isTruthyForDefault determines if a value is truthy for the default operator (?:).
// This has special semantics: arrays are truthy only if they contain at least one truthy value,
// and functions are considered falsy.

func (e *Evaluator) isTruthyForDefault(value interface{}) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0
	case int:
		return v != 0
	case types.Null:
		return false
	case []interface{}:
		// Array is truthy only if it contains at least one truthy element (recursively)
		for _, item := range v {
			if e.isTruthyForDefault(item) {
				return true
			}
		}
		return false
	case map[string]interface{}:
		return len(v) > 0
	case *OrderedObject:
		return len(v.Values) > 0
	case *Lambda:
		// Functions are falsy for the default operator
		return false
	default:
		// Other types (including functions) are considered falsy
		return false
	}
}

// toArray converts a value to an array.

func (e *Evaluator) toArray(value interface{}) ([]interface{}, error) {
	if value == nil {
		return []interface{}{}, nil
	}

	// Already an array
	if arr, ok := value.([]interface{}); ok {
		return arr, nil
	}

	// Single value becomes single-element array
	return []interface{}{value}, nil
}

// toNumber converts a value to a number.

func (e *Evaluator) toNumber(value interface{}) (float64, error) {
	// Handle undefined (nil) - return 0 but with error to signal undefined
	if value == nil {
		return 0, fmt.Errorf("undefined value")
	}

	switch v := value.(type) {
	case types.Null:
		return 0, fmt.Errorf("null value")
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case bool:
		// JSONata spec: true → 1, false → 0
		if v {
			return 1.0, nil
		}
		return 0.0, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to number", value)
	}
}

// tryNumber attempts to convert a value to number without error.
// Returns (value, true) if successful, (0, false) otherwise.
// NOTE: Does NOT convert bool to avoid issues with comparison operators.
// Bool should be handled explicitly in functions that need it (e.g., fnNumber, opEqual).

func (e *Evaluator) tryNumber(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	default:
		return 0, false
	}
}

// toString converts a value to a string.

func (e *Evaluator) toString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case types.Null:
		return "null"
	case string:
		return v
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return ""
		}
		return e.formatNumberForString(v)
	case int:
		return strconv.Itoa(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		// Arrays, objects, and other types: serialize as JSON
		b, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(b)
	}
}

// roundNumberForJSON rounds a float to 15 significant digits, matching JSONata.

func (e *Evaluator) roundNumberForJSON(v float64) float64 {
	str := strconv.FormatFloat(v, 'g', 15, 64)
	rounded, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return v
	}
	return rounded
}

// formatNumberForString formats numbers with JSONata's exponent rules.

func (e *Evaluator) formatNumberForString(v float64) string {
	rounded := e.roundNumberForJSON(v)
	abs := math.Abs(rounded)
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		str := strconv.FormatFloat(rounded, 'g', -1, 64)
		// Remove leading zero from exponent: 1e-07 → 1e-7
		str = strings.ReplaceAll(str, "e-0", "e-")
		str = strings.ReplaceAll(str, "e+0", "e+")
		str = strings.ReplaceAll(str, "E-0", "E-")
		str = strings.ReplaceAll(str, "E+0", "E+")
		return str
	}

	str := strconv.FormatFloat(rounded, 'f', 15, 64)

	// Handle floating-point artifacts: if we see patterns like ...9999... or ...0000...
	// these are likely precision errors. Common patterns:
	// 90.569999999999993 → should be 90.57
	// 245.789999999999992 → should be 245.79
	str = e.cleanFloatingPointArtifacts(str, rounded)

	str = strings.TrimRight(str, "0")
	str = strings.TrimRight(str, ".")
	if str == "" || str == "-0" {
		return "0"
	}
	return str
}

// cleanFloatingPointArtifacts removes floating-point representation errors.
// E.g., 90.569999999999993 → 90.57, 245.789999999999992 → 245.79

func (e *Evaluator) cleanFloatingPointArtifacts(str string, rounded float64) string {
	// Look for patterns of many repeated 9s or 0s
	// Pattern: find '9999' (4 or more 9s) or '0000' (4 or more 0s) as indicators of floating-point errors
	if idx := strings.Index(str, "9999"); idx >= 0 {
		// Try rounding to fewer decimal places
		parts := strings.Split(str, ".")
		if len(parts) == 2 {
			// Round up at the position before the 9s
			decimalPos := idx - len(parts[0]) - 1
			if decimalPos > 0 && decimalPos < len(parts[1]) {
				// Round to one less decimal place
				factor := math.Pow(10, float64(decimalPos))
				roundedUp := math.Round(rounded*factor) / factor
				return strconv.FormatFloat(roundedUp, 'f', decimalPos, 64)
			}
		}
	} else if idx := strings.Index(str, "0000"); idx >= 0 && idx > len(strings.Split(str, ".")[0]) {
		// For patterns like ...000001, truncate
		parts := strings.Split(str, ".")
		if len(parts) == 2 {
			decimalPos := idx - len(parts[0]) - 1
			if decimalPos > 0 && decimalPos < len(parts[1]) {
				factor := math.Pow(10, float64(decimalPos))
				roundedDown := math.Round(rounded*factor) / factor
				return strconv.FormatFloat(roundedDown, 'f', decimalPos, 64)
			}
		}
	}
	return str
}

// Arithmetic operators

// requireNumericOperand validates that a value is a numeric type for arithmetic operations.
// Returns T2001 error for non-numeric types (bool, string, object, etc.).
