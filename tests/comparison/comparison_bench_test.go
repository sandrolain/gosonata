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
	cmpSmallData = map[string]interface{}{
		"name": "John Doe", "age": 30, "active": true, "score": 95.5,
	}
	cmpMediumData = build(10)
	cmpLargeData = build(100)
	cmpXLData = build(1000)
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
	expr := mustCmpParse("$.users[age > 30].name")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sharedCmpEval.Eval(ctx, expr, cmpMediumData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGoSonata_Filter_Large(b *testing.B) {
	expr := mustCmpParse("$.users[age > 30 and department = 'Engineering'].name")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sharedCmpEval.Eval(ctx, expr, cmpLargeData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGoSonata_Filter_XL(b *testing.B) {
	expr := mustCmpParse("$.users[age > 30 and department = 'Engineering'].name")
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
	expr := mustCmpParse("$sum($.users[active = true].salary)")
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
		{"Filter/medium", "$.users[age > 30].name", cmpMediumData, 20000, 500},
		{"Filter/large", "$.users[age > 30 and department = 'Engineering'].name", cmpLargeData, 5000, 200},
		{"Filter/xl", "$.users[age > 30 and department = 'Engineering'].name", cmpXLData, 1000, 100},
		{"Aggregation/medium", "$sum($.users.salary)", cmpMediumData, 20000, 500},
		{"Aggregation/large", "$sum($.users[active = true].salary)", cmpLargeData, 5000, 200},
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
