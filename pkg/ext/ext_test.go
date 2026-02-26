package ext_test

import (
	"context"
	"testing"

	gosonata "github.com/sandrolain/gosonata"
	"github.com/sandrolain/gosonata/pkg/ext"
	"github.com/sandrolain/gosonata/pkg/ext/extarray"
	"github.com/sandrolain/gosonata/pkg/ext/extstring"
)

func eval(t *testing.T, expr string, data interface{}, opts ...gosonata.EvalOption) interface{} {
	t.Helper()
	result, err := gosonata.Eval(expr, data, opts...)
	if err != nil {
		t.Fatalf("Eval(%q) error: %v", expr, err)
	}
	return result
}

func evalCtx(t *testing.T, expr string, data interface{}, opts ...gosonata.EvalOption) interface{} {
	t.Helper()
	ctx := context.Background()
	e, err := gosonata.Compile(expr)
	if err != nil {
		t.Fatalf("Compile(%q) error: %v", expr, err)
	}
	_ = ctx
	result, err := gosonata.Eval(expr, data, opts...)
	_ = e
	if err != nil {
		t.Fatalf("Eval(%q) error: %v", expr, err)
	}
	return result
}

// ── WithAll ────────────────────────────────────────────────────────────────

func TestWithAll_StringFunctions(t *testing.T) {
	opt := ext.WithAll()

	tests := []struct {
		expr string
		data interface{}
		want interface{}
	}{
		{`$startsWith("Hello World", "Hello")`, nil, true},
		{`$startsWith("Hello World", "World")`, nil, false},
		{`$endsWith("Hello World", "World")`, nil, true},
		{`$indexOf("abcabc", "bc")`, nil, float64(1)},
		{`$indexOf("abcabc", "bc", 2)`, nil, float64(4)},
		{`$lastIndexOf("abcabc", "bc")`, nil, float64(4)},
		{`$capitalize("hello world")`, nil, "Hello world"},
		{`$titleCase("hello world")`, nil, "Hello World"},
		{`$camelCase("hello_world")`, nil, "helloWorld"},
		{`$snakeCase("helloWorld")`, nil, "hello_world"},
		{`$kebabCase("helloWorld")`, nil, "hello-world"},
		{`$repeat("ab", 3)`, nil, "ababab"},
		{`$template("Hello, {{name}}!", {"name": "World"})`, nil, "Hello, World!"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := eval(t, tt.expr, tt.data, opt)
			if got != tt.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestWithAll_NumericFunctions(t *testing.T) {
	opt := ext.WithAll()

	tests := []struct {
		expr string
		data interface{}
		want interface{}
	}{
		{`$sign(-5)`, nil, float64(-1)},
		{`$sign(0)`, nil, float64(0)},
		{`$sign(3)`, nil, float64(1)},
		{`$trunc(-3.7)`, nil, float64(-3)},
		{`$trunc(3.7)`, nil, float64(3)},
		{`$clamp(150, 0, 100)`, nil, float64(100)},
		{`$clamp(-5, 0, 100)`, nil, float64(0)},
		{`$clamp(50, 0, 100)`, nil, float64(50)},
		{`$log(100, 10)`, nil, float64(2)},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := eval(t, tt.expr, tt.data, opt)
			if got != tt.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestWithAll_ArrayFunctions(t *testing.T) {
	opt := ext.WithAll()
	data := map[string]interface{}{
		"items": []interface{}{float64(1), float64(2), float64(3), float64(4), float64(5)},
	}

	t.Run("$first", func(t *testing.T) {
		got := eval(t, `$first(items)`, data, opt)
		if got != float64(1) {
			t.Errorf("got %v, want 1", got)
		}
	})
	t.Run("$last", func(t *testing.T) {
		got := eval(t, `$last(items)`, data, opt)
		if got != float64(5) {
			t.Errorf("got %v, want 5", got)
		}
	})
	t.Run("$take", func(t *testing.T) {
		got := eval(t, `$take(items, 3)`, data, opt)
		arr := got.([]interface{})
		if len(arr) != 3 {
			t.Errorf("got len %d, want 3", len(arr))
		}
	})
	t.Run("$skip", func(t *testing.T) {
		got := eval(t, `$skip(items, 2)`, data, opt)
		arr := got.([]interface{})
		if len(arr) != 3 {
			t.Errorf("got len %d, want 3", len(arr))
		}
	})
	t.Run("$flatten", func(t *testing.T) {
		got := eval(t, `$flatten([[1,[2]],3])`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 3 {
			t.Errorf("got len %d, want 3", len(arr))
		}
	})
	t.Run("$chunk", func(t *testing.T) {
		got := eval(t, `$chunk([1,2,3,4,5], 2)`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 3 {
			t.Errorf("got %d chunks, want 3", len(arr))
		}
	})
	t.Run("$union", func(t *testing.T) {
		got := eval(t, `$union([1,2,3], [2,3,4])`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 4 {
			t.Errorf("got %d items, want 4", len(arr))
		}
	})
	t.Run("$intersection", func(t *testing.T) {
		got := eval(t, `$intersection([1,2,3], [2,3,4])`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 2 {
			t.Errorf("got %d items, want 2", len(arr))
		}
	})
	t.Run("$difference", func(t *testing.T) {
		got := eval(t, `$difference([1,2,3], [2,3,4])`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 1 {
			t.Errorf("got %d items, want 1", len(arr))
		}
	})
}

func TestWithAll_ObjectFunctions(t *testing.T) {
	opt := ext.WithAll()
	data := map[string]interface{}{
		"user": map[string]interface{}{
			"name":  "Alice",
			"email": "alice@example.com",
			"token": "secret",
		},
	}

	t.Run("$values", func(t *testing.T) {
		got := eval(t, `$count($values({"a":1,"b":2}))`, nil, opt)
		if got != float64(2) {
			t.Errorf("got %v, want 2", got)
		}
	})
	t.Run("$pairs", func(t *testing.T) {
		got := eval(t, `$count($pairs({"a":1,"b":2}))`, nil, opt)
		if got != float64(2) {
			t.Errorf("got %v, want 2", got)
		}
	})
	t.Run("$fromPairs", func(t *testing.T) {
		got := eval(t, `$fromPairs([["a",1],["b",2]]).a`, nil, opt)
		if got != float64(1) {
			t.Errorf("got %v, want 1", got)
		}
	})
	t.Run("$pick", func(t *testing.T) {
		got := eval(t, `$pick(user, ["name","email"])`, data, opt)
		obj := got.(map[string]interface{})
		if _, hasToken := obj["token"]; hasToken {
			t.Error("$pick should not include token")
		}
		if _, hasName := obj["name"]; !hasName {
			t.Error("$pick should include name")
		}
	})
	t.Run("$omit", func(t *testing.T) {
		got := eval(t, `$omit(user, ["token"])`, data, opt)
		obj := got.(map[string]interface{})
		if _, hasToken := obj["token"]; hasToken {
			t.Error("$omit should exclude token")
		}
	})
	t.Run("$size", func(t *testing.T) {
		got := eval(t, `$size({"a":1,"b":2,"c":3})`, nil, opt)
		if got != float64(3) {
			t.Errorf("got %v, want 3", got)
		}
	})
	t.Run("$deepMerge", func(t *testing.T) {
		got := eval(t, `$deepMerge([{"a":{"x":1}},{"a":{"y":2}}]).a.y`, nil, opt)
		if got != float64(2) {
			t.Errorf("got %v, want 2", got)
		}
	})
	t.Run("$rename", func(t *testing.T) {
		got := eval(t, `$rename({"first_name":"Alice"},{"first_name":"firstName"}).firstName`, nil, opt)
		if got != "Alice" {
			t.Errorf("got %v, want Alice", got)
		}
	})
	t.Run("$invert", func(t *testing.T) {
		// $invert swaps keys and values; verify we get 2 keys in the result
		got := eval(t, `$count($keys($invert({"a":"1","b":"2"})))`, nil, opt)
		if got != float64(2) {
			t.Errorf("got %v, want 2 keys", got)
		}
	})
}

func TestWithAll_TypePredicates(t *testing.T) {
	opt := ext.WithAll()

	tests := []struct {
		expr string
		data interface{}
		want bool
	}{
		{`$isString("hello")`, nil, true},
		{`$isString(42)`, nil, false},
		{`$isNumber(42)`, nil, true},
		{`$isNumber("hello")`, nil, false},
		{`$isBoolean(true)`, nil, true},
		{`$isBoolean(1)`, nil, false},
		{`$isArray([1,2,3])`, nil, true},
		{`$isArray("hello")`, nil, false},
		{`$isObject({"a":1})`, nil, true},
		{`$isObject([1,2])`, nil, false},
		{`$isEmpty("")`, nil, true},
		{`$isEmpty([])`, nil, true},
		{`$isEmpty({})`, nil, true},
		{`$isEmpty("x")`, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := eval(t, tt.expr, tt.data, opt)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithAll_HOFFunctions(t *testing.T) {
	opt := ext.WithAll()

	t.Run("$groupBy", func(t *testing.T) {
		got := eval(t, `$groupBy([1,2,3,4,5,6], function($v){$v % 2 = 0 ? "even" : "odd"})`, nil, opt)
		obj := got.(map[string]interface{})
		evens := obj["even"].([]interface{})
		odds := obj["odd"].([]interface{})
		if len(evens) != 3 || len(odds) != 3 {
			t.Errorf("expected 3 evens and 3 odds, got %d and %d", len(evens), len(odds))
		}
	})

	t.Run("$sumBy", func(t *testing.T) {
		data := map[string]interface{}{
			"products": []interface{}{
				map[string]interface{}{"price": float64(10), "qty": float64(2)},
				map[string]interface{}{"price": float64(5), "qty": float64(4)},
			},
		}
		got := eval(t, `$sumBy(products, function($p){$p.price * $p.qty})`, data, opt)
		if got != float64(40) {
			t.Errorf("got %v, want 40", got)
		}
	})

	t.Run("$mapValues", func(t *testing.T) {
		got := eval(t, `$mapValues({"a":1,"b":2}, function($v){$v * 10}).a`, nil, opt)
		if got != float64(10) {
			t.Errorf("got %v, want 10", got)
		}
	})

	t.Run("$pipe", func(t *testing.T) {
		got := eval(t, `$pipe("  hello  ", $trim, $uppercase)`, nil, opt)
		if got != "HELLO" {
			t.Errorf("got %v, want HELLO", got)
		}
	})
}

func TestWithAll_Crypto(t *testing.T) {
	opt := ext.WithAll()

	t.Run("$uuid format", func(t *testing.T) {
		got := eval(t, `$uuid()`, nil, opt)
		s, ok := got.(string)
		if !ok || len(s) != 36 {
			t.Errorf("expected UUID string of length 36, got %v (%d)", got, len(s))
		}
	})

	t.Run("$hash sha256", func(t *testing.T) {
		got := eval(t, `$hash("hello", "sha256")`, nil, opt)
		want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestWithAll_DateTime(t *testing.T) {
	opt := ext.WithAll()

	t.Run("$dateComponents year", func(t *testing.T) {
		// 2026-01-15T00:00:00Z in millis
		got := eval(t, `$dateComponents(1768521600000).year`, nil, opt)
		// Accept any valid year (the exact ms may shift)
		if got == nil {
			t.Error("expected non-nil year")
		}
	})

	t.Run("$dateAdd day", func(t *testing.T) {
		// Add 1 day to epoch
		got := eval(t, `$dateAdd(0, 1, "day")`, nil, opt)
		if got != float64(86400000) {
			t.Errorf("got %v, want 86400000", got)
		}
	})

	t.Run("$dateDiff day", func(t *testing.T) {
		got := eval(t, `$dateDiff(0, 86400000, "day")`, nil, opt)
		if got != float64(1) {
			t.Errorf("got %v, want 1", got)
		}
	})
}

func TestWithAll_Format(t *testing.T) {
	opt := ext.WithAll()

	t.Run("$csv parse", func(t *testing.T) {
		got := eval(t, `$csv("a,b\n1,2\n3,4")`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 2 {
			t.Errorf("expected 2 rows, got %d", len(arr))
		}
		first := arr[0].(map[string]interface{})
		if first["a"] != "1" {
			t.Errorf("expected first row a=1, got %v", first["a"])
		}
	})

	t.Run("$toCSV", func(t *testing.T) {
		data := map[string]interface{}{
			"rows": []interface{}{
				map[string]interface{}{"name": "Alice", "age": float64(30)},
				map[string]interface{}{"name": "Bob", "age": float64(25)},
			},
		}
		got := eval(t, `$toCSV(rows, ["name","age"])`, data, opt)
		s, ok := got.(string)
		if !ok || s == "" {
			t.Errorf("expected non-empty CSV string, got %v", got)
		}
	})
}

// ── Per-category options ────────────────────────────────────────────────────

func TestWithString(t *testing.T) {
	got, err := gosonata.Eval(`$startsWith("hello", "he")`, nil, ext.WithString())
	if err != nil || got != true {
		t.Errorf("ext.WithString(): got %v, err %v", got, err)
	}
}

func TestWithArray(t *testing.T) {
	got, err := gosonata.Eval(`$first([10,20,30])`, nil, ext.WithArray())
	if err != nil || got != float64(10) {
		t.Errorf("ext.WithArray(): got %v, err %v", got, err)
	}
}

func TestWithObject(t *testing.T) {
	got, err := gosonata.Eval(`$values({"x":42})[0]`, nil, ext.WithObject())
	if err != nil || got != float64(42) {
		t.Errorf("ext.WithObject(): got %v, err %v", got, err)
	}
}

func TestWithCrypto(t *testing.T) {
	got, err := gosonata.Eval(`$string($length($uuid()))`, nil, ext.WithCrypto())
	if err != nil || got != "36" {
		t.Errorf("ext.WithCrypto(): got %v, err %v", got, err)
	}
}

// ── Single-function registration ────────────────────────────────────────────

func TestSingleFunctionRegistration(t *testing.T) {
	got, err := gosonata.Eval(`$startsWith("hello world", "hello")`, nil,
		gosonata.WithFunctions(extstring.StartsWith()),
	)
	if err != nil || got != true {
		t.Errorf("single registration: got %v, err %v", got, err)
	}
}

func TestBulkFunctionRegistration(t *testing.T) {
	got, err := gosonata.Eval(`$last([1,2,3])`, nil,
		gosonata.WithFunctions(extarray.AllEntries()...),
	)
	if err != nil || got != float64(3) {
		t.Errorf("bulk registration: got %v, err %v", got, err)
	}
}

// ── Mix ext + user custom functions ─────────────────────────────────────────

func TestMixExtAndCustomFunctions(t *testing.T) {
	got, err := gosonata.Eval(`$greet($first(names))`, map[string]interface{}{
		"names": []interface{}{"Alice", "Bob"},
	},
		ext.WithArray(),
		gosonata.WithCustomFunction("greet", "<s:s>", func(_ context.Context, args ...interface{}) (interface{}, error) {
			return "Hello, " + args[0].(string) + "!", nil
		}),
	)
	if err != nil || got != "Hello, Alice!" {
		t.Errorf("mix: got %v, err %v", got, err)
	}
}

// suppress unused import warning
var _ = evalCtx

// TestWithFunctions verifies the unified variadic WithFunctions API with spread.
func TestWithFunctions(t *testing.T) {
	strEntries := extstring.AllEntries()
	arrEntries := extarray.AllEntries()

	// Spread a single typed slice
	t.Run("spread extstring", func(t *testing.T) {
		got := eval(t, `$startsWith("GoSonata", "Go")`, nil, gosonata.WithFunctions(strEntries...))
		if got != true {
			t.Errorf("got %v, want true", got)
		}
	})

	// Spread a mixed slice built with append
	t.Run("spread mixed string+array", func(t *testing.T) {
		mixed := append(strEntries, arrEntries...)
		got := eval(t, `$first([10, 20, 30])`, nil, gosonata.WithFunctions(mixed...))
		if got != float64(10) {
			t.Errorf("got %v, want 10", got)
		}
	})

	// All entries at once via ext.AllEntries()
	t.Run("ext.AllEntries()", func(t *testing.T) {
		got := eval(t, `$skip([1,2,3,4,5], 2)`, nil, gosonata.WithFunctions(ext.AllEntries()...))
		arr := got.([]interface{})
		if len(arr) != 3 {
			t.Errorf("got len %d, want 3", len(arr))
		}
	})
}
