package unit_test

import (
	"testing"

	gosonata "github.com/sandrolain/gosonata"
	"github.com/sandrolain/gosonata/pkg/ext"
	"github.com/sandrolain/gosonata/pkg/ext/extarray"
	"github.com/sandrolain/gosonata/pkg/ext/extcrypto"
	"github.com/sandrolain/gosonata/pkg/ext/extdatetime"
	"github.com/sandrolain/gosonata/pkg/ext/extformat"
	"github.com/sandrolain/gosonata/pkg/ext/extfunc"
	"github.com/sandrolain/gosonata/pkg/ext/extnumeric"
	"github.com/sandrolain/gosonata/pkg/ext/extobject"
	"github.com/sandrolain/gosonata/pkg/ext/extstring"
	"github.com/sandrolain/gosonata/pkg/ext/exttypes"
)

// extEval is a small helper that calls gosonata.Eval with the given options
// and fails the test on the first error.
func extEval(t *testing.T, expr string, data interface{}, opts ...gosonata.EvalOption) interface{} {
	t.Helper()
	got, err := gosonata.Eval(expr, data, opts...)
	if err != nil {
		t.Fatalf("Eval(%q): %v", expr, err)
	}
	return got
}

// ── WithFunctions unified API ─────────────────────────────────────────────────

func TestWithFunctions_SingleEntry(t *testing.T) {
	got := extEval(t, `$startsWith("GoSonata", "Go")`, nil,
		gosonata.WithFunctions(extstring.StartsWith()),
	)
	if got != true {
		t.Errorf("got %v, want true", got)
	}
}

func TestWithFunctions_SpreadSlice(t *testing.T) {
	got := extEval(t, `$last([10,20,30])`, nil,
		gosonata.WithFunctions(extarray.AllEntries()...),
	)
	if got != float64(30) {
		t.Errorf("got %v, want 30", got)
	}
}

func TestWithFunctions_MixedEntries(t *testing.T) {
	entries := append(extstring.AllEntries(), extarray.AllEntries()...)
	got := extEval(t, `$startsWith($string($last([1,2,3])), "3")`, nil,
		gosonata.WithFunctions(entries...),
	)
	if got != true {
		t.Errorf("got %v, want true", got)
	}
}

func TestWithFunctions_AdvancedEntry(t *testing.T) {
	got := extEval(t,
		`$groupBy([1,2,3,4], function($v){$v % 2 = 0 ? "even" : "odd"})`,
		nil,
		gosonata.WithFunctions(extarray.GroupBy()),
	)
	obj, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", got)
	}
	if len(obj["even"].([]interface{})) != 2 {
		t.Errorf("expected 2 evens")
	}
}

func TestAllEntries_ExtPackage(t *testing.T) {
	entries := ext.AllEntries()
	if len(entries) == 0 {
		t.Fatal("ext.AllEntries() returned empty slice")
	}
	opt := gosonata.WithFunctions(entries...)
	cases := []struct {
		expr string
		data interface{}
		want interface{}
	}{
		{`$startsWith("abc", "ab")`, nil, true},
		{`$last([1,2,3])`, nil, float64(3)},
		{`$sign(-1)`, nil, float64(-1)},
		{`$isNumber(42)`, nil, true},
		{`$size({"a":1,"b":2})`, nil, float64(2)},
	}
	for _, c := range cases {
		t.Run(c.expr, func(t *testing.T) {
			got := extEval(t, c.expr, c.data, opt)
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// ── extstring ────────────────────────────────────────────────────────────────

func TestExtString(t *testing.T) {
	opt := gosonata.WithFunctions(extstring.AllEntries()...)

	cases := []struct {
		name string
		expr string
		data interface{}
		want interface{}
	}{
		{"startsWith true", `$startsWith("Hello World", "Hello")`, nil, true},
		{"startsWith false", `$startsWith("Hello World", "World")`, nil, false},
		{"endsWith true", `$endsWith("Hello World", "World")`, nil, true},
		{"endsWith false", `$endsWith("Hello World", "Hello")`, nil, false},
		{"indexOf", `$indexOf("abcabc", "bc")`, nil, float64(1)},
		{"indexOf offset", `$indexOf("abcabc", "bc", 2)`, nil, float64(4)},
		{"lastIndexOf", `$lastIndexOf("abcabc", "bc")`, nil, float64(4)},
		{"capitalize", `$capitalize("hello world")`, nil, "Hello world"},
		{"titleCase", `$titleCase("hello world")`, nil, "Hello World"},
		{"camelCase", `$camelCase("hello_world")`, nil, "helloWorld"},
		{"snakeCase", `$snakeCase("helloWorld")`, nil, "hello_world"},
		{"kebabCase", `$kebabCase("helloWorld")`, nil, "hello-world"},
		{"repeat", `$repeat("ab", 3)`, nil, "ababab"},
		{"template", `$template("Hello, {{name}}!", {"name": "World"})`, nil, "Hello, World!"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extEval(t, c.expr, c.data, opt)
			if got != c.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, c.want, c.want)
			}
		})
	}
}

func TestExtString_Words(t *testing.T) {
	opt := gosonata.WithFunctions(extstring.AllEntries()...)
	got := extEval(t, `$count($words("hello world foo"))`, nil, opt)
	if got != float64(3) {
		t.Errorf("$words: got %v, want 3", got)
	}
}

func TestExtString_AllEntries_Count(t *testing.T) {
	entries := extstring.AllEntries()
	if len(entries) == 0 {
		t.Fatal("extstring.AllEntries() is empty")
	}
}

// ── extarray ─────────────────────────────────────────────────────────────────

func TestExtArray_Simple(t *testing.T) {
	opt := gosonata.WithFunctions(extarray.AllEntries()...)
	nums := []interface{}{float64(1), float64(2), float64(3), float64(4), float64(5)}

	t.Run("$first", func(t *testing.T) {
		got := extEval(t, `$first(items)`, map[string]interface{}{"items": nums}, opt)
		if got != float64(1) {
			t.Errorf("got %v, want 1", got)
		}
	})
	t.Run("$last", func(t *testing.T) {
		got := extEval(t, `$last(items)`, map[string]interface{}{"items": nums}, opt)
		if got != float64(5) {
			t.Errorf("got %v, want 5", got)
		}
	})
	t.Run("$take", func(t *testing.T) {
		got := extEval(t, `$take(items, 2)`, map[string]interface{}{"items": nums}, opt)
		arr := got.([]interface{})
		if len(arr) != 2 {
			t.Errorf("$take: got len %d, want 2", len(arr))
		}
	})
	t.Run("$skip", func(t *testing.T) {
		got := extEval(t, `$skip(items, 2)`, map[string]interface{}{"items": nums}, opt)
		arr := got.([]interface{})
		if len(arr) != 3 {
			t.Errorf("$skip: got len %d, want 3", len(arr))
		}
	})
	t.Run("$slice", func(t *testing.T) {
		got := extEval(t, `$slice([10,20,30,40,50], 1, 3)`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 2 || arr[0] != float64(20) {
			t.Errorf("$slice: got %v, want [20 30]", arr)
		}
	})
	t.Run("$flatten", func(t *testing.T) {
		got := extEval(t, `$flatten([[1,[2]],3])`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 3 {
			t.Errorf("$flatten: got len %d, want 3", len(arr))
		}
	})
	t.Run("$chunk", func(t *testing.T) {
		got := extEval(t, `$chunk([1,2,3,4,5], 2)`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 3 {
			t.Errorf("$chunk: got %d chunks, want 3", len(arr))
		}
	})
	t.Run("$range", func(t *testing.T) {
		// $range is end-inclusive: $range(1,5,1) → [1,2,3,4,5]
		got := extEval(t, `$range(1, 5, 1)`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 5 {
			t.Errorf("$range: got len %d, want 5", len(arr))
		}
		if arr[0] != float64(1) || arr[4] != float64(5) {
			t.Errorf("$range: got %v, want [1..5]", arr)
		}
	})
}

func TestExtArray_SetOps(t *testing.T) {
	opt := gosonata.WithFunctions(extarray.AllEntries()...)

	t.Run("$union", func(t *testing.T) {
		got := extEval(t, `$union([1,2,3],[2,3,4])`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 4 {
			t.Errorf("$union: got len %d, want 4", len(arr))
		}
	})
	t.Run("$intersection", func(t *testing.T) {
		got := extEval(t, `$intersection([1,2,3],[2,3,4])`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 2 {
			t.Errorf("$intersection: got len %d, want 2", len(arr))
		}
	})
	t.Run("$difference", func(t *testing.T) {
		got := extEval(t, `$difference([1,2,3],[2,3])`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 1 || arr[0] != float64(1) {
			t.Errorf("$difference: got %v, want [1]", arr)
		}
	})
	t.Run("$symmetricDifference", func(t *testing.T) {
		got := extEval(t, `$symmetricDifference([1,2,3],[2,3,4])`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 2 {
			t.Errorf("$symmetricDifference: got len %d, want 2", len(arr))
		}
	})
}

func TestExtArray_HOF(t *testing.T) {
	opt := gosonata.WithFunctions(extarray.AllEntries()...)

	t.Run("$groupBy", func(t *testing.T) {
		got := extEval(t, `$groupBy([1,2,3,4], function($v){$string($v % 2)})`, nil, opt)
		obj := got.(map[string]interface{})
		if obj == nil || len(obj) != 2 {
			t.Errorf("$groupBy: expected 2 groups, got %v", obj)
		}
	})
	t.Run("$countBy", func(t *testing.T) {
		got := extEval(t, `$countBy([1,2,3,4,5,6], function($v){$v % 2 = 0 ? "even" : "odd"}).even`, nil, opt)
		if got != float64(3) {
			t.Errorf("$countBy even: got %v, want 3", got)
		}
	})
	t.Run("$sumBy", func(t *testing.T) {
		data := map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"price": float64(10), "qty": float64(3)},
				map[string]interface{}{"price": float64(5), "qty": float64(2)},
			},
		}
		got := extEval(t, `$sumBy(items, function($x){$x.price * $x.qty})`, data, opt)
		if got != float64(40) {
			t.Errorf("$sumBy: got %v, want 40", got)
		}
	})
	t.Run("$minBy", func(t *testing.T) {
		got := extEval(t, `$minBy([3,1,2], function($v){$v})`, nil, opt)
		if got != float64(1) {
			t.Errorf("$minBy: got %v, want 1", got)
		}
	})
	t.Run("$maxBy", func(t *testing.T) {
		got := extEval(t, `$maxBy([3,1,2], function($v){$v})`, nil, opt)
		if got != float64(3) {
			t.Errorf("$maxBy: got %v, want 3", got)
		}
	})
	t.Run("$accumulate", func(t *testing.T) {
		// $accumulate returns a scan array (init + each partial result)
		// [1,2,3,4] with init=0 → [0, 1, 3, 6, 10]
		got := extEval(t, `$last($accumulate([1,2,3,4], function($acc,$v){$acc+$v}, 0))`, nil, opt)
		if got != float64(10) {
			t.Errorf("$accumulate: got %v, want 10", got)
		}
	})
}

func TestExtArray_Window(t *testing.T) {
	opt := gosonata.WithFunctions(extarray.AllEntries()...)
	// $window(array, size, step): with size=2, step=1 over [1,2,3,4] → 3 windows
	got := extEval(t, `$count($window([1,2,3,4], 2, 1))`, nil, opt)
	if got != float64(3) {
		t.Errorf("$window: got %v, want 3", got)
	}
}

func TestExtArray_ZipLongest(t *testing.T) {
	opt := gosonata.WithFunctions(extarray.AllEntries()...)
	got := extEval(t, `$count($zipLongest([1,2,3],[4,5]))`, nil, opt)
	if got != float64(3) {
		t.Errorf("$zipLongest: got %v, want 3", got)
	}
}

// ── extnumeric ───────────────────────────────────────────────────────────────

func TestExtNumeric(t *testing.T) {
	opt := gosonata.WithFunctions(extnumeric.AllEntries()...)

	cases := []struct {
		name string
		expr string
		want interface{}
	}{
		{"sign negative", `$sign(-5)`, float64(-1)},
		{"sign zero", `$sign(0)`, float64(0)},
		{"sign positive", `$sign(3)`, float64(1)},
		{"trunc positive", `$trunc(3.9)`, float64(3)},
		{"trunc negative", `$trunc(-3.9)`, float64(-3)},
		{"clamp below", `$clamp(-5, 0, 100)`, float64(0)},
		{"clamp above", `$clamp(150, 0, 100)`, float64(100)},
		{"clamp inside", `$clamp(50, 0, 100)`, float64(50)},
		{"log base10", `$log(100, 10)`, float64(2)},
		{"sin", `$sin(0)`, float64(0)},
		{"cos", `$cos(0)`, float64(1)},
		{"atan2", `$atan2(0, 1)`, float64(0)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extEval(t, c.expr, nil, opt)
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestExtNumeric_Stats(t *testing.T) {
	opt := gosonata.WithFunctions(extnumeric.AllEntries()...)
	nums := `[1,2,3,4,5]`

	t.Run("$median", func(t *testing.T) {
		got := extEval(t, `$median(`+nums+`)`, nil, opt)
		if got != float64(3) {
			t.Errorf("$median: got %v, want 3", got)
		}
	})
	t.Run("$mode", func(t *testing.T) {
		// $mode returns the most frequent value; with [1,2,2,3] the mode is 2.
		// The result may be a scalar or a single-element array depending on the impl.
		got := extEval(t, `$mode([1,2,2,3])`, nil, opt)
		// Normalise to a slice so the check works in both cases.
		var modes []interface{}
		switch v := got.(type) {
		case []interface{}:
			modes = v
		default:
			modes = []interface{}{v}
		}
		if len(modes) == 0 || modes[0] != float64(2) {
			t.Errorf("$mode: got %v, want [2]", got)
		}
	})
	t.Run("$variance positive", func(t *testing.T) {
		got := extEval(t, `$variance(`+nums+`) > 0`, nil, opt)
		if got != true {
			t.Errorf("$variance: expected > 0")
		}
	})
	t.Run("$stddev positive", func(t *testing.T) {
		got := extEval(t, `$stddev(`+nums+`) > 0`, nil, opt)
		if got != true {
			t.Errorf("$stddev: expected > 0")
		}
	})
	t.Run("$percentile 50", func(t *testing.T) {
		got := extEval(t, `$percentile(`+nums+`, 50)`, nil, opt)
		if got == nil {
			t.Error("$percentile: got nil")
		}
	})
}

func TestExtNumeric_Constants(t *testing.T) {
	opt := gosonata.WithFunctions(extnumeric.AllEntries()...)
	t.Run("$pi", func(t *testing.T) {
		got := extEval(t, `$pi() > 3.14`, nil, opt)
		if got != true {
			t.Errorf("$pi: got %v, want true", got)
		}
	})
	t.Run("$e", func(t *testing.T) {
		got := extEval(t, `$e() > 2.71`, nil, opt)
		if got != true {
			t.Errorf("$e: got %v, want true", got)
		}
	})
}

// ── extobject ────────────────────────────────────────────────────────────────

func TestExtObject_Simple(t *testing.T) {
	opt := gosonata.WithFunctions(extobject.AllEntries()...)

	t.Run("$values", func(t *testing.T) {
		got := extEval(t, `$count($values({"a":1,"b":2}))`, nil, opt)
		if got != float64(2) {
			t.Errorf("$values: got %v, want 2", got)
		}
	})
	t.Run("$pairs", func(t *testing.T) {
		got := extEval(t, `$count($pairs({"a":1,"b":2}))`, nil, opt)
		if got != float64(2) {
			t.Errorf("$pairs: got %v, want 2", got)
		}
	})
	t.Run("$fromPairs", func(t *testing.T) {
		got := extEval(t, `$fromPairs([["x",10],["y",20]]).x`, nil, opt)
		if got != float64(10) {
			t.Errorf("$fromPairs: got %v, want 10", got)
		}
	})
	t.Run("$pick", func(t *testing.T) {
		got := extEval(t, `$count($keys($pick({"a":1,"b":2,"c":3}, ["a","c"])))`, nil, opt)
		if got != float64(2) {
			t.Errorf("$pick: got %v, want 2", got)
		}
	})
	t.Run("$omit", func(t *testing.T) {
		got := extEval(t, `$count($keys($omit({"a":1,"b":2,"c":3}, ["b"])))`, nil, opt)
		if got != float64(2) {
			t.Errorf("$omit: got %v, want 2", got)
		}
	})
	t.Run("$size", func(t *testing.T) {
		got := extEval(t, `$size({"a":1,"b":2,"c":3})`, nil, opt)
		if got != float64(3) {
			t.Errorf("$size: got %v, want 3", got)
		}
	})
	t.Run("$deepMerge", func(t *testing.T) {
		got := extEval(t, `$deepMerge([{"a":{"x":1}},{"a":{"y":2}}]).a.y`, nil, opt)
		if got != float64(2) {
			t.Errorf("$deepMerge: got %v, want 2", got)
		}
	})
	t.Run("$rename", func(t *testing.T) {
		got := extEval(t, `$rename({"old_key":"val"},{"old_key":"newKey"}).newKey`, nil, opt)
		if got != "val" {
			t.Errorf("$rename: got %v, want val", got)
		}
	})
	t.Run("$invert", func(t *testing.T) {
		got := extEval(t, `$count($keys($invert({"a":"1","b":"2"})))`, nil, opt)
		if got != float64(2) {
			t.Errorf("$invert: got %v, want 2 keys", got)
		}
	})
}

func TestExtObject_HOF(t *testing.T) {
	opt := gosonata.WithFunctions(extobject.AllEntries()...)

	t.Run("$mapValues", func(t *testing.T) {
		got := extEval(t, `$mapValues({"a":1,"b":2}, function($v){$v*10}).a`, nil, opt)
		if got != float64(10) {
			t.Errorf("$mapValues: got %v, want 10", got)
		}
	})
	t.Run("$mapKeys", func(t *testing.T) {
		got := extEval(t, `$count($keys($mapKeys({"a":1,"b":2}, function($k){$uppercase($k)})))`, nil, opt)
		if got != float64(2) {
			t.Errorf("$mapKeys: got %v, want 2", got)
		}
	})
}

// ── exttypes ─────────────────────────────────────────────────────────────────

func TestExtTypes(t *testing.T) {
	opt := gosonata.WithFunctions(exttypes.AllEntries()...)

	cases := []struct {
		name string
		expr string
		want bool
	}{
		{"isString string", `$isString("hello")`, true},
		{"isString number", `$isString(42)`, false},
		{"isNumber number", `$isNumber(42)`, true},
		{"isNumber string", `$isNumber("42")`, false},
		{"isBoolean true", `$isBoolean(true)`, true},
		{"isBoolean 1", `$isBoolean(1)`, false},
		{"isArray array", `$isArray([1,2,3])`, true},
		{"isArray string", `$isArray("abc")`, false},
		{"isObject object", `$isObject({"a":1})`, true},
		{"isObject array", `$isObject([1,2])`, false},
		{"isNull null", `$isNull(null)`, true},
		{"isNull string", `$isNull("x")`, false},
		{"isEmpty empty-string", `$isEmpty("")`, true},
		{"isEmpty empty-array", `$isEmpty([])`, true},
		{"isEmpty empty-object", `$isEmpty({})`, true},
		{"isEmpty nonempty-string", `$isEmpty("x")`, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extEval(t, c.expr, nil, opt)
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// ── extcrypto ────────────────────────────────────────────────────────────────

func TestExtCrypto_UUID(t *testing.T) {
	opt := gosonata.WithFunctions(extcrypto.AllEntries()...)
	got := extEval(t, `$uuid()`, nil, opt)
	s, ok := got.(string)
	if !ok {
		t.Fatalf("$uuid: expected string, got %T", got)
	}
	if len(s) != 36 {
		t.Errorf("$uuid: expected len 36, got %d (%q)", len(s), s)
	}
}

func TestExtCrypto_Hash(t *testing.T) {
	opt := gosonata.WithFunctions(extcrypto.AllEntries()...)

	cases := []struct {
		algo string
	}{
		{"md5"},
		{"sha1"},
		{"sha256"},
		{"sha512"},
	}

	for _, c := range cases {
		t.Run("hash-"+c.algo, func(t *testing.T) {
			got := extEval(t, `$hash("hello", "`+c.algo+`")`, nil, opt)
			if s, ok := got.(string); !ok || len(s) == 0 {
				t.Errorf("$hash(%s): got empty or non-string: %v", c.algo, got)
			}
		})
	}
}

func TestExtCrypto_Hash_SHA256_KnownValue(t *testing.T) {
	opt := gosonata.WithFunctions(extcrypto.AllEntries()...)
	got := extEval(t, `$hash("hello", "sha256")`, nil, opt)
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Errorf("$hash sha256: got %v, want %v", got, want)
	}
}

func TestExtCrypto_HMAC(t *testing.T) {
	opt := gosonata.WithFunctions(extcrypto.AllEntries()...)
	got := extEval(t, `$hmac("message", "secret", "sha256")`, nil, opt)
	s, ok := got.(string)
	if !ok || len(s) == 0 {
		t.Errorf("$hmac: expected non-empty string, got %v", got)
	}
}

// ── extdatetime ──────────────────────────────────────────────────────────────

func TestExtDateTime(t *testing.T) {
	opt := gosonata.WithFunctions(extdatetime.AllEntries()...)

	t.Run("$dateAdd day", func(t *testing.T) {
		got := extEval(t, `$dateAdd(0, 1, "day")`, nil, opt)
		if got != float64(86400000) {
			t.Errorf("$dateAdd day: got %v, want 86400000", got)
		}
	})
	t.Run("$dateDiff day", func(t *testing.T) {
		got := extEval(t, `$dateDiff(0, 86400000, "day")`, nil, opt)
		if got != float64(1) {
			t.Errorf("$dateDiff day: got %v, want 1", got)
		}
	})
	t.Run("$dateAdd hour", func(t *testing.T) {
		got := extEval(t, `$dateAdd(0, 2, "hour")`, nil, opt)
		if got != float64(7200000) {
			t.Errorf("$dateAdd hour: got %v, want 7200000", got)
		}
	})
	t.Run("$dateComponents year", func(t *testing.T) {
		got := extEval(t, `$dateComponents(0).year`, nil, opt)
		if got != float64(1970) {
			t.Errorf("$dateComponents year: got %v, want 1970", got)
		}
	})
	t.Run("$dateComponents month", func(t *testing.T) {
		got := extEval(t, `$dateComponents(0).month`, nil, opt)
		if got != float64(1) {
			t.Errorf("$dateComponents month: got %v, want 1", got)
		}
	})
	t.Run("$dateStartOf day", func(t *testing.T) {
		got := extEval(t, `$dateStartOf(1000, "day")`, nil, opt)
		if got != float64(0) {
			t.Errorf("$dateStartOf day: got %v, want 0", got)
		}
	})
	t.Run("$dateEndOf day", func(t *testing.T) {
		got := extEval(t, `$dateEndOf(0, "day")`, nil, opt)
		if got != float64(86399999) {
			t.Errorf("$dateEndOf day: got %v, want 86399999", got)
		}
	})
}

// ── extformat ────────────────────────────────────────────────────────────────

func TestExtFormat_CSV(t *testing.T) {
	opt := gosonata.WithFunctions(extformat.AllEntries()...)

	t.Run("$csv parse", func(t *testing.T) {
		got := extEval(t, `$csv("name,age\nAlice,30\nBob,25")`, nil, opt)
		arr := got.([]interface{})
		if len(arr) != 2 {
			t.Fatalf("$csv: expected 2 rows, got %d", len(arr))
		}
		first := arr[0].(map[string]interface{})
		if first["name"] != "Alice" {
			t.Errorf("$csv: first row name = %v, want Alice", first["name"])
		}
	})
	t.Run("$toCSV", func(t *testing.T) {
		data := map[string]interface{}{
			"rows": []interface{}{
				map[string]interface{}{"name": "Alice", "age": float64(30)},
				map[string]interface{}{"name": "Bob", "age": float64(25)},
			},
		}
		got := extEval(t, `$toCSV(rows, ["name","age"])`, data, opt)
		s, ok := got.(string)
		if !ok || s == "" {
			t.Errorf("$toCSV: expected non-empty string, got %v", got)
		}
	})
}

func TestExtFormat_Template(t *testing.T) {
	opt := gosonata.WithFunctions(extformat.AllEntries()...)
	got := extEval(t, `$template("Hi {{first}} {{last}}!", {"first":"John","last":"Doe"})`, nil, opt)
	if got != "Hi John Doe!" {
		t.Errorf("$template: got %v, want 'Hi John Doe!'", got)
	}
}

// ── extfunc ──────────────────────────────────────────────────────────────────

func TestExtFunc_Pipe(t *testing.T) {
	opt := gosonata.WithFunctions(extfunc.AllEntries()...)
	got := extEval(t, `$pipe("  hello  ", $trim, $uppercase)`, nil, opt)
	if got != "HELLO" {
		t.Errorf("$pipe: got %v, want HELLO", got)
	}
}

func TestExtFunc_Memoize(t *testing.T) {
	opt := gosonata.WithFunctions(extfunc.AllEntries()...)
	got := extEval(t, `
		($sq := $memoize(function($n){$n * $n});
		 [$sq(4), $sq(4)])
	`, nil, opt)
	arr, ok := got.([]interface{})
	if !ok || len(arr) != 2 || arr[0] != float64(16) || arr[1] != float64(16) {
		t.Errorf("$memoize: got %v, want [16, 16]", got)
	}
}
