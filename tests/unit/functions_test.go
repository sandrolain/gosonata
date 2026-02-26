package unit_test

import (
	"math"
	"testing"
)

// --- Aggregation Function Tests ---

func TestFnSum(t *testing.T) {
	tests := []struct {
		name  string
		query string
		data  interface{}
		want  float64
	}{
		{"simple array", "$sum([1, 2, 3, 4, 5])", nil, 15.0},
		{"from data", "$sum(numbers)", map[string]interface{}{"numbers": []interface{}{10.0, 20.0, 30.0}}, 60.0},
		{"empty array", "$sum([])", nil, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, tt.data)
			if num, ok := result.(float64); ok {
				compareFloat(t, num, tt.want)
			} else {
				t.Errorf("got %T, want float64", result)
			}
		})
	}
}

func TestFnCount(t *testing.T) {
	tests := []struct {
		name  string
		query string
		data  interface{}
		want  float64
	}{
		{"simple array", "$count([1, 2, 3])", nil, 3.0},
		{"from data", "$count(items)", map[string]interface{}{"items": []interface{}{"a", "b", "c", "d"}}, 4.0},
		{"empty array", "$count([])", nil, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, tt.data)
			if num, ok := result.(float64); ok {
				compareFloat(t, num, tt.want)
			} else {
				t.Errorf("got %T, want float64", result)
			}
		})
	}
}

func TestFnAverage(t *testing.T) {
	tests := []struct {
		name  string
		query string
		data  interface{}
		want  float64
	}{
		{"simple array", "$average([10, 20, 30])", nil, 20.0},
		{"from data", "$average(values)", map[string]interface{}{"values": []interface{}{5.0, 10.0, 15.0}}, 10.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, tt.data)
			if num, ok := result.(float64); ok {
				compareFloat(t, num, tt.want)
			} else {
				t.Errorf("got %T, want float64", result)
			}
		})
	}
}

func TestFnMinMax(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  float64
	}{
		{"min", "$min([5, 2, 8, 1, 9])", 1.0},
		{"max", "$max([5, 2, 8, 1, 9])", 9.0},
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

// --- String Function Tests ---

func TestFnString(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"number to string", "$string(42)", "42"},
		{"boolean to string", "$string(true)", "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if str, ok := result.(string); ok {
				if str != tt.want {
					t.Errorf("got %q, want %q", str, tt.want)
				}
			} else {
				t.Errorf("got %T, want string", result)
			}
		})
	}
}

func TestFnLength(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  float64
	}{
		{"string length", `$length("hello")`, 5.0},
		// Note: $length() accepts only strings per JSONata spec.
		// Use $count() for arrays.
		{"array count", "$count([1, 2, 3])", 3.0},
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

func TestFnSubstring(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"from start", `$substring("hello", 1)`, "ello"},
		{"with length", `$substring("hello", 1, 3)`, "ell"},
		{"zero start", `$substring("hello", 0, 2)`, "he"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if str, ok := result.(string); ok {
				if str != tt.want {
					t.Errorf("got %q, want %q", str, tt.want)
				}
			} else {
				t.Errorf("got %T, want string", result)
			}
		})
	}
}

func TestFnUpperLowercase(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"uppercase", `$uppercase("hello")`, "HELLO"},
		{"lowercase", `$lowercase("WORLD")`, "world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if str, ok := result.(string); ok {
				if str != tt.want {
					t.Errorf("got %q, want %q", str, tt.want)
				}
			} else {
				t.Errorf("got %T, want string", result)
			}
		})
	}
}

func TestFnTrim(t *testing.T) {
	result := eval(t, `$trim("  hello  ")`, nil)
	if str, ok := result.(string); ok {
		if str != "hello" {
			t.Errorf("got %q, want %q", str, "hello")
		}
	} else {
		t.Errorf("got %T, want string", result)
	}
}

func TestFnContains(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"contains true", `$contains("hello world", "world")`, true},
		{"contains false", `$contains("hello world", "foo")`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if b, ok := result.(bool); ok {
				if b != tt.want {
					t.Errorf("got %v, want %v", b, tt.want)
				}
			} else {
				t.Errorf("got %T, want bool", result)
			}
		})
	}
}

func TestFnSplit(t *testing.T) {
	result := eval(t, `$split("a,b,c", ",")`, nil)
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("got %T, want []interface{}", result)
	}

	if len(arr) != 3 {
		t.Errorf("got length %d, want 3", len(arr))
	}

	if arr[0] != "a" || arr[1] != "b" || arr[2] != "c" {
		t.Errorf("got %v, want [a b c]", arr)
	}
}

func TestFnJoin(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"with separator", `$join(["a", "b", "c"], ",")`, "a,b,c"},
		{"without separator", `$join(["a", "b", "c"])`, "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if str, ok := result.(string); ok {
				if str != tt.want {
					t.Errorf("got %q, want %q", str, tt.want)
				}
			} else {
				t.Errorf("got %T, want string", result)
			}
		})
	}
}

// --- Type Function Tests ---

func TestFnType(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"string type", `$type("hello")`, "string"},
		{"number type", "$type(42)", "number"},
		{"boolean type", "$type(true)", "boolean"},
		{"array type", "$type([1,2,3])", "array"},
		{"object type", `$type({"key": "value"})`, "object"},
		{"null type", "$type(null)", "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if str, ok := result.(string); ok {
				if str != tt.want {
					t.Errorf("got %q, want %q", str, tt.want)
				}
			} else {
				t.Errorf("got %T, want string", result)
			}
		})
	}
}

func TestFnExists(t *testing.T) {
	tests := []struct {
		name  string
		query string
		data  interface{}
		want  bool
	}{
		{"exists true", "$exists(name)", map[string]interface{}{"name": "John"}, true},
		{"exists false", "$exists(missing)", map[string]interface{}{"name": "John"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, tt.data)
			if b, ok := result.(bool); ok {
				if b != tt.want {
					t.Errorf("got %v, want %v", b, tt.want)
				}
			} else {
				t.Errorf("got %T, want bool", result)
			}
		})
	}
}

func TestFnNumber(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  float64
	}{
		{"string to number", `$number("42")`, 42.0},
		{"string with decimal", `$number("3.14")`, 3.14},
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

func TestFnBoolean(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"truthy number", "$boolean(1)", true},
		{"falsy number", "$boolean(0)", false},
		{"truthy string", `$boolean("hello")`, true},
		{"falsy string", `$boolean("")`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if b, ok := result.(bool); ok {
				if b != tt.want {
					t.Errorf("got %v, want %v", b, tt.want)
				}
			} else {
				t.Errorf("got %T, want bool", result)
			}
		})
	}
}

// --- Math Function Tests ---

func TestFnAbs(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  float64
	}{
		{"positive", "$abs(5)", 5.0},
		{"negative", "$abs(-5)", 5.0},
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

func TestFnFloorCeil(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  float64
	}{
		{"floor", "$floor(3.7)", 3.0},
		{"ceil", "$ceil(3.2)", 4.0},
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

func TestFnRound(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  float64
	}{
		{"simple round", "$round(3.5)", 4.0},
		{"with precision", "$round(3.14159, 2)", 3.14},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval(t, tt.query, nil)
			if num, ok := result.(float64); ok {
				if math.Abs(num-tt.want) > 0.0001 {
					t.Errorf("got %v, want %v", num, tt.want)
				}
			} else {
				t.Errorf("got %T, want float64", result)
			}
		})
	}
}

func TestFnSqrt(t *testing.T) {
	result := eval(t, "$sqrt(16)", nil)
	if num, ok := result.(float64); ok {
		compareFloat(t, num, 4.0)
	} else {
		t.Errorf("got %T, want float64", result)
	}
}

func TestFnPower(t *testing.T) {
	result := eval(t, "$power(2, 3)", nil)
	if num, ok := result.(float64); ok {
		compareFloat(t, num, 8.0)
	} else {
		t.Errorf("got %T, want float64", result)
	}
}

// --- Array Function Tests with Lambdas ---

func TestFnMap(t *testing.T) {
	result := eval(t, "$map([1, 2, 3], function($x) { $x * 2 })", nil)
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("got %T, want []interface{}", result)
	}

	want := []float64{2.0, 4.0, 6.0}
	if len(arr) != len(want) {
		t.Fatalf("got length %d, want %d", len(arr), len(want))
	}

	for i, v := range want {
		if arr[i] != v {
			t.Errorf("element %d: got %v, want %v", i, arr[i], v)
		}
	}
}

func TestFnFilter(t *testing.T) {
	result := eval(t, "$filter([1, 2, 3, 4, 5], function($x) { $x > 2 })", nil)
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("got %T, want []interface{}", result)
	}

	want := []float64{3.0, 4.0, 5.0}
	if len(arr) != len(want) {
		t.Fatalf("got length %d, want %d", len(arr), len(want))
	}

	for i, v := range want {
		if arr[i] != v {
			t.Errorf("element %d: got %v, want %v", i, arr[i], v)
		}
	}
}

func TestFnReduce(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  float64
	}{
		{"sum with reduce", "$reduce([1, 2, 3, 4], function($acc, $x) { $acc + $x }, 0)", 10.0},
		{"product with reduce", "$reduce([2, 3, 4], function($acc, $x) { $acc * $x }, 1)", 24.0},
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

func TestFnSort(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []float64
	}{
		{"default sort", "$sort([3, 1, 4, 1, 5])", []float64{1.0, 1.0, 3.0, 4.0, 5.0}},
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

			for i, v := range tt.want {
				if arr[i] != v {
					t.Errorf("element %d: got %v, want %v", i, arr[i], v)
				}
			}
		})
	}
}

func TestFnAppend(t *testing.T) {
	result := eval(t, "$append([1, 2], [3, 4])", nil)
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("got %T, want []interface{}", result)
	}

	want := []float64{1.0, 2.0, 3.0, 4.0}
	if len(arr) != len(want) {
		t.Fatalf("got length %d, want %d", len(arr), len(want))
	}

	for i, v := range want {
		if arr[i] != v {
			t.Errorf("element %d: got %v, want %v", i, arr[i], v)
		}
	}
}

func TestFnReverse(t *testing.T) {
	result := eval(t, "$reverse([1, 2, 3])", nil)
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("got %T, want []interface{}", result)
	}

	want := []float64{3.0, 2.0, 1.0}
	if len(arr) != len(want) {
		t.Fatalf("got length %d, want %d", len(arr), len(want))
	}

	for i, v := range want {
		if arr[i] != v {
			t.Errorf("element %d: got %v, want %v", i, arr[i], v)
		}
	}
}

// --- Lambda and Apply Tests ---

func TestLambdaBasic(t *testing.T) {
	// Lambda basic usage with map
	result := eval(t, "$map([10, 20, 30], function($x) { $x / 10 })", nil)
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("got %T, want []interface{}", result)
	}

	want := []float64{1.0, 2.0, 3.0}
	if len(arr) != len(want) {
		t.Fatalf("got length %d, want %d", len(arr), len(want))
	}

	for i, v := range want {
		if arr[i] != v {
			t.Errorf("element %d: got %v, want %v", i, arr[i], v)
		}
	}
}

func TestApplyOperator(t *testing.T) {
	// Apply operator with lambda
	result := eval(t, "5 ~> function($x) { $x * 2 }", nil)
	if num, ok := result.(float64); ok {
		compareFloat(t, num, 10.0)
	} else {
		t.Errorf("got %T, want float64", result)
	}
}

func TestComplexLambda(t *testing.T) {
	data := map[string]interface{}{
		"numbers": []interface{}{1.0, 2.0, 3.0, 4.0, 5.0},
	}

	// Chain map and filter
	result := eval(t, "$filter($map(numbers, function($x) { $x * 2 }), function($x) { $x > 5 })", data)
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("got %T, want []interface{}", result)
	}

	// Should be [6, 8, 10]
	want := []float64{6.0, 8.0, 10.0}
	if len(arr) != len(want) {
		t.Fatalf("got length %d, want %d", len(arr), len(want))
	}

	for i, v := range want {
		if arr[i] != v {
			t.Errorf("element %d: got %v, want %v", i, arr[i], v)
		}
	}
}

// --- $distinct deep-equality tests ---

// TestFnDistinctDeepEquality verifies that $distinct uses structural deep equality,
// not string representation. This was the bug fixed on 2026-02-26.
func TestFnDistinctDeepEquality(t *testing.T) {
	t.Run("primitives regression", func(t *testing.T) {
		// Basic deduplication of primitives must still work.
		result := eval(t, `$distinct([1, 2, 1, 3, 2])`, nil)
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("got %T, want []interface{}", result)
		}
		if len(arr) != 3 {
			t.Errorf("got %d elements, want 3: %v", len(arr), arr)
		}
	})

	t.Run("strings regression", func(t *testing.T) {
		result := eval(t, `$distinct(["a", "b", "a", "c"])`, nil)
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("got %T, want []interface{}", result)
		}
		if len(arr) != 3 {
			t.Errorf("got %d elements, want 3: %v", len(arr), arr)
		}
	})

	t.Run("objects same content same key order", func(t *testing.T) {
		// Two objects with identical content → deduplicated to 1.
		result := eval(t, `$count($distinct([{"a":1,"b":2}, {"a":1,"b":2}]))`, nil)
		if result != 1.0 {
			t.Errorf("got %v, want 1 (objects with same content must be deduped)", result)
		}
	})

	t.Run("objects same content different key order", func(t *testing.T) {
		// The old fmt.Sprintf bug: {"a":1,"b":2} and {"b":2,"a":1}
		// had different string representations but are semantically equal.
		data := map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"a": 1.0, "b": 2.0},
				map[string]interface{}{"b": 2.0, "a": 1.0},
			},
		}
		result := eval(t, `$count($distinct(items))`, data)
		if result != 1.0 {
			t.Errorf("got %v, want 1 (objects with same keys/values but different insertion order must be deduped)", result)
		}
	})

	t.Run("objects different values not deduped", func(t *testing.T) {
		result := eval(t, `$count($distinct([{"a":1,"b":2}, {"a":1,"b":99}]))`, nil)
		if result != 2.0 {
			t.Errorf("got %v, want 2 (objects with different values must NOT be deduped)", result)
		}
	})

	t.Run("nested objects deduped correctly", func(t *testing.T) {
		result := eval(t, `$count($distinct([{"x":{"y":1}}, {"x":{"y":1}}, {"x":{"y":2}}]))`, nil)
		if result != 2.0 {
			t.Errorf("got %v, want 2 (nested equal objects deduped, different kept)", result)
		}
	})

	t.Run("null values", func(t *testing.T) {
		result := eval(t, `$count($distinct([null, null, 1]))`, nil)
		if result != 2.0 {
			t.Errorf("got %v, want 2 (null deduped)", result)
		}
	})

	t.Run("mixed types not deduped", func(t *testing.T) {
		// "1" (string) and 1 (number) are different values.
		result := eval(t, `$count($distinct(["1", 1]))`, nil)
		if result != 2.0 {
			t.Errorf("got %v, want 2 (string '1' and number 1 are distinct)", result)
		}
	})
}

// --- $match next() iterator tests ---

// TestFnMatchNextProperty verifies that each match object returned by $match
// contains a callable next() function that walks the match chain, as required
// by the JSONata spec. This was implemented on 2026-02-26.
func TestFnMatchNextProperty(t *testing.T) {
	t.Run("next exists on first match", func(t *testing.T) {
		result := eval(t, `$exists($match("hello world", /\w+/)[0].next)`, nil)
		if result != true {
			t.Errorf("got %v, want true (next must be present on first match)", result)
		}
	})

	t.Run("next exists on middle match", func(t *testing.T) {
		result := eval(t, `$exists($match("a b c", /\w+/)[1].next)`, nil)
		if result != true {
			t.Errorf("got %v, want true (next must be present on middle match)", result)
		}
	})

	t.Run("next on first match returns second match string", func(t *testing.T) {
		result := eval(t, `$match("hello world", /\w+/)[0].next().match`, nil)
		if result != "world" {
			t.Errorf("got %v, want 'world'", result)
		}
	})

	t.Run("next chain: first -> second -> third", func(t *testing.T) {
		result := eval(t, `$match("a b c", /\w+/)[0].next().next().match`, nil)
		if result != "c" {
			t.Errorf("got %v, want 'c'", result)
		}
	})

	t.Run("next on last match returns undefined", func(t *testing.T) {
		// next() on the last match must return undefined (nil in Go).
		result := eval(t, `$exists($match("a b", /\w+/)[1].next())`, nil)
		if result != false {
			t.Errorf("got %v, want false (next() on last match must return undefined)", result)
		}
	})

	t.Run("next preserves match and index fields", func(t *testing.T) {
		result := eval(t, `$match("foo bar", /\w+/)[0].next().index`, nil)
		if result != 4.0 {
			t.Errorf("got %v, want 4 (index of 'bar' in 'foo bar')", result)
		}
	})

	t.Run("next preserves groups", func(t *testing.T) {
		result := eval(t, `$match("a1 b2", /([a-z])(\d)/)[0].next().groups[0]`, nil)
		if result != "b" {
			t.Errorf("got %v, want 'b'", result)
		}
	})

	t.Run("single match has no next result", func(t *testing.T) {
		// Only one match: next returns undefined.
		result := eval(t, `$exists($match("hello", /\w+/)[0].next())`, nil)
		if result != false {
			t.Errorf("got %v, want false", result)
		}
	})
}
