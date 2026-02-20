// complex_test.go — Additional conformance tests for complex JSONata scenarios.
// These tests cover advanced features documented at https://docs.jsonata.org/next/.
// All expected values are derived directly from the official JSONata documentation
// and verified against the JavaScript reference implementation.

package conformance_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/parser"
)

// PersonData is the standard JSONata documentation example from
// https://docs.jsonata.org/next/simple and path/predicate docs.
var PersonData = map[string]interface{}{
	"FirstName": "Fred",
	"Surname":   "Smith",
	"Age":       28.0,
	"Address": map[string]interface{}{
		"Street":   "Hursley Park",
		"City":     "Winchester",
		"Postcode": "SO21 2JN",
	},
	"Phone": []interface{}{
		map[string]interface{}{"type": "home", "number": "0203 544 1234"},
		map[string]interface{}{"type": "office", "number": "01962 001234"},
		map[string]interface{}{"type": "office", "number": "01962 001235"},
		map[string]interface{}{"type": "mobile", "number": "077 7700 1234"},
	},
	"Email": []interface{}{
		map[string]interface{}{"type": "work", "address": []interface{}{"fred.smith@my-work.com", "fsmith@my-work.com"}},
		map[string]interface{}{"type": "home", "address": []interface{}{"freddy@my-social.com", "frederic.smith@very-serious.com"}},
	},
	"Other": map[string]interface{}{
		"Over 18 ?":   true,
		"Miscellaneous": nil,
		"Alternative.Address": map[string]interface{}{
			"Street":   "Brick Lane",
			"City":     "London",
			"Postcode": "E1 6RF",
		},
	},
}

// AccountData is the standard dataset5 from the official JSONata test suite.
var AccountData = map[string]interface{}{
	"Account": map[string]interface{}{
		"Account Name": "Firefly",
		"Order": []interface{}{
			map[string]interface{}{
				"OrderID": "order103",
				"Product": []interface{}{
					map[string]interface{}{
						"Product Name": "Bowler Hat",
						"ProductID":    858383.0,
						"SKU":          "0406654608",
						"Description":  map[string]interface{}{"Colour": "Purple", "Width": 300.0, "Height": 200.0, "Depth": 210.0, "Weight": 0.75},
						"Price":        34.45,
						"Quantity":     2.0,
					},
					map[string]interface{}{
						"Product Name": "Trilby hat",
						"ProductID":    858236.0,
						"SKU":          "0406634348",
						"Description":  map[string]interface{}{"Colour": "Orange", "Width": 300.0, "Height": 200.0, "Depth": 210.0, "Weight": 0.6},
						"Price":        21.67,
						"Quantity":     1.0,
					},
				},
			},
			map[string]interface{}{
				"OrderID": "order104",
				"Product": []interface{}{
					map[string]interface{}{
						"Product Name": "Bowler Hat",
						"ProductID":    858383.0,
						"SKU":          "040657863",
						"Description":  map[string]interface{}{"Colour": "Purple", "Width": 300.0, "Height": 200.0, "Depth": 210.0, "Weight": 0.75},
						"Price":        34.45,
						"Quantity":     4.0,
					},
					map[string]interface{}{
						"Product Name": "Cloak",
						"ProductID":    345664.0,
						"SKU":          "0406654603",
						"Description":  map[string]interface{}{"Colour": "Black", "Width": 30.0, "Height": 20.0, "Depth": 210.0, "Weight": 2.0},
						"Price":        107.99,
						"Quantity":     1.0,
					},
				},
			},
		},
	},
}

// evalComplex is a helper for evaluating a JSONata expression against data.
func evalComplex(t *testing.T, query string, data interface{}) (interface{}, error) {
	t.Helper()
	expr, err := parser.Parse(query)
	if err != nil {
		return nil, err
	}
	ev := evaluator.New()
	return ev.Eval(context.Background(), expr, data)
}

// evalComplexWithBindings evaluates a query with additional variable bindings.
func evalComplexWithBindings(t *testing.T, query string, data interface{}, bindings map[string]interface{}) (interface{}, error) {
	t.Helper()
	expr, err := parser.Parse(query)
	if err != nil {
		return nil, err
	}
	ev := evaluator.New()
	return ev.EvalWithBindings(context.Background(), expr, data, bindings)
}

// toMap converts either map[string]interface{} or *evaluator.OrderedObject
// to a plain map for easy key access in tests.
func toMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	if oo, ok := v.(*evaluator.OrderedObject); ok {
		m := make(map[string]interface{}, len(oo.Keys))
		for _, k := range oo.Keys {
			val, _ := oo.Get(k)
			m[k] = val
		}
		return m
	}
	return nil
}

// ---------------------------------------------------------------------------
// TestComplexPathOperators covers wildcard, descendant wildcard, and predicate usage
// as documented in https://docs.jsonata.org/next/path-operators and
// https://docs.jsonata.org/next/predicate
// ---------------------------------------------------------------------------

func TestComplexPathOperators(t *testing.T) {
	t.Run("wildcard_address_values", func(t *testing.T) {
		// Address.* returns all values of Address fields
		// Documented in predicate.md: "Select the values of all the fields of Address"
		result, err := evalComplex(t, "Address.*", PersonData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T: %v", result, result)
		}
		// Address has 3 fields (Street, City, Postcode) → 3 values
		if len(arr) != 3 {
			t.Errorf("expected 3 values, got %d: %v", len(arr), arr)
		}
	})

	t.Run("filter_mobile_phone", func(t *testing.T) {
		// Phone[type='mobile'] — filter for mobile phone
		// Expected single result (not array): "077 7700 1234"
		result, err := evalComplex(t, "Phone[type='mobile'].number", PersonData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "077 7700 1234" {
			t.Errorf("expected '077 7700 1234', got %v", result)
		}
	})

	t.Run("filter_office_phones_multiple", func(t *testing.T) {
		// Phone[type='office'].number — two office numbers
		// Expected: ["01962 001234", "01962 001235"]
		result, err := evalComplex(t, "Phone[type='office'].number", PersonData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []interface{}{"01962 001234", "01962 001235"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("force_array_single_result", func(t *testing.T) {
		// Phone[type='mobile'][].number — force array even for single result
		// Expected: ["077 7700 1234"]
		result, err := evalComplex(t, "Phone[type='mobile'][].number", PersonData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T: %v", result, result)
		}
		if len(arr) != 1 || arr[0] != "077 7700 1234" {
			t.Errorf("expected [\"077 7700 1234\"], got %v", arr)
		}
	})

	t.Run("index_first_element", func(t *testing.T) {
		// Phone[0].number — first phone number
		result, err := evalComplex(t, "Phone[0].number", PersonData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "0203 544 1234" {
			t.Errorf("expected '0203 544 1234', got %v", result)
		}
	})

	t.Run("descendant_wildcard_postcode", func(t *testing.T) {
		// **.Postcode — select all Postcode values at any depth
		// Expected: ["SO21 2JN", "E1 6RF"]
		result, err := evalComplex(t, "**.Postcode", PersonData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T: %v", result, result)
		}
		if len(arr) != 2 {
			t.Errorf("expected 2 postcodes, got %d: %v", len(arr), arr)
		}
	})

	t.Run("path_product_names", func(t *testing.T) {
		// Account.Order.Product."Product Name" — all product names (flattened)
		result, err := evalComplex(t, `Account.Order.Product."Product Name"`, AccountData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []interface{}{"Bowler Hat", "Trilby hat", "Bowler Hat", "Cloak"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("filter_order_by_id", func(t *testing.T) {
		// Account.Order[OrderID="order103"].Product."Product Name"
		result, err := evalComplex(t, `Account.Order[OrderID="order103"].Product."Product Name"`, AccountData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []interface{}{"Bowler Hat", "Trilby hat"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})
}

// ---------------------------------------------------------------------------
// TestComplexGroupingAndAggregation covers grouping and aggregation patterns
// as documented in https://docs.jsonata.org/next/sorting-grouping
// ---------------------------------------------------------------------------

func TestComplexGroupingAndAggregation(t *testing.T) {
	t.Run("sum_all_prices", func(t *testing.T) {
		// $sum(Account.Order.Product.Price) — total of all product prices
		// 34.45 + 21.67 + 34.45 + 107.99 = 198.56
		result, err := evalComplex(t, "$sum(Account.Order.Product.Price)", AccountData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := 198.56
		if v, ok := result.(float64); !ok || v != expected {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("sum_price_times_quantity", func(t *testing.T) {
		// $sum(Account.Order.Product.(Price*Quantity)) — total revenue
		// 34.45*2 + 21.67*1 + 34.45*4 + 107.99*1 = 68.9 + 21.67 + 137.8 + 107.99 = 336.36
		result, err := evalComplex(t, "$sum(Account.Order.Product.(Price*Quantity))", AccountData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		v, ok := result.(float64)
		if !ok {
			t.Fatalf("expected float64, got %T: %v", result, result)
		}
		// Use approximate comparison to handle floating point
		if v < 336.35 || v > 336.37 {
			t.Errorf("expected ~336.36, got %v", v)
		}
	})

	t.Run("order_totals_object_constructor", func(t *testing.T) {
		// Account.Order.{OrderID: $sum(Product.(Price*Quantity))}
		// order103: 34.45*2 + 21.67*1 = 90.57
		// order104: 34.45*4 + 107.99*1 = 245.79
		result, err := evalComplex(t, "Account.Order.{OrderID: $sum(Product.(Price*Quantity))}", AccountData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T: %v", result, result)
		}
		if len(arr) != 2 {
			t.Fatalf("expected 2 orders, got %d", len(arr))
		}
		obj0 := toMap(arr[0])
		// Check order103 total
		if v103, ok := obj0["order103"].(float64); !ok || (v103 < 90.56 || v103 > 90.58) {
			t.Errorf("order103 total: expected ~90.57, got %v", obj0["order103"])
		}
		obj1 := toMap(arr[1])
		// Check order104 total
		if v104, ok := obj1["order104"].(float64); !ok || (v104 < 245.78 || v104 > 245.80) {
			t.Errorf("order104 total: expected ~245.79, got %v", obj1["order104"])
		}
	})

	t.Run("infix_grouping_by_product_name", func(t *testing.T) {
		// Account.Order.Product{"Product Name": $sum(Price)}
		// Total price per product name (merged):
		// "Bowler Hat": 34.45+34.45=68.9, "Trilby hat": 21.67, "Cloak": 107.99
		result, err := evalComplex(t, "Account.Order.Product{`Product Name`: $sum(Price)}", AccountData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		obj := toMap(result)
		if v, ok := obj["Bowler Hat"].(float64); !ok || v != 68.9 {
			t.Errorf("Bowler Hat total: expected 68.9, got %v", obj["Bowler Hat"])
		}
		if v, ok := obj["Trilby hat"].(float64); !ok || v != 21.67 {
			t.Errorf("Trilby hat total: expected 21.67, got %v", obj["Trilby hat"])
		}
		if v, ok := obj["Cloak"].(float64); !ok || v != 107.99 {
			t.Errorf("Cloak total: expected 107.99, got %v", obj["Cloak"])
		}
	})

	t.Run("count_all_products", func(t *testing.T) {
		// $count(Account.Order.Product) — total product items across all orders = 4
		result, err := evalComplex(t, "$count(Account.Order.Product)", AccountData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 4.0 {
			t.Errorf("expected 4, got %v", result)
		}
	})

	t.Run("average_product_price", func(t *testing.T) {
		// $average(Account.Order.Product.Price) = 198.56/4 = 49.64
		result, err := evalComplex(t, "$average(Account.Order.Product.Price)", AccountData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		v, ok := result.(float64)
		if !ok {
			t.Fatalf("expected float64, got %T: %v", result, result)
		}
		if v < 49.63 || v > 49.65 {
			t.Errorf("expected ~49.64, got %v", v)
		}
	})
}

// ---------------------------------------------------------------------------
// TestComplexHigherOrderFunctions covers $map, $filter, $reduce, $sift, $each, $single
// as documented in https://docs.jsonata.org/next/higher-order-functions
// ---------------------------------------------------------------------------

func TestComplexHigherOrderFunctions(t *testing.T) {
	t.Run("map_string_range", func(t *testing.T) {
		// $map([1..5], $string) => ["1","2","3","4","5"]
		result, err := evalComplex(t, "$map([1..5], $string)", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []interface{}{"1", "2", "3", "4", "5"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("map_with_index", func(t *testing.T) {
		// $map([10,20,30], function($v, $i) { $i & ": " & $string($v) })
		// => ["0: 10", "1: 20", "2: 30"]
		result, err := evalComplex(t, `$map([10,20,30], function($v, $i) { $string($i) & ": " & $string($v) })`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []interface{}{"0: 10", "1: 20", "2: 30"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("filter_greater_than", func(t *testing.T) {
		// $filter([1,2,3,4,5], function($v) { $v > 2 }) => [3,4,5]
		result, err := evalComplex(t, "$filter([1,2,3,4,5], function($v) { $v > 2 })", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []interface{}{3.0, 4.0, 5.0}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("reduce_product_range", func(t *testing.T) {
		// ($product := function($i, $j){$i * $j}; $reduce([1..5], $product)) => 120
		result, err := evalComplex(t, "($product := function($i, $j){$i * $j}; $reduce([1..5], $product))", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 120.0 {
			t.Errorf("expected 120, got %v", result)
		}
	})

	t.Run("reduce_sum_with_init", func(t *testing.T) {
		// $reduce([1,2,3,4], function($acc, $v) { $acc + $v }, 0) => 10
		result, err := evalComplex(t, "$reduce([1,2,3,4], function($acc, $v) { $acc + $v }, 0)", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10.0 {
			t.Errorf("expected 10, got %v", result)
		}
	})

	t.Run("sift_filter_object_keys", func(t *testing.T) {
		// $sift({a:1,b:2,c:3}, function($v) { $v > 1 }) => {b:2,c:3}
		result, err := evalComplex(t, `$sift({"a":1,"b":2,"c":3}, function($v) { $v > 1 })`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		obj := toMap(result)
		if _, hasA := obj["a"]; hasA {
			t.Errorf("key 'a' should have been filtered out")
		}
		if obj["b"] != 2.0 {
			t.Errorf("expected b=2, got %v", obj["b"])
		}
		if obj["c"] != 3.0 {
			t.Errorf("expected c=3, got %v", obj["c"])
		}
	})

	t.Run("each_kv_pairs", func(t *testing.T) {
		// $each({a:1,b:2}, function($v,$k) { $k & "=" & $string($v) })
		// => ["a=1","b=2"] (order may vary, check length and values)
		result, err := evalComplex(t, `$each({"a":1,"b":2}, function($v,$k) { $k & "=" & $string($v) })`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T: %v", result, result)
		}
		if len(arr) != 2 {
			t.Errorf("expected 2 elements, got %d: %v", len(arr), arr)
		}
	})

	t.Run("single_matching_item", func(t *testing.T) {
		// $single([1,2,3,4,5], function($v) { $v = 3 }) => 3
		result, err := evalComplex(t, "$single([1,2,3,4,5], function($v) { $v = 3 })", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 3.0 {
			t.Errorf("expected 3, got %v", result)
		}
	})

	t.Run("single_no_match_error", func(t *testing.T) {
		// $single([1,2,3], function($v) { $v > 10 }) => error (no match)
		_, err := evalComplex(t, "$single([1,2,3], function($v) { $v > 10 })", nil)
		if err == nil {
			t.Error("expected error for $single with no match, got nil")
		}
	})

	t.Run("filter_above_average_price", func(t *testing.T) {
		// $filter(Account.Order.Product, function($v,$i,$a) { $v.Price > $average($a.Price) })
		// Average price = 49.64; only Cloak (107.99) is above average
		result, err := evalComplex(t,
			"$filter(Account.Order.Product, function($v,$i,$a) { $v.Price > $average($a.Price) })",
			AccountData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var products []interface{}
		switch v := result.(type) {
		case []interface{}:
			products = v
		case map[string]interface{}:
			products = []interface{}{v}
		default:
			t.Fatalf("unexpected result type %T: %v", result, result)
		}
		if len(products) != 1 {
			t.Errorf("expected 1 product above average, got %d", len(products))
		} else if p := toMap(products[0]); p["Product Name"] != "Cloak" {
			t.Errorf("expected Cloak, got %v", p["Product Name"])
		}
	})
}

// ---------------------------------------------------------------------------
// TestComplexProgramming covers lambda definitions, closures, recursion, and apply
// as documented in https://docs.jsonata.org/next/programming
// ---------------------------------------------------------------------------

func TestComplexProgramming(t *testing.T) {
	t.Run("lambda_double", func(t *testing.T) {
		// ($double := function($x){$x*2}; $double(5)) => 10
		result, err := evalComplex(t, "($double := function($x){$x*2}; $double(5))", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10.0 {
			t.Errorf("expected 10, got %v", result)
		}
	})

	t.Run("lambda_map", func(t *testing.T) {
		// ($double := function($x){$x*2}; $map([1,2,3], $double)) => [2,4,6]
		result, err := evalComplex(t, "($double := function($x){$x*2}; $map([1,2,3], $double))", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []interface{}{2.0, 4.0, 6.0}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("recursive_factorial", func(t *testing.T) {
		// ($factorial := function($n) { $n <= 1 ? 1 : $n * $factorial($n-1) }; $factorial(5))
		// => 120
		result, err := evalComplex(t,
			"($factorial := function($n){ $n <= 1 ? 1 : $n * $factorial($n-1) }; $factorial(5))",
			nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 120.0 {
			t.Errorf("expected 120, got %v", result)
		}
	})

	t.Run("apply_operator_single", func(t *testing.T) {
		// 5 ~> function($x){ $x * 2 } => 10
		result, err := evalComplex(t, "5 ~> function($x){ $x * 2 }", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10.0 {
			t.Errorf("expected 10, got %v", result)
		}
	})

	t.Run("apply_operator_chain", func(t *testing.T) {
		// [1,2,3] ~> $sum() => 6
		result, err := evalComplex(t, "[1,2,3] ~> $sum()", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 6.0 {
			t.Errorf("expected 6, got %v", result)
		}
	})

	t.Run("apply_operator_pipeline", func(t *testing.T) {
		// [3,1,2] ~> $sort() ~> $reverse() => [3,2,1]
		result, err := evalComplex(t, "[3,1,2] ~> $sort() ~> $reverse()", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []interface{}{3.0, 2.0, 1.0}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("conditional_ternary", func(t *testing.T) {
		// 5 > 3 ? "yes" : "no" => "yes"
		result, err := evalComplex(t, `5 > 3 ? "yes" : "no"`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "yes" {
			t.Errorf("expected 'yes', got %v", result)
		}
	})

	t.Run("block_expression", func(t *testing.T) {
		// ( $x := 5; $y := $x * 2; $x + $y ) => 15
		result, err := evalComplex(t, "( $x := 5; $y := $x * 2; $x + $y )", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 15.0 {
			t.Errorf("expected 15, got %v", result)
		}
	})

	t.Run("closure_counter", func(t *testing.T) {
		// ($add := function($x) { function($y) { $x + $y } }; $add5 := $add(5); $add5(3)) => 8
		// Closure: function returns a function that remembers state via argument passing
		result, err := evalComplex(t,
			"($add := function($x){ function($y){ $x + $y } }; $add5 := $add(5); $add5(3))",
			nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 8.0 {
			t.Errorf("expected 8, got %v", result)
		}
	})
}

// ---------------------------------------------------------------------------
// TestComplexObjectFunctions covers $keys, $lookup, $spread, $merge
// as documented in https://docs.jsonata.org/next/object-functions
// ---------------------------------------------------------------------------

func TestComplexObjectFunctions(t *testing.T) {
	t.Run("keys_simple_object", func(t *testing.T) {
		// $keys({"a":1,"b":2,"c":3}) => ["a","b","c"]
		result, err := evalComplex(t, `$keys({"a":1,"b":2,"c":3})`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T: %v", result, result)
		}
		if len(arr) != 3 {
			t.Errorf("expected 3 keys, got %d: %v", len(arr), arr)
		}
	})

	t.Run("keys_dedup_from_array", func(t *testing.T) {
		// $keys([{"a":1,"b":2},{"b":3,"c":4}]) => de-duplicated keys ["a","b","c"]
		result, err := evalComplex(t, `$keys([{"a":1,"b":2},{"b":3,"c":4}])`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T: %v", result, result)
		}
		if len(arr) != 3 {
			t.Errorf("expected 3 unique keys, got %d: %v", len(arr), arr)
		}
	})

	t.Run("lookup_by_key", func(t *testing.T) {
		// $lookup({"a":1,"b":2}, "b") => 2
		result, err := evalComplex(t, `$lookup({"a":1,"b":2}, "b")`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 2.0 {
			t.Errorf("expected 2, got %v", result)
		}
	})

	t.Run("lookup_missing_key", func(t *testing.T) {
		// $lookup({"a":1}, "z") => undefined (nil)
		result, err := evalComplex(t, `$lookup({"a":1}, "z")`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil for missing key, got %v", result)
		}
	})

	t.Run("spread_object", func(t *testing.T) {
		// $spread({"a":1,"b":2}) => [{"a":1},{"b":2}]
		result, err := evalComplex(t, `$spread({"a":1,"b":2})`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T: %v", result, result)
		}
		if len(arr) != 2 {
			t.Errorf("expected 2 elements, got %d", len(arr))
		}
	})

	t.Run("merge_objects", func(t *testing.T) {
		// $merge([{"a":1},{"b":2},{"c":3}]) => {"a":1,"b":2,"c":3}
		result, err := evalComplex(t, `$merge([{"a":1},{"b":2},{"c":3}])`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		obj := toMap(result)
		if obj["a"] != 1.0 || obj["b"] != 2.0 || obj["c"] != 3.0 {
			t.Errorf("expected merged object, got %v", obj)
		}
	})

	t.Run("merge_override_last_wins", func(t *testing.T) {
		// $merge([{"a":1,"b":99},{"b":2}]) => {"a":1,"b":2} (last wins)
		result, err := evalComplex(t, `$merge([{"a":1,"b":99},{"b":2}])`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		obj := toMap(result)
		if obj["b"] != 2.0 {
			t.Errorf("expected b=2 (last wins), got %v", obj["b"])
		}
	})
}

// ---------------------------------------------------------------------------
// TestComplexNumericFunctions covers $round, $floor, $ceil, $power, $sqrt, $formatNumber
// as documented in https://docs.jsonata.org/next/numeric-functions
// ---------------------------------------------------------------------------

func TestComplexNumericFunctions(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected interface{}
	}{
		{"round_default", "$round(123.456)", 123.0},
		{"round_precision_2", "$round(123.456, 2)", 123.46},
		{"round_precision_neg1", "$round(123.456, -1)", 120.0},
		{"round_precision_neg2", "$round(123.456, -2)", 100.0},
		{"round_half_even_11.5", "$round(11.5)", 12.0},
		{"round_half_even_12.5", "$round(12.5)", 12.0}, // rounds to even
		{"floor_negative", "$floor(-5.3)", -6.0},
		{"ceil_negative", "$ceil(-5.3)", -5.0},
		{"power_base2_exp8", "$power(2, 8)", 256.0},
		{"power_negative_exp", "$power(2, -2)", 0.25},
		{"sqrt_4", "$sqrt(4)", 2.0},
		{"number_from_hex", `$number("0x12")`, 18.0},
		{"number_from_string", `$number("5")`, 5.0},
		{"abs_negative", "$abs(-42)", 42.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evalComplex(t, tc.query, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestComplexStringFunctions covers $split, $join, $replace, $match, $pad, $trim
// as documented in https://docs.jsonata.org/next/string-functions
// ---------------------------------------------------------------------------

func TestComplexStringFunctions(t *testing.T) {
	t.Run("split_by_space", func(t *testing.T) {
		// $split("hello world foo", " ") => ["hello","world","foo"]
		result, err := evalComplex(t, `$split("hello world foo", " ")`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []interface{}{"hello", "world", "foo"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("split_with_limit", func(t *testing.T) {
		// $split("hello world foo", " ", 2) => ["hello","world"]
		result, err := evalComplex(t, `$split("hello world foo", " ", 2)`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []interface{}{"hello", "world"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("join_with_separator", func(t *testing.T) {
		// $join(["a","b","c"], "-") => "a-b-c"
		result, err := evalComplex(t, `$join(["a","b","c"], "-")`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "a-b-c" {
			t.Errorf("expected 'a-b-c', got %v", result)
		}
	})

	t.Run("join_no_separator", func(t *testing.T) {
		// $join(["a","b","c"]) => "abc"
		result, err := evalComplex(t, `$join(["a","b","c"])`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "abc" {
			t.Errorf("expected 'abc', got %v", result)
		}
	})

	t.Run("replace_string", func(t *testing.T) {
		// $replace("hello world", "world", "there") => "hello there"
		result, err := evalComplex(t, `$replace("hello world", "world", "there")`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "hello there" {
			t.Errorf("expected 'hello there', got %v", result)
		}
	})

	t.Run("replace_with_regex", func(t *testing.T) {
		// $replace("John Smith and Fred Smith", /Smith/, "Jones") => "John Jones and Fred Jones"
		result, err := evalComplex(t, `$replace("John Smith and Fred Smith", /Smith/, "Jones")`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "John Jones and Fred Jones" {
			t.Errorf("expected 'John Jones and Fred Jones', got %v", result)
		}
	})

	t.Run("contains_string", func(t *testing.T) {
		// $contains("hello world", "world") => true
		result, err := evalComplex(t, `$contains("hello world", "world")`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != true {
			t.Errorf("expected true, got %v", result)
		}
	})

	t.Run("contains_regex", func(t *testing.T) {
		// $contains("hello world", /wor.d/) => true
		result, err := evalComplex(t, `$contains("hello world", /wor.d/)`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != true {
			t.Errorf("expected true, got %v", result)
		}
	})

	t.Run("pad_left", func(t *testing.T) {
		// $pad("abc", 5) => "abc  " (right pad with spaces to width 5)
		result, err := evalComplex(t, `$pad("abc", 5)`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "abc  " {
			t.Errorf("expected 'abc  ', got '%v'", result)
		}
	})

	t.Run("pad_right", func(t *testing.T) {
		// $pad("abc", -5) => "  abc" (left pad to width 5)
		result, err := evalComplex(t, `$pad("abc", -5)`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "  abc" {
			t.Errorf("expected '  abc', got '%v'", result)
		}
	})

	t.Run("substrng_before", func(t *testing.T) {
		// $substringBefore("hello world", " ") => "hello"
		result, err := evalComplex(t, `$substringBefore("hello world", " ")`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "hello" {
			t.Errorf("expected 'hello', got %v", result)
		}
	})

	t.Run("substring_after", func(t *testing.T) {
		// $substringAfter("hello world", " ") => "world"
		result, err := evalComplex(t, `$substringAfter("hello world", " ")`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "world" {
			t.Errorf("expected 'world', got %v", result)
		}
	})

	t.Run("string_map_over_numbers", func(t *testing.T) {
		// ["1","2","3","4","5"].$number() => [1,2,3,4,5]
		result, err := evalComplex(t, `["1","2","3","4","5"].$number()`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []interface{}{1.0, 2.0, 3.0, 4.0, 5.0}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})
}

// ---------------------------------------------------------------------------
// TestComplexSortingOrderBy covers order-by operator and $sort
// as documented in https://docs.jsonata.org/next/sorting-grouping
// ---------------------------------------------------------------------------

func TestComplexSortingOrderBy(t *testing.T) {
	t.Run("sort_numbers_ascending", func(t *testing.T) {
		// $sort([3,1,2]) => [1,2,3]
		result, err := evalComplex(t, "$sort([3,1,2])", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []interface{}{1.0, 2.0, 3.0}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("sort_strings", func(t *testing.T) {
		// $sort(["banana","apple","cherry"]) => ["apple","banana","cherry"]
		result, err := evalComplex(t, `$sort(["banana","apple","cherry"])`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []interface{}{"apple", "banana", "cherry"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("sort_with_comparator", func(t *testing.T) {
		// $sort([{n:3},{n:1},{n:2}], function($l,$r){$l.n > $r.n}) => [{n:1},{n:2},{n:3}]
		result, err := evalComplex(t,
			`$sort([{"n":3},{"n":1},{"n":2}], function($l,$r){$l.n > $r.n})`,
			nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T: %v", result, result)
		}
		if len(arr) != 3 {
			t.Fatalf("expected 3 elements, got %d", len(arr))
		}
		// Verify ascending order by n
		for i, item := range arr {
			obj := toMap(item)
			if obj["n"] != float64(i+1) {
				t.Errorf("position %d: expected n=%d, got %v", i, i+1, obj["n"])
			}
		}
	})

	t.Run("orderby_ascending", func(t *testing.T) {
		// Account.Order.Product^(Price)."Product Name"
		// Ascending by price: Trilby hat(21.67), Bowler Hat(34.45), Bowler Hat(34.45), Cloak(107.99)
		result, err := evalComplex(t, `Account.Order.Product^(Price)."Product Name"`, AccountData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T: %v", result, result)
		}
		if len(arr) != 4 {
			t.Fatalf("expected 4 products, got %d", len(arr))
		}
		if arr[0] != "Trilby hat" {
			t.Errorf("first should be cheapest (Trilby hat), got %v", arr[0])
		}
		if arr[3] != "Cloak" {
			t.Errorf("last should be most expensive (Cloak), got %v", arr[3])
		}
	})

	t.Run("orderby_descending", func(t *testing.T) {
		// Account.Order.Product^(>Price)."Product Name" — descending
		result, err := evalComplex(t, `Account.Order.Product^(>Price)."Product Name"`, AccountData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T: %v", result, result)
		}
		if len(arr) != 4 {
			t.Fatalf("expected 4 products, got %d", len(arr))
		}
		if arr[0] != "Cloak" {
			t.Errorf("first should be most expensive (Cloak), got %v", arr[0])
		}
		if arr[3] != "Trilby hat" {
			t.Errorf("last should be cheapest (Trilby hat), got %v", arr[3])
		}
	})
}

// ---------------------------------------------------------------------------
// TestComplexConstruction covers complex object/array construction patterns
// as documented in https://docs.jsonata.org/next/construction
// ---------------------------------------------------------------------------

func TestComplexConstruction(t *testing.T) {
	t.Run("object_from_path", func(t *testing.T) {
		// Account.Order.{"id": OrderID, "total": $sum(Product.Price)}
		result, err := evalComplex(t,
			`Account.Order.{"id": OrderID, "total": $sum(Product.Price)}`,
			AccountData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T: %v", result, result)
		}
		if len(arr) != 2 {
			t.Fatalf("expected 2 orders, got %d", len(arr))
		}
		obj0 := toMap(arr[0])
		if obj0["id"] != "order103" {
			t.Errorf("expected id=order103, got %v", obj0["id"])
		}
		// order103: 34.45 + 21.67 = 56.12
		if v, ok := obj0["total"].(float64); !ok || v < 56.11 || v > 56.13 {
			t.Errorf("expected total~56.12 for order103, got %v", obj0["total"])
		}
	})

	t.Run("array_constructor", func(t *testing.T) {
		// [ Account."Account Name", $count(Account.Order), $sum(Account.Order.Product.Price) ]
		result, err := evalComplex(t,
			`[ Account."Account Name", $count(Account.Order), $sum(Account.Order.Product.Price) ]`,
			AccountData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]interface{})
		if !ok {
			t.Fatalf("expected array, got %T: %v", result, result)
		}
		if len(arr) != 3 {
			t.Fatalf("expected 3 elements, got %d", len(arr))
		}
		if arr[0] != "Firefly" {
			t.Errorf("expected 'Firefly', got %v", arr[0])
		}
		if arr[1] != 2.0 {
			t.Errorf("expected 2 orders, got %v", arr[1])
		}
	})

	t.Run("nested_object_construction", func(t *testing.T) {
		// { "name": FirstName & " " & Surname, "age": Age }
		result, err := evalComplex(t,
			`{ "name": FirstName & " " & Surname, "age": Age }`,
			PersonData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		obj := toMap(result)
		if obj["name"] != "Fred Smith" {
			t.Errorf("expected 'Fred Smith', got %v", obj["name"])
		}
		if obj["age"] != 28.0 {
			t.Errorf("expected 28, got %v", obj["age"])
		}
	})
}

// ---------------------------------------------------------------------------
// TestComplexErrorHandling covers error functions and assertions
// as documented in https://docs.jsonata.org/next/object-functions
// ---------------------------------------------------------------------------

func TestComplexErrorHandling(t *testing.T) {
	t.Run("assert_true_no_error", func(t *testing.T) {
		// $assert(true, "should not fail") => undefined (nil)
		result, err := evalComplex(t, `$assert(true, "should not fail")`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("$assert(true) should return nil, got %v", result)
		}
	})

	t.Run("assert_false_raises_error", func(t *testing.T) {
		// $assert(false, "assertion failed") => error
		_, err := evalComplex(t, `$assert(false, "assertion failed")`, nil)
		if err == nil {
			t.Error("expected error from $assert(false), got nil")
		}
	})

	t.Run("error_in_else_branch", func(t *testing.T) {
		// 50 > 100 ? 50 : $error("too cheap") — false branch triggers
		_, err := evalComplex(t, `50 > 100 ? 50 : $error("too cheap")`, nil)
		if err == nil {
			t.Error("expected $error to trigger, got nil")
		}
	})

	t.Run("error_in_true_branch_not_triggered", func(t *testing.T) {
		// 50 > 10 ? 50 : $error("too cheap") — true branch, no error
		result, err := evalComplex(t, `50 > 10 ? 50 : $error("too cheap")`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 50.0 {
			t.Errorf("expected 50, got %v", result)
		}
	})
}

// ---------------------------------------------------------------------------
// TestComplexBooleanAndComparison covers boolean operators and comparison
// as documented in https://docs.jsonata.org/next/boolean-operators
// ---------------------------------------------------------------------------

func TestComplexBooleanAndComparison(t *testing.T) {
	t.Run("in_operator", func(t *testing.T) {
		// "world" in ["hello", "world", "foo"] => true
		result, err := evalComplex(t, `"world" in ["hello", "world", "foo"]`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != true {
			t.Errorf("expected true, got %v", result)
		}
	})

	t.Run("not_in_operator", func(t *testing.T) {
		// "bar" in ["hello", "world"] => false
		result, err := evalComplex(t, `"bar" in ["hello", "world"]`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != false {
			t.Errorf("expected false, got %v", result)
		}
	})

	t.Run("boolean_cast_empty_string", func(t *testing.T) {
		// $boolean("") => false
		result, err := evalComplex(t, `$boolean("")`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != false {
			t.Errorf("expected false, got %v", result)
		}
	})

	t.Run("boolean_cast_nonempty_string", func(t *testing.T) {
		// $boolean("hello") => true
		result, err := evalComplex(t, `$boolean("hello")`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != true {
			t.Errorf("expected true, got %v", result)
		}
	})

	t.Run("boolean_cast_zero", func(t *testing.T) {
		// $boolean(0) => false
		result, err := evalComplex(t, `$boolean(0)`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != false {
			t.Errorf("expected false, got %v", result)
		}
	})

	t.Run("not_function", func(t *testing.T) {
		// $not(false) => true
		result, err := evalComplex(t, `$not(false)`, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != true {
			t.Errorf("expected true, got %v", result)
		}
	})
}
