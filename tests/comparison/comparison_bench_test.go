// Package comparison benchmarks GoSonata against the official JSONata JS implementation.
//
// Go benchmarks use the standard testing.B framework (pre-parsed expression).
// JS benchmarks invoke bench_runner.js which runs N iterations inside a single
// Node.js process, so startup cost is paid only once per query.
//
// Run comparison report:
//
//	go test -run TestJSComparison -v -count=1 ./tests/comparison/...
//
// Run Go-only benchmarks:
//
//	go test -bench=BenchmarkGoSonata -benchmem ./tests/comparison/...
package comparison_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/parser"
	"github.com/sandrolain/gosonata/pkg/types"
)

// ---------------------------------------------------------------------------
// Shared test data
// ---------------------------------------------------------------------------

var (
	cmpSmallData  interface{}
	cmpMediumData interface{}
	cmpLargeData  interface{}
	cmpXLData     interface{}
)

// jsonRoundTrip marshals v to JSON and back so all integer fields become float64.
// This is necessary because GoSonata's evaluator (like JSONata JS) expects numeric
// values in the same format produced by json.Unmarshal — i.e. float64, not int.
// Without this, comparisons like `age > 25` silently fail on native Go int data.
func jsonRoundTrip(v interface{}) interface{} {
	b, _ := json.Marshal(v)
	var out interface{}
	_ = json.Unmarshal(b, &out)
	return out
}

func init() {
	departments := []string{"Engineering", "Sales", "Marketing", "HR", "Finance"}
	build := func(n int) interface{} {
		users := make([]map[string]interface{}, n)
		for i := 0; i < n; i++ {
			users[i] = map[string]interface{}{
				"id":         i + 1,
				"name":       fmt.Sprintf("User%d", i+1),
				"age":        20 + (i % 40),
				"department": departments[i%5],
				"salary":     70000 + (i * 1000),
				"active":     i%2 == 0,
			}
		}
		return map[string]interface{}{"users": users}
	}
	// Round-trip through JSON so numeric fields are float64 (not int),
	// matching what both GoSonata and JSONata JS receive when data arrives as JSON.
	cmpSmallData = jsonRoundTrip(map[string]interface{}{
		"name": "John Doe", "age": 30, "active": true, "score": 95.5,
	})
	cmpMediumData = jsonRoundTrip(build(10))
	cmpLargeData = jsonRoundTrip(build(100))
	cmpXLData = jsonRoundTrip(build(1000))
}

// ---------------------------------------------------------------------------
// JS benchmark helper
// ---------------------------------------------------------------------------

type jsBenchResult struct {
	Success    bool    `json:"success"`
	NsPerOp    float64 `json:"nsPerOp"`
	OpsPerSec  float64 `json:"opsPerSec"`
	Iterations int     `json:"iterations"`
	Error      string  `json:"error"`
}

func runnerPath(t testing.TB) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(thisFile)
		p := filepath.Join(dir, "bench_runner.js")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join("tests", "comparison", "bench_runner.js")
}

func runJSBench(t testing.TB, query string, data interface{}, iterations, warmup int) jsBenchResult {
	t.Helper()
	payload, err := json.Marshal(map[string]interface{}{
		"query": query, "data": data,
		"iterations": iterations, "warmup": warmup,
	})
	if err != nil {
		t.Fatalf("runJSBench marshal: %v", err)
	}
	cmd := exec.Command("node", runnerPath(t))
	cmd.Stdin = bytes.NewReader(payload)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("runJSBench node exec: %v\noutput: %s", err, string(out))
	}
	var result jsBenchResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("runJSBench unmarshal: %v\nraw: %s", err, string(out))
	}
	if !result.Success {
		t.Fatalf("runJSBench JS error: %s", result.Error)
	}
	return result
}

// ---------------------------------------------------------------------------
// Go benchmark helpers
// ---------------------------------------------------------------------------

var sharedCmpEval = evaluator.New()

func mustCmpParse(expr string) *types.Expression {
	e, err := parser.Parse(expr)
	if err != nil {
		panic(fmt.Sprintf("mustCmpParse(%q): %v", expr, err))
	}
	return e
}

// ---------------------------------------------------------------------------
// Go benchmarks (testing.B)
// ---------------------------------------------------------------------------

func BenchmarkGoSonata_SimplePath_Small(b *testing.B) {
	expr := mustCmpParse("$.name")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sharedCmpEval.Eval(ctx, expr, cmpSmallData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGoSonata_Filter_Medium(b *testing.B) {
	expr := mustCmpParse("$.users[age > 25].name")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sharedCmpEval.Eval(ctx, expr, cmpMediumData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGoSonata_Filter_Large(b *testing.B) {
	expr := mustCmpParse("$.users[age > 25].name")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sharedCmpEval.Eval(ctx, expr, cmpLargeData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGoSonata_Filter_XL(b *testing.B) {
	expr := mustCmpParse("$.users[age > 25].name")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sharedCmpEval.Eval(ctx, expr, cmpXLData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGoSonata_Aggregation_Medium(b *testing.B) {
	expr := mustCmpParse("$sum($.users.salary)")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sharedCmpEval.Eval(ctx, expr, cmpMediumData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGoSonata_Aggregation_Large(b *testing.B) {
	expr := mustCmpParse("$sum($.users.salary)")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sharedCmpEval.Eval(ctx, expr, cmpLargeData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGoSonata_Transform_Medium(b *testing.B) {
	expr := mustCmpParse(`{"count": $count($.users), "avg": $average($.users.salary), "names": $.users.name}`)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sharedCmpEval.Eval(ctx, expr, cmpMediumData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGoSonata_Transform_Large(b *testing.B) {
	expr := mustCmpParse(`{"count": $count($.users), "avg": $average($.users.salary), "departments": $distinct($.users.department)}`)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sharedCmpEval.Eval(ctx, expr, cmpLargeData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGoSonata_Sort_Medium(b *testing.B) {
	expr := mustCmpParse("$sort($.users, function($a, $b) { $a.salary > $b.salary })")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sharedCmpEval.Eval(ctx, expr, cmpMediumData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGoSonata_Sort_Large(b *testing.B) {
	expr := mustCmpParse("$sort($.users, function($a, $b) { $a.salary > $b.salary })")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sharedCmpEval.Eval(ctx, expr, cmpLargeData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGoSonata_Arithmetic(b *testing.B) {
	expr := mustCmpParse("(1 + 2) * 3 / 4 - 5 % 3")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sharedCmpEval.Eval(ctx, expr, nil); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// JS comparison report
//
// Runs each case in Go (fixed 5000 iters) and JS (via bench_runner.js),
// then prints a comparison table.
//
//	go test -run TestJSComparison -v -count=1 ./tests/comparison/...
// ---------------------------------------------------------------------------

func TestJSComparison(t *testing.T) {
	type benchCase struct {
		name   string
		query  string
		data   interface{}
		jsIter int
		jsWarm int
	}

	cases := []benchCase{
		{"SimplePath/small", "$.name", cmpSmallData, 50000, 1000},
		{"Filter/medium", "$.users[age > 25].name", cmpMediumData, 20000, 500},
		{"Filter/large", "$.users[age > 25].name", cmpLargeData, 5000, 200},
		{"Filter/xl", "$.users[age > 25].name", cmpXLData, 1000, 100},
		{"Aggregation/medium", "$sum($.users.salary)", cmpMediumData, 20000, 500},
		{"Aggregation/large", "$sum($.users.salary)", cmpLargeData, 5000, 200},
		{"Transform/medium", `{"count": $count($.users), "avg": $average($.users.salary), "names": $.users.name}`, cmpMediumData, 20000, 500},
		{"Transform/large", `{"count": $count($.users), "avg": $average($.users.salary), "departments": $distinct($.users.department)}`, cmpLargeData, 3000, 100},
		{"Sort/medium", "$sort($.users, function($a, $b) { $a.salary > $b.salary })", cmpMediumData, 10000, 300},
		{"Sort/large", "$sort($.users, function($a, $b) { $a.salary > $b.salary })", cmpLargeData, 2000, 100},
		{"Arithmetic", "(1 + 2) * 3 / 4 - 5 % 3", nil, 50000, 1000},
	}

	type row struct {
		name      string
		goNsPerOp float64
		jsNsPerOp float64
		speedup   float64
	}

	const goIter = 5000
	const goWarmup = 200

	ev := evaluator.New()
	ctx := context.Background()

	var rows []row

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expr, err := parser.Parse(c.query)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			for i := 0; i < goWarmup; i++ {
				_, _ = ev.Eval(ctx, expr, c.data)
			}
			start := time.Now()
			for i := 0; i < goIter; i++ {
				if _, err := ev.Eval(ctx, expr, c.data); err != nil {
					t.Fatalf("go eval: %v", err)
				}
			}
			gons := float64(time.Since(start).Nanoseconds()) / float64(goIter)

			js := runJSBench(t, c.query, c.data, c.jsIter, c.jsWarm)
			speedup := js.NsPerOp / gons

			rows = append(rows, row{c.name, gons, js.NsPerOp, speedup})
			t.Logf("Go: %.0f ns/op  |  JS: %.0f ns/op  |  speedup: %.2fx", gons, js.NsPerOp, speedup)
		})
	}

	fmt.Println()
	fmt.Println("+----------------------------------------------------------------------------+")
	fmt.Println("|       GoSonata vs JSONata JS  --  comparison benchmark                     |")
	fmt.Println("|  eval only (expr pre-compiled); Go: 5000 iters, JS: varies                 |")
	fmt.Println("+----------------------------------------------------------------------------+")
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "Case\tGoSonata ns/op\tJS ns/op\tSpeedup (Go vs JS)")
	fmt.Fprintln(tw, "----\t--------------\t--------\t------------------")
	for _, r := range rows {
		dir := "faster"
		if r.speedup < 1 {
			dir = "slower"
		}
		fmt.Fprintf(tw, "%s\t%.0f\t%.0f\t%.2fx %s\n",
			r.name, r.goNsPerOp, r.jsNsPerOp, r.speedup, dir)
	}
	tw.Flush()
	fmt.Println()
}

// ---------------------------------------------------------------------------
// Result correctness verification
//
// Ensures GoSonata and JSONata JS produce semantically identical output for
// every benchmarked query. This guards against spurious speedups caused by
// GoSonata returning wrong (e.g. null) results faster than JS returns correct ones.
//
//	go test -run TestResultCorrectness -v -count=1 ./tests/comparison/...
// ---------------------------------------------------------------------------

func evalRunnerPath(t testing.TB) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(thisFile)
		p := filepath.Join(dir, "eval_runner.js")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join("tests", "comparison", "eval_runner.js")
}

// runJSEval evaluates query against data once in the JS engine and returns the
// JSON-serialised result (undefined → null).
func runJSEval(t testing.TB, query string, data interface{}) string {
	t.Helper()
	payload, err := json.Marshal(map[string]interface{}{"query": query, "data": data})
	if err != nil {
		t.Fatalf("runJSEval marshal: %v", err)
	}
	cmd := exec.Command("node", evalRunnerPath(t))
	cmd.Stdin = bytes.NewReader(payload)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("runJSEval node exec: %v\noutput: %s", err, string(out))
	}
	var envelope struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
		Error   string          `json:"error"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("runJSEval unmarshal: %v\nraw: %s", err, string(out))
	}
	if !envelope.Success {
		t.Fatalf("runJSEval JS error: %s", envelope.Error)
	}
	return string(envelope.Result)
}

func TestResultCorrectness(t *testing.T) {
	type checkCase struct {
		name  string
		query string
		data  interface{}
	}
	cases := []checkCase{
		{"SimplePath/small", "$.name", cmpSmallData},
		{"Filter/medium", "$.users[age > 25].name", cmpMediumData},
		{"Filter/large", "$.users[age > 25].name", cmpLargeData},
		{"Filter/xl", "$.users[age > 25].name", cmpXLData},
		{"Aggregation/medium", "$sum($.users.salary)", cmpMediumData},
		{"Aggregation/large", "$sum($.users.salary)", cmpLargeData},
		{"Transform/medium", `{"count": $count($.users), "avg": $average($.users.salary), "names": $.users.name}`, cmpMediumData},
		{"Transform/large", `{"count": $count($.users), "avg": $average($.users.salary), "departments": $distinct($.users.department)}`, cmpLargeData},
		{"Sort/medium", "$sort($.users, function($a, $b) { $a.salary > $b.salary })", cmpMediumData},
		{"Arithmetic", "(1 + 2) * 3 / 4 - 5 % 3", nil},
	}

	ev := evaluator.New()
	ctx := context.Background()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expr, err := parser.Parse(c.query)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			goResult, err := ev.Eval(ctx, expr, c.data)
			if err != nil {
				t.Fatalf("go eval error: %v", err)
			}

			// Normalise Go result through JSON round-trip so types are comparable.
			goJSON, err := json.Marshal(goResult)
			if err != nil {
				t.Fatalf("marshal go result: %v", err)
			}
			var goNorm interface{}
			_ = json.Unmarshal(goJSON, &goNorm)

			jsRaw := runJSEval(t, c.query, c.data)
			var jsNorm interface{}
			if err := json.Unmarshal([]byte(jsRaw), &jsNorm); err != nil {
				t.Fatalf("unmarshal js result: %v", err)
			}

			goFinal, _ := json.Marshal(goNorm)
			jsFinal, _ := json.Marshal(jsNorm)

			if string(goFinal) != string(jsFinal) {
				t.Errorf("result mismatch:\n  GoSonata : %s\n  JSONata JS: %s", string(goFinal), string(jsFinal))
			} else {
				t.Logf("OK: %s", string(goFinal))
			}
		})
	}
}
