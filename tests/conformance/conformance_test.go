package conformance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/parser"
)

// TestCase represents a conformance test case.
type TestCase struct {
	Name        string                 `json:"name"`
	Query       string                 `json:"query"`
	Data        interface{}            `json:"data"`
	Bindings    map[string]interface{} `json:"bindings,omitempty"`
	Expected    interface{}            `json:"expected,omitempty"`
	ShouldError bool                   `json:"shouldError,omitempty"`
}

// JSOutput represents the output from the JS runner.
type JSOutput struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result"`
	Error   *JSError    `json:"error"`
}

// JSError represents an error from JS execution.
type JSError struct {
	Message  string `json:"message"`
	Position int    `json:"position"`
	Token    string `json:"token"`
	Code     string `json:"code"`
}

// runJSJSONata executes a JSONata query using the JavaScript implementation.
func runJSJSONata(t *testing.T, query string, data interface{}, bindings map[string]interface{}) (interface{}, error) {
	t.Helper()

	// Prepare input
	input := map[string]interface{}{
		"query": query,
		"data":  data,
	}
	if bindings != nil {
		input["bindings"] = bindings
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	// Find runner.js path - try current directory first, then relative paths
	runnerPath := "runner.js"
	if _, err := os.Stat(runnerPath); os.IsNotExist(err) {
		// Try from one level up (if run from tests/)
		runnerPath = filepath.Join("conformance", "runner.js")
		if _, err := os.Stat(runnerPath); os.IsNotExist(err) {
			// Try from project root
			runnerPath = filepath.Join("tests", "conformance", "runner.js")
		}
	}

	// Execute JS runner
	cmd := exec.Command("node", runnerPath)
	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	// Parse output regardless of error (JS runner outputs JSON even on error)
	var output JSOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("failed to parse JS output: %w\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	if !output.Success {
		if output.Error != nil {
			return nil, fmt.Errorf("JS error: %s", output.Error.Message)
		}
		return nil, fmt.Errorf("JS execution failed")
	}

	return output.Result, nil
}

// runGoJSONata executes a JSONata query using the Go implementation.
func runGoJSONata(t *testing.T, query string, data interface{}, bindings map[string]interface{}) (interface{}, error) {
	t.Helper()

	// Parse
	expr, err := parser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Evaluate
	ev := evaluator.New()
	var result interface{}

	if bindings != nil {
		result, err = ev.EvalWithBindings(context.Background(), expr, data, bindings)
	} else {
		result, err = ev.Eval(context.Background(), expr, data)
	}

	if err != nil {
		return nil, fmt.Errorf("eval error: %w", err)
	}

	return result, nil
}

// compareResults compares two results, handling JSONata semantics.
func compareResults(t *testing.T, testName string, goResult, jsResult interface{}) bool {
	t.Helper()

	// Convert both to JSON and back for normalization
	goJSON, err := json.Marshal(goResult)
	if err != nil {
		t.Errorf("%s: failed to marshal Go result: %v", testName, err)
		return false
	}

	jsJSON, err := json.Marshal(jsResult)
	if err != nil {
		t.Errorf("%s: failed to marshal JS result: %v", testName, err)
		return false
	}

	// Compare JSON strings
	if string(goJSON) == string(jsJSON) {
		return true
	}

	// Try deep equal as fallback
	var goNorm, jsNorm interface{}
	json.Unmarshal(goJSON, &goNorm)
	json.Unmarshal(jsJSON, &jsNorm)

	if reflect.DeepEqual(goNorm, jsNorm) {
		return true
	}

	t.Errorf("%s: results differ\nGo:  %s\nJS:  %s", testName, string(goJSON), string(jsJSON))
	return false
}

// TestConformance runs conformance tests comparing Go and JS implementations.
func TestConformance(t *testing.T) {
	// Check if Node.js is available
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node.js not available, skipping conformance tests")
	}

	testCases := []TestCase{
		// Literals
		{Name: "string literal", Query: `"hello"`, Data: nil},
		{Name: "number literal", Query: "42", Data: nil},
		{Name: "boolean true", Query: "true", Data: nil},
		{Name: "boolean false", Query: "false", Data: nil},
		{Name: "null literal", Query: "null", Data: nil},

		// Variables and paths
		{Name: "context", Query: "$", Data: map[string]interface{}{"name": "John"}},
		{Name: "field access", Query: "name", Data: map[string]interface{}{"name": "Alice"}},
		{Name: "nested path", Query: "user.name", Data: map[string]interface{}{"user": map[string]interface{}{"name": "Bob"}}},
		{Name: "deep path", Query: "a.b.c", Data: map[string]interface{}{"a": map[string]interface{}{"b": map[string]interface{}{"c": 42.0}}}},

		// Arithmetic operators
		{Name: "addition", Query: "2 + 3", Data: nil},
		{Name: "subtraction", Query: "10 - 7", Data: nil},
		{Name: "multiplication", Query: "4 * 5", Data: nil},
		{Name: "division", Query: "20 / 4", Data: nil},
		{Name: "modulo", Query: "10 % 3", Data: nil},
		{Name: "negation", Query: "-5", Data: nil},
		{Name: "arithmetic precedence", Query: "2 + 3 * 4", Data: nil},

		// Comparison operators
		{Name: "equal true", Query: "5 = 5", Data: nil},
		{Name: "equal false", Query: "5 = 3", Data: nil},
		{Name: "not equal", Query: "5 != 3", Data: nil},
		{Name: "less than", Query: "3 < 5", Data: nil},
		{Name: "greater than", Query: "5 > 3", Data: nil},
		{Name: "less equal", Query: "5 <= 5", Data: nil},
		{Name: "greater equal", Query: "5 >= 5", Data: nil},

		// Logical operators
		{Name: "and true", Query: "true and true", Data: nil},
		{Name: "and false", Query: "true and false", Data: nil},
		{Name: "or true", Query: "false or true", Data: nil},
		{Name: "or false", Query: "false or false", Data: nil},

		// String operators
		{Name: "string concat", Query: `"hello" & " " & "world"`, Data: nil},
		{Name: "string concat with number", Query: `"value: " & 42`, Data: nil},

		// Arrays
		{Name: "empty array", Query: "[]", Data: nil},
		{Name: "array literal", Query: "[1, 2, 3]", Data: nil},
		{Name: "array with expressions", Query: "[1 + 1, 2 * 2, 3]", Data: nil},
		{Name: "array index", Query: "items[0]", Data: map[string]interface{}{"items": []interface{}{10.0, 20.0, 30.0}}},
		// TODO: Implement negative array indexing (items[-1] should return last element)
		// {Name: "array index negative", Query: "items[-1]", Data: map[string]interface{}{"items": []interface{}{10.0, 20.0, 30.0}}},

		// Array projection
		{Name: "array projection", Query: "items.name", Data: map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"name": "Item1"},
				map[string]interface{}{"name": "Item2"},
			},
		}},

		// Filter expressions
		{Name: "filter simple", Query: "$[$ > 2]", Data: []interface{}{1.0, 2.0, 3.0, 4.0, 5.0}},
		{Name: "filter with field", Query: "items[price > 100]", Data: map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"name": "A", "price": 50.0},
				map[string]interface{}{"name": "B", "price": 150.0},
				map[string]interface{}{"name": "C", "price": 200.0},
			},
		}},

		// Objects
		{Name: "empty object", Query: "{}", Data: nil},
		{Name: "object literal", Query: `{"name": "Alice", "age": 30}`, Data: nil},
		{Name: "object with expressions", Query: `{"sum": 2 + 3, "product": 4 * 5}`, Data: nil},

		// Conditionals
		{Name: "conditional true", Query: "true ? 'yes' : 'no'", Data: nil},
		{Name: "conditional false", Query: "false ? 'yes' : 'no'", Data: nil},
		{Name: "conditional with expression", Query: "5 > 3 ? 'greater' : 'lesser'", Data: nil},

		// Range operator is not supported in JSONata JS 2.1.0
		// GoSonata implements it as an extension
		// Uncomment these tests when comparing with JSONata implementations that support range
		// {Name: "range ascending", Query: "1..5", Data: nil},
		// {Name: "range descending", Query: "5..1", Data: nil},

		// Built-in functions - Aggregation
		{Name: "sum", Query: "$sum([1, 2, 3, 4, 5])", Data: nil},
		{Name: "count", Query: "$count([1, 2, 3])", Data: nil},
		{Name: "average", Query: "$average([10, 20, 30])", Data: nil},
		{Name: "min", Query: "$min([5, 2, 8, 1, 9])", Data: nil},
		{Name: "max", Query: "$max([5, 2, 8, 1, 9])", Data: nil},

		// Built-in functions - String
		{Name: "string", Query: "$string(42)", Data: nil},
		{Name: "length string", Query: `$length("hello")`, Data: nil},
		// $length with array is a GoSonata extension, not in JSONata JS 2.1.0
		// {Name: "length array", Query: "$length([1, 2, 3])", Data: nil},
		{Name: "substring", Query: `$substring("hello", 1, 3)`, Data: nil},
		{Name: "uppercase", Query: `$uppercase("hello")`, Data: nil},
		{Name: "lowercase", Query: `$lowercase("WORLD")`, Data: nil},
		{Name: "trim", Query: `$trim("  hello  ")`, Data: nil},
		{Name: "contains", Query: `$contains("hello world", "world")`, Data: nil},

		// Built-in functions - Type
		{Name: "type string", Query: `$type("hello")`, Data: nil},
		{Name: "type number", Query: "$type(42)", Data: nil},
		{Name: "type boolean", Query: "$type(true)", Data: nil},
		{Name: "type array", Query: "$type([1,2,3])", Data: nil},
		{Name: "type object", Query: `$type({"key": "value"})`, Data: nil},
		{Name: "type null", Query: "$type(null)", Data: nil},
		{Name: "exists true", Query: "$exists(name)", Data: map[string]interface{}{"name": "John"}},
		{Name: "exists false", Query: "$exists(missing)", Data: map[string]interface{}{"name": "John"}},

		// Built-in functions - Math
		{Name: "abs positive", Query: "$abs(5)", Data: nil},
		{Name: "abs negative", Query: "$abs(-5)", Data: nil},
		{Name: "floor", Query: "$floor(3.7)", Data: nil},
		{Name: "ceil", Query: "$ceil(3.2)", Data: nil},
		{Name: "round", Query: "$round(3.5)", Data: nil},
		{Name: "sqrt", Query: "$sqrt(16)", Data: nil},
		{Name: "power", Query: "$power(2, 3)", Data: nil},

		// Lambdas and higher-order functions
		{Name: "map with lambda", Query: "$map([1, 2, 3], function($x) { $x * 2 })", Data: nil},
		{Name: "filter with lambda", Query: "$filter([1, 2, 3, 4, 5], function($x) { $x > 2 })", Data: nil},
		{Name: "reduce with lambda", Query: "$reduce([1, 2, 3, 4], function($acc, $x) { $acc + $x }, 0)", Data: nil},

		// Apply operator
		{Name: "apply operator", Query: "5 ~> function($x) { $x * 2 }", Data: nil},

		// Complex expressions
		{Name: "complex filter and map", Query: "items[price > 75].name", Data: map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"name": "Item1", "price": 100.0},
				map[string]interface{}{"name": "Item2", "price": 50.0},
				map[string]interface{}{"name": "Item3", "price": 200.0},
			},
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Run JS version
			jsResult, jsErr := runJSJSONata(t, tc.Query, tc.Data, tc.Bindings)

			// Run Go version
			goResult, goErr := runGoJSONata(t, tc.Query, tc.Data, tc.Bindings)

			// Compare errors
			if (jsErr != nil) != (goErr != nil) {
				t.Errorf("Error mismatch:\nJS error: %v\nGo error: %v", jsErr, goErr)
				return
			}

			// If both errored, that's acceptable
			if jsErr != nil && goErr != nil {
				t.Logf("Both implementations errored (expected): JS=%v, Go=%v", jsErr, goErr)
				return
			}

			// Compare results
			if !compareResults(t, tc.Name, goResult, jsResult) {
				t.Logf("Query: %s", tc.Query)
				t.Logf("Data: %v", tc.Data)
			}
		})
	}
}

// TestConformanceWithExpected tests cases with known expected results.
func TestConformanceWithExpected(t *testing.T) {
	testCases := []TestCase{
		{Name: "simple addition", Query: "2 + 3", Data: nil, Expected: 5.0},
		{Name: "string concat", Query: `"hello" & " world"`, Data: nil, Expected: "hello world"},
		{Name: "array length", Query: "$length([1,2,3])", Data: nil, Expected: 3.0},
		{Name: "field access", Query: "name", Data: map[string]interface{}{"name": "Alice"}, Expected: "Alice"},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			goResult, goErr := runGoJSONata(t, tc.Query, tc.Data, tc.Bindings)
			if goErr != nil {
				t.Fatalf("Go error: %v", goErr)
			}

			if !reflect.DeepEqual(goResult, tc.Expected) {
				t.Errorf("Result mismatch\nGot:      %v (%T)\nExpected: %v (%T)", goResult, goResult, tc.Expected, tc.Expected)
			}
		})
	}
}
