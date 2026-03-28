package unit_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/parser"
	"github.com/sandrolain/gosonata/pkg/types"
)

// ---------------------------------------------------------------------------
// Classification tests: does AnalyzeFastPath produce the expected FastPathKind?
// ---------------------------------------------------------------------------

var fastPathClassificationCases = []struct {
	query    string
	wantKind types.FastPathKind
	wantPath string // gjson path (empty when not FP)
}{
	// Pure path
	{"Account", types.FastPathPurePath, "Account"},
	{"Account.Name", types.FastPathPurePath, "Account.Name"},
	{"a.b.c", types.FastPathPurePath, "a.b.c"},

	// Comparison
	{`a = "foo"`, types.FastPathComparison, "a"},
	{`a != "bar"`, types.FastPathComparison, "a"},
	{`user.role = "admin"`, types.FastPathComparison, "user.role"},

	// Function fast-path
	{"$exists(a.b)", types.FastPathFunc, "a.b"},
	{"$lowercase(name)", types.FastPathFunc, "name"},
	{"$uppercase(name)", types.FastPathFunc, "name"},
	{"$trim(field)", types.FastPathFunc, "field"},
	{"$length(field)", types.FastPathFunc, "field"},
	{"$type(field)", types.FastPathFunc, "field"},
	{"$boolean(flag)", types.FastPathFunc, "flag"},
	{"$not(flag)", types.FastPathFunc, "flag"},
	{"$string(num)", types.FastPathFunc, "num"},
	{"$number(str)", types.FastPathFunc, "str"},
	{"$count(arr)", types.FastPathFunc, "arr"},
	{"$sum(arr)", types.FastPathFunc, "arr"},
	{"$max(arr)", types.FastPathFunc, "arr"},
	{"$min(arr)", types.FastPathFunc, "arr"},
	{"$average(arr)", types.FastPathFunc, "arr"},
	{"$reverse(arr)", types.FastPathFunc, "arr"},
	{"$keys(obj)", types.FastPathFunc, "obj"},
	{"$distinct(arr)", types.FastPathFunc, "arr"},
	{"$abs(n)", types.FastPathFunc, "n"},
	{"$floor(n)", types.FastPathFunc, "n"},
	{"$ceil(n)", types.FastPathFunc, "n"},
	{"$sqrt(n)", types.FastPathFunc, "n"},

	// NOT eligible
	{"a.b[c = 1]", types.FastPathNone, ""}, // predicate
	{"*", types.FastPathNone, ""},          // wildcard
	{"**", types.FastPathNone, ""},         // descendant
	{"$map(a, function($v){$v})", types.FastPathNone, ""},
	{"$sum(a) + $sum(b)", types.FastPathNone, ""}, // binary on functions
}

func TestFastPathClassification(t *testing.T) {
	for _, tc := range fastPathClassificationCases {
		tc := tc
		t.Run(tc.query, func(t *testing.T) {
			expr, err := parser.Compile(tc.query)
			if err != nil {
				t.Fatalf("Compile(%q) error: %v", tc.query, err)
			}

			if tc.wantKind == types.FastPathNone {
				if expr.IsFastPath() {
					t.Errorf("expected no fast-path, got FastPath=%+v", expr.FastPath)
				}
				return
			}

			if !expr.IsFastPath() {
				t.Fatalf("expected fast-path, IsFastPath()=false")
			}
			if expr.FastPath.Kind != tc.wantKind {
				t.Errorf("Kind: want %d, got %d", tc.wantKind, expr.FastPath.Kind)
			}
			if tc.wantPath != "" && expr.FastPath.GJSONPath != tc.wantPath {
				t.Errorf("GJSONPath: want %q, got %q", tc.wantPath, expr.FastPath.GJSONPath)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// EvalBytes correctness: fast-path results must match full-eval results.
// ---------------------------------------------------------------------------

var evalBytesCases = []struct {
	name  string
	query string
	json  string
	want  interface{}
}{
	// --- Pure path ---
	{"string field", "user.email", `{"user":{"email":"test@example.com"}}`, "test@example.com"},
	{"numeric field", "price", `{"price":42.5}`, float64(42.5)},
	{"bool field true", "active", `{"active":true}`, true},
	{"bool field false", "active", `{"active":false}`, false},
	{"missing field", "x.y", `{"a":1}`, nil},

	// --- Comparison ---
	{`eq string true`, `user.role = "admin"`, `{"user":{"role":"admin"}}`, true},
	{`eq string false`, `user.role = "admin"`, `{"user":{"role":"user"}}`, false},
	{`neq string true`, `status != "inactive"`, `{"status":"active"}`, true},
	{`neq string false`, `status != "inactive"`, `{"status":"inactive"}`, false},

	// --- $exists ---
	{"exists present", "$exists(user.email)", `{"user":{"email":"x"}}`, true},
	{"exists missing", "$exists(user.email)", `{"user":{}}`, false},
	{"exists null", "$exists(val)", `{"val":null}`, true},

	// --- $not ---
	{"not false", "$not(flag)", `{"flag":false}`, true},
	{"not true", "$not(flag)", `{"flag":true}`, false},
	{"not empty string", "$not(s)", `{"s":""}`, true},

	// --- $lowercase / $uppercase ---
	{"lowercase", "$lowercase(name)", `{"name":"HELLO"}`, "hello"},
	{"uppercase", "$uppercase(name)", `{"name":"hello"}`, "HELLO"},

	// --- $trim ---
	{"trim", "$trim(s)", `{"s":"  hello  world  "}`, "hello world"},

	// --- $length ---
	{"length string", "$length(s)", `{"s":"hello"}`, float64(5)},
	{"length utf8", "$length(s)", `{"s":"héllo"}`, float64(5)},

	// --- $type ---
	{"type string", "$type(v)", `{"v":"x"}`, "string"},
	{"type number", "$type(v)", `{"v":1}`, "number"},
	{"type boolean", "$type(v)", `{"v":true}`, "boolean"},
	{"type null", "$type(v)", `{"v":null}`, "null"},
	{"type array", "$type(v)", `{"v":[1,2]}`, "array"},
	{"type object", "$type(v)", `{"v":{"a":1}}`, "object"},

	// --- $boolean ---
	{"boolean true val", "$boolean(v)", `{"v":1}`, true},
	{"boolean false val", "$boolean(v)", `{"v":0}`, false},
	{"boolean empty str", "$boolean(v)", `{"v":""}`, false},
	{"boolean nonempty str", "$boolean(v)", `{"v":"x"}`, true},

	// --- $string ---
	{"string from number int", "$string(v)", `{"v":42}`, "42"},
	{"string from bool", "$string(v)", `{"v":true}`, "true"},

	// --- $number ---
	{"number from string", "$number(v)", `{"v":"3.14"}`, float64(3.14)},
	{"number from number", "$number(v)", `{"v":7}`, float64(7)},

	// --- $count ---
	{"count array", "$count(arr)", `{"arr":[1,2,3]}`, float64(3)},
	{"count scalar", "$count(v)", `{"v":42}`, float64(1)},

	// --- $sum ---
	{"sum", "$sum(arr)", `{"arr":[1,2,3,4]}`, float64(10)},

	// --- $max / $min ---
	{"max", "$max(arr)", `{"arr":[3,1,4,1,5,9]}`, float64(9)},
	{"min", "$min(arr)", `{"arr":[3,1,4,1,5,9]}`, float64(1)},

	// --- $average ---
	{"average", "$average(arr)", `{"arr":[1,2,3]}`, float64(2)},

	// --- $abs / $floor / $ceil / $sqrt ---
	{"abs negative", "$abs(v)", `{"v":-5}`, float64(5)},
	{"floor", "$floor(v)", `{"v":2.9}`, float64(2)},
	{"ceil", "$ceil(v)", `{"v":2.1}`, float64(3)},
	{"sqrt", "$sqrt(v)", `{"v":4}`, float64(2)},

	// --- $keys ---
	{"keys", "$keys(obj)", `{"obj":{"a":1,"b":2}}`, []interface{}{"a", "b"}},

	// --- $distinct ---
	{"distinct", "$distinct(arr)", `{"arr":[1,2,1,3]}`, []interface{}{float64(1), float64(2), float64(3)}},

	// --- $reverse ---
	{"reverse", "$reverse(arr)", `{"arr":[1,2,3]}`, []interface{}{float64(3), float64(2), float64(1)}},

	// --- $contains ---
	{`contains true`, `$contains(email, "@")`, `{"email":"test@example.com"}`, true},
	{`contains false`, `$contains(email, "@")`, `{"email":"testexample.com"}`, false},
}

func TestEvalBytes(t *testing.T) {
	ev := evaluator.New()
	ctx := context.Background()

	for _, tc := range evalBytesCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			expr, err := parser.Compile(tc.query)
			if err != nil {
				t.Fatalf("Compile(%q): %v", tc.query, err)
			}

			got, err := ev.EvalBytes(ctx, expr, json.RawMessage(tc.json))
			if err != nil {
				t.Fatalf("EvalBytes: %v", err)
			}

			// Normalize types.Null → nil for comparison.
			if _, isNull := got.(types.Null); isNull {
				got = nil
			}

			deepEqual(t, tc.want, got)
		})
	}
}

// TestEvalBytes_MatchesEval verifies that EvalBytes produces the same result
// as the full-eval path (via json.Unmarshal + Eval) for all fast-path cases.
func TestEvalBytes_MatchesEval(t *testing.T) {
	ev := evaluator.New()
	ctx := context.Background()

	for _, tc := range evalBytesCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			expr, err := parser.Compile(tc.query)
			if err != nil {
				t.Fatalf("Compile(%q): %v", tc.query, err)
			}

			raw := json.RawMessage(tc.json)

			// Full eval via json.Unmarshal.
			var data interface{}
			if err := json.Unmarshal(raw, &data); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			wantFull, errFull := ev.Eval(ctx, expr, data)
			if errFull != nil {
				t.Fatalf("Eval: %v", errFull)
			}

			// Fast-path eval.
			gotFast, errFast := ev.EvalBytes(ctx, expr, raw)
			if errFast != nil {
				t.Fatalf("EvalBytes: %v", errFast)
			}

			deepEqual(t, wantFull, gotFast)
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

var benchJSON = json.RawMessage(`{
	"user": {"email": "test@example.com", "role": "admin", "active": true},
	"price": 99.99,
	"tags": ["go","json","fast"],
	"count": 42
}`)

func BenchmarkEvalBytes_PurePath(b *testing.B) {
	ev := evaluator.New()
	expr, _ := parser.Compile("user.email")
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ev.EvalBytes(ctx, expr, benchJSON)
	}
}

func BenchmarkEval_PurePath_WithUnmarshal(b *testing.B) {
	ev := evaluator.New()
	expr, _ := parser.Compile("user.email")
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var data interface{}
		_ = json.Unmarshal(benchJSON, &data)
		_, _ = ev.Eval(ctx, expr, data)
	}
}

func BenchmarkEvalBytes_Comparison(b *testing.B) {
	ev := evaluator.New()
	expr, _ := parser.Compile(`user.role = "admin"`)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ev.EvalBytes(ctx, expr, benchJSON)
	}
}

func BenchmarkEvalBytes_Exists(b *testing.B) {
	ev := evaluator.New()
	expr, _ := parser.Compile("$exists(user.email)")
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ev.EvalBytes(ctx, expr, benchJSON)
	}
}

func BenchmarkEvalBytes_Sum(b *testing.B) {
	ev := evaluator.New()
	expr, _ := parser.Compile("$sum(tags)")
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ev.EvalBytes(ctx, expr, benchJSON)
	}
}

// ---------------------------------------------------------------------------
// helper
// ---------------------------------------------------------------------------

// deepEqual compares want and got, handling slice ordering for $keys/$distinct.
func deepEqual(t *testing.T, want, got interface{}) {
	t.Helper()

	// Normalize types.Null → nil.
	if _, isNull := got.(types.Null); isNull {
		got = nil
	}
	if _, isNull := want.(types.Null); isNull {
		want = nil
	}

	wJSON, _ := json.Marshal(want)
	gJSON, _ := json.Marshal(got)

	if string(wJSON) != string(gJSON) {
		t.Errorf("want %s, got %s", wJSON, gJSON)
	}
}
