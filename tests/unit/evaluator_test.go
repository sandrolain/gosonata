package unit_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/parser"
)

// Helper functions

func eval(t *testing.T, query string, data interface{}) interface{} {
	t.Helper()

	expr, err := parser.Parse(query)
	if err != nil {
		t.Fatalf("Failed to parse %q: %v", query, err)
	}

	ev := evaluator.New()
	result, err := ev.Eval(context.Background(), expr, data)
	if err != nil {
		t.Fatalf("Failed to eval %q: %v", query, err)
	}

	return result
}

func evalExpectError(t *testing.T, query string, data interface{}) error {
	t.Helper()

	expr, err := parser.Parse(query)
	if err != nil {
		return err
	}

	ev := evaluator.New()
	_, err = ev.Eval(context.Background(), expr, data)
	return err
}

func compareFloat(t *testing.T, got, want float64) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func compareValue(t *testing.T, got, want interface{}) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Literal tests

func TestEvalLiterals(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  interface{}
	}{
		{"string", `"hello"`, "hello"},
		{"number int", "42", 42.0},
		{"number float", "3.14", 3.14},
		{"boolean true", "true", true},
		{"boolean false", "false", false},
		{"null", "null", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if result != tt.want {
				t.Errorf("got %v, want %v", result, tt.want)
			}
		})
	}
}

// Variable tests

func TestEvalVariables(t *testing.T) {
	data := map[string]interface{}{
		"name":   "John",
		"age":    30.0,
		"active": true,
	}

	tests := []struct {
		name  string
		query string
		want  interface{}
	}{
		{"context", "$", data},
		{"field", "name", "John"},
		{"number field", "age", 30.0},
		{"boolean field", "active", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, data)
			compareValue(t, result, tt.want)
		})
	}
}

// Arithmetic operator tests

func TestEvalArithmetic(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  float64
	}{
		{"addition", "2 + 3", 5.0},
		{"subtraction", "10 - 7", 3.0},
		{"multiplication", "4 * 5", 20.0},
		{"division", "20 / 4", 5.0},
		{"modulo", "10 % 3", 1.0},
		{"negation", "-5", -5.0},
		{"complex", "2 + 3 * 4", 14.0},
		{"with parens", "(2 + 3) * 4", 20.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if num, ok := result.(float64); ok {
				compareFloat(t, num, tt.want)
			} else {
				t.Errorf("got %T, want float64", result)
			}
		})
	}
}

func TestEvalArithmeticErrors(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		expectError bool
		checkValue  func(interface{}) bool
	}{
		{
			name:  "division by zero",
			query: "10 / 0",
			// Current implementation returns D1001 for division by zero.
			// JSONata JS returns +Infinity; this is a known difference (TODO).
			expectError: true,
		},
		{
			name:        "modulo by zero",
			query:       "10 % 0",
			expectError: true, // Modulo by zero is an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectError {
				err := evalExpectError(t, tt.query, nil)
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				result := eval(t, tt.query, nil)
				if tt.checkValue != nil && !tt.checkValue(result) {
					t.Errorf("value check failed for %v", result)
				}
			}
		})
	}
}

// Comparison operator tests

func TestEvalComparison(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"equal true", "5 = 5", true},
		{"equal false", "5 = 3", false},
		{"not equal true", "5 != 3", true},
		{"not equal false", "5 != 5", false},
		{"less true", "3 < 5", true},
		{"less false", "5 < 3", false},
		{"less equal true", "5 <= 5", true},
		{"less equal false", "6 <= 5", false},
		{"greater true", "5 > 3", true},
		{"greater false", "3 > 5", false},
		{"greater equal true", "5 >= 5", true},
		{"greater equal false", "4 >= 5", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if result != tt.want {
				t.Errorf("got %v, want %v", result, tt.want)
			}
		})
	}
}

// Logical operator tests

func TestEvalLogical(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"and true", "true and true", true},
		{"and false left", "false and true", false},
		{"and false right", "true and false", false},
		{"and both false", "false and false", false},
		{"or true left", "true or false", true},
		{"or true right", "false or true", true},
		{"or both true", "true or true", true},
		{"or both false", "false or false", false},
		{"complex", "true and false or true", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if result != tt.want {
				t.Errorf("got %v, want %v", result, tt.want)
			}
		})
	}
}

// String operator tests

func TestEvalStringConcat(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"simple", `"hello" & " " & "world"`, "hello world"},
		{"with number", `"value: " & 42`, "value: 42"},
		{"empty string", `"" & "test"`, "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if result != tt.want {
				t.Errorf("got %v, want %v", result, tt.want)
			}
		})
	}
}

// Path navigation tests

func TestEvalPath(t *testing.T) {
	data := map[string]interface{}{
		"user": map[string]interface{}{
			"name": "Alice",
			"address": map[string]interface{}{
				"city": "NYC",
				"zip":  "10001",
			},
		},
		"count": 5.0,
	}

	tests := []struct {
		name  string
		query string
		want  interface{}
	}{
		{"simple path", "user.name", "Alice"},
		{"nested path", "user.address.city", "NYC"},
		{"deep path", "user.address.zip", "10001"},
		{"path from context", "$.user.name", "Alice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, data)
			if result != tt.want {
				t.Errorf("got %v, want %v", result, tt.want)
			}
		})
	}
}

func TestEvalPathMissing(t *testing.T) {
	data := map[string]interface{}{
		"name": "test",
	}

	tests := []struct {
		name  string
		query string
	}{
		{"missing field", "missing"},
		{"missing nested", "name.missing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, data)
			if result != nil {
				t.Errorf("got %v, want nil", result)
			}
		})
	}
}

// Array constructor tests

func TestEvalArray(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []interface{}
	}{
		{"empty", "[]", []interface{}{}},
		{"numbers", "[1, 2, 3]", []interface{}{1.0, 2.0, 3.0}},
		{"mixed", `[1, "two", true]`, []interface{}{1.0, "two", true}},
		{"with expressions", "[1 + 1, 2 * 2]", []interface{}{2.0, 4.0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			arr, ok := result.([]interface{})
			if !ok {
				t.Fatalf("got %T, want []interface{}", result)
			}

			if len(arr) != len(tt.want) {
				t.Fatalf("got length %d, want %d", len(arr), len(tt.want))
			}

			for i := range arr {
				if arr[i] != tt.want[i] {
					t.Errorf("element %d: got %v, want %v", i, arr[i], tt.want[i])
				}
			}
		})
	}
}

// Object constructor tests

func TestEvalObject(t *testing.T) {
	tests := []struct {
		name  string
		query string
		check func(t *testing.T, result interface{})
	}{
		{
			name:  "empty",
			query: "{}",
			check: func(t *testing.T, result interface{}) {
				// Accept both map and OrderedObject
				switch obj := result.(type) {
				case map[string]interface{}:
					if len(obj) != 0 {
						t.Errorf("got length %d, want 0", len(obj))
					}
				case *evaluator.OrderedObject:
					if len(obj.Keys) != 0 {
						t.Errorf("got length %d, want 0", len(obj.Keys))
					}
				default:
					t.Fatalf("got %T, want map or OrderedObject", result)
				}
			},
		},
		{
			name:  "simple",
			query: `{"name": "Alice", "age": 30}`,
			check: func(t *testing.T, result interface{}) {
				// Accept both map and OrderedObject
				var name interface{}
				var age interface{}
				switch obj := result.(type) {
				case map[string]interface{}:
					name = obj["name"]
					age = obj["age"]
				case *evaluator.OrderedObject:
					name, _ = obj.Get("name")
					age, _ = obj.Get("age")
				default:
					t.Fatalf("got %T, want map or OrderedObject", result)
				}
				if name != "Alice" {
					t.Errorf("got name %v, want Alice", name)
				}
				if age != 30.0 {
					t.Errorf("got age %v, want 30", age)
				}
			},
		},
		{
			name:  "with expressions",
			query: `{"sum": 2 + 3, "product": 4 * 5}`,
			check: func(t *testing.T, result interface{}) {
				var sum, product interface{}
				switch obj := result.(type) {
				case map[string]interface{}:
					sum = obj["sum"]
					product = obj["product"]
				case *evaluator.OrderedObject:
					sum, _ = obj.Get("sum")
					product, _ = obj.Get("product")
				default:
					t.Fatalf("got %T, want map or OrderedObject", result)
				}
				if sum != 5.0 {
					t.Errorf("got sum %v, want 5", sum)
				}
				if product != 20.0 {
					t.Errorf("got product %v, want 20", product)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			tt.check(t, result)
		})
	}
}

// Filter tests

func TestEvalFilter(t *testing.T) {
	data := []interface{}{
		map[string]interface{}{"name": "Alice", "age": 25.0},
		map[string]interface{}{"name": "Bob", "age": 30.0},
		map[string]interface{}{"name": "Charlie", "age": 35.0},
	}

	tests := []struct {
		name  string
		query string
		check func(t *testing.T, result interface{})
	}{
		{
			name:  "simple filter",
			query: "$[age > 28]",
			check: func(t *testing.T, result interface{}) {
				arr, ok := result.([]interface{})
				if !ok {
					t.Fatalf("got %T, want []interface{}", result)
				}
				if len(arr) != 2 {
					t.Errorf("got length %d, want 2", len(arr))
				}
			},
		},
		{
			name:  "equality filter",
			query: "$[name = \"Bob\"]",
			check: func(t *testing.T, result interface{}) {
				// Filter with single result may be unwrapped from array
				if arr, ok := result.([]interface{}); ok {
					if len(arr) != 1 {
						t.Errorf("got array length %d, want 1", len(arr))
					}
				} else if obj, ok := result.(map[string]interface{}); ok {
					// Singleton unwrapping - check it's Bob
					if obj["name"] != "Bob" {
						t.Errorf("got name %v, want Bob", obj["name"])
					}
				} else {
					t.Fatalf("got %T, want []interface{} or map", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, data)
			tt.check(t, result)
		})
	}
}

// Conditional tests

func TestEvalConditional(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  interface{}
	}{
		{"true condition", "true ? 'yes' : 'no'", "yes"},
		{"false condition", "false ? 'yes' : 'no'", "no"},
		{"with expression", "5 > 3 ? 'greater' : 'lesser'", "greater"},
		{"nested", "true ? (false ? 'a' : 'b') : 'c'", "b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if result != tt.want {
				t.Errorf("got %v, want %v", result, tt.want)
			}
		})
	}
}

// Range tests

func TestEvalRange(t *testing.T) {
	tests := []struct {
		name  string
		query string
		check func(t *testing.T, result interface{})
	}{
		{
			name:  "simple range",
			query: "1..5",
			check: func(t *testing.T, result interface{}) {
				arr, ok := result.([]interface{})
				if !ok {
					t.Fatalf("got %T, want []interface{}", result)
				}
				want := []float64{1, 2, 3, 4, 5}
				if len(arr) != len(want) {
					t.Fatalf("got length %d, want %d", len(arr), len(want))
				}
				for i, v := range want {
					if arr[i] != v {
						t.Errorf("element %d: got %v, want %v", i, arr[i], v)
					}
				}
			},
		},
		{
			name:  "reverse range",
			query: "5..1",
			check: func(t *testing.T, result interface{}) {
				// Current implementation returns an empty slice for descending ranges.
				// JSONata JS returns undefined; this is a known difference (TODO).
				arr, ok := result.([]interface{})
				if !ok {
					t.Fatalf("got %T, want []interface{}", result)
				}
				if len(arr) != 0 {
					t.Errorf("expected empty slice, got %v", arr)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			tt.check(t, result)
		})
	}
}

// Assignment tests

func TestEvalAssignment(t *testing.T) {
	ev := evaluator.New()

	expr, err := parser.Parse("$x := 42")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	result, err := ev.Eval(context.Background(), expr, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	if result != 42.0 {
		t.Errorf("got %v, want 42", result)
	}
}

// In operator tests

func TestEvalIn(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"in array true", "2 in [1, 2, 3]", true},
		{"in array false", "4 in [1, 2, 3]", false},
		{"string in", `"b" in ["a", "b", "c"]`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if result != tt.want {
				t.Errorf("got %v, want %v", result, tt.want)
			}
		})
	}
}

// Complex integration tests

func TestEvalComplexExpressions(t *testing.T) {
	data := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"name": "Item1", "price": 100.0, "quantity": 2.0},
			map[string]interface{}{"name": "Item2", "price": 50.0, "quantity": 5.0},
			map[string]interface{}{"name": "Item3", "price": 200.0, "quantity": 1.0},
		},
	}

	tests := []struct {
		name  string
		query string
		check func(t *testing.T, result interface{})
	}{
		{
			name:  "filter with path",
			query: "items[price > 75].name",
			check: func(t *testing.T, result interface{}) {
				// Should return names of items with price > 75
				if result == nil {
					t.Error("got nil result")
				}
			},
		},
		{
			name:  "conditional with path",
			query: "items[0].price > 50 ? 'expensive' : 'cheap'",
			check: func(t *testing.T, result interface{}) {
				if result != "expensive" {
					t.Errorf("got %v, want expensive", result)
				}
			},
		},
		{
			name:  "array of computed values",
			query: "[items[0].price, items[1].price, items[2].price]",
			check: func(t *testing.T, result interface{}) {
				arr, ok := result.([]interface{})
				if !ok {
					t.Fatalf("got %T, want []interface{}", result)
				}
				if len(arr) != 3 {
					t.Errorf("got length %d, want 3", len(arr))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, data)
			tt.check(t, result)
		})
	}
}
