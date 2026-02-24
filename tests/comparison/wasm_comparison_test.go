// Package comparison_test provides three-way benchmarks and correctness checks
// comparing GoSonata (native Go), JSONata JS (Node.js) and GoSonata WASM (js/wasm).
//
// # Run the three-way comparison report
//
//	go test -run TestTripleComparison -v -count=1 ./tests/comparison/...
//
// # Run WASM-specific correctness check
//
//	go test -run TestWASMCorrectness -v -count=1 ./tests/comparison/...
//
// # Skip if WASM binary is not built
//
// The WASM tests are skipped automatically when gosonata.wasm is not present;
// build it first with:
//
//	task wasm:build:js
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
)

// ── helpers ────────────────────────────────────────────────────────────────────

// wasmRunnerPath returns the absolute path to wasm_bench_runner.js.
func wasmRunnerPath(t testing.TB) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Join(filepath.Dir(thisFile), "wasm_bench_runner.js")
	}
	return filepath.Join("tests", "comparison", "wasm_bench_runner.js")
}

// wasmEvalRunnerPath returns the absolute path to wasm_eval_runner.js.
func wasmEvalRunnerPath(t testing.TB) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Join(filepath.Dir(thisFile), "wasm_eval_runner.js")
	}
	return filepath.Join("tests", "comparison", "wasm_eval_runner.js")
}

// wasmBinaryPath returns the path to gosonata.wasm (js/wasm build).
func wasmBinaryPath(t testing.TB) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Join(filepath.Dir(thisFile), "..", "..", "cmd", "wasm", "js", "gosonata.wasm")
	}
	return filepath.Join("cmd", "wasm", "js", "gosonata.wasm")
}

// wasmExecJSPath returns the path to the copied wasm_exec.js.
func wasmExecJSPath(t testing.TB) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Join(filepath.Dir(thisFile), "..", "..", "cmd", "wasm", "js", "wasm_exec.js")
	}
	return filepath.Join("cmd", "wasm", "js", "wasm_exec.js")
}

// skipIfNoWASM skips the test when gosonata.wasm or wasm_exec.js are not present.
func skipIfNoWASM(t testing.TB) {
	t.Helper()
	wasmBin := wasmBinaryPath(t)
	wasmExec := wasmExecJSPath(t)
	if _, err := os.Stat(wasmBin); err != nil {
		t.Skipf("gosonata.wasm not found (%s) — run: task wasm:build:js", wasmBin)
	}
	if _, err := os.Stat(wasmExec); err != nil {
		t.Skipf("wasm_exec.js not found (%s) — run: task wasm:copy-support:js", wasmExec)
	}
}

// runWASMBench invokes wasm_bench_runner.js and returns timing in ns/op.
func runWASMBench(t testing.TB, query string, data interface{}, iterations, warmup int) jsBenchResult {
	t.Helper()
	payload, err := json.Marshal(map[string]interface{}{
		"query": query, "data": data,
		"iterations": iterations, "warmup": warmup,
	})
	if err != nil {
		t.Fatalf("runWASMBench marshal: %v", err)
	}

	env := append(os.Environ(),
		"GOSONATA_WASM="+wasmBinaryPath(t),
		"WASM_EXEC_JS="+wasmExecJSPath(t),
	)
	cmd := exec.Command("node", wasmRunnerPath(t))
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Env = env
	out, execErr := cmd.Output()
	if execErr != nil {
		t.Fatalf("runWASMBench node exec: %v\noutput: %s", execErr, string(out))
	}
	var result jsBenchResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("runWASMBench unmarshal: %v\nraw: %s", err, string(out))
	}
	if !result.Success {
		t.Fatalf("runWASMBench WASM error: %s", result.Error)
	}
	return result
}

// runWASMEval evaluates query against data once in the WASM engine.
func runWASMEval(t testing.TB, query string, data interface{}) string {
	t.Helper()
	payload, err := json.Marshal(map[string]interface{}{"query": query, "data": data})
	if err != nil {
		t.Fatalf("runWASMEval marshal: %v", err)
	}
	env := append(os.Environ(),
		"GOSONATA_WASM="+wasmBinaryPath(t),
		"WASM_EXEC_JS="+wasmExecJSPath(t),
	)
	cmd := exec.Command("node", wasmEvalRunnerPath(t))
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Env = env
	out, execErr := cmd.Output()
	if execErr != nil {
		t.Fatalf("runWASMEval node exec: %v\noutput: %s", execErr, string(out))
	}
	var envelope struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
		Error   string          `json:"error"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("runWASMEval unmarshal: %v\nraw: %s", err, string(out))
	}
	if !envelope.Success {
		t.Fatalf("runWASMEval WASM error: %s", envelope.Error)
	}
	return string(envelope.Result)
}

// ── TestTripleComparison ───────────────────────────────────────────────────────

// TestTripleComparison prints a three-column performance comparison:
// GoSonata (native) | JSONata JS (Node.js) | GoSonata WASM (js/wasm via Node.js)
//
//	go test -run TestTripleComparison -v -count=1 ./tests/comparison/...
func TestTripleComparison(t *testing.T) {
	skipIfNoWASM(t)

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
		{"Aggregation/medium", "$sum($.users.salary)", cmpMediumData, 20000, 500},
		{"Aggregation/large", "$sum($.users.salary)", cmpLargeData, 5000, 200},
		{"Transform/medium", `{"count": $count($.users), "avg": $average($.users.salary), "names": $.users.name}`, cmpMediumData, 20000, 500},
		{"Transform/large", `{"count": $count($.users), "avg": $average($.users.salary), "departments": $distinct($.users.department)}`, cmpLargeData, 3000, 100},
		{"Sort/medium", "$sort($.users, function($a, $b) { $a.salary > $b.salary })", cmpMediumData, 10000, 300},
		{"Arithmetic", "(1 + 2) * 3 / 4 - 5 % 3", nil, 50000, 1000},
	}

	type row struct {
		name       string
		goNsPerOp  float64
		jsNsPerOp  float64
		wasmNsPerOp float64
		goVsJS     float64 // JS/Go — >1 means Go is faster
		goVsWASM   float64 // WASM/Go — >1 means Go is faster than WASM
		jsVsWASM   float64 // WASM/JS — >1 means JS is faster than WASM
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
			// Go warmup
			for i := 0; i < goWarmup; i++ {
				_, _ = ev.Eval(ctx, expr, c.data)
			}
			// Go timed
			start := time.Now()
			for i := 0; i < goIter; i++ {
				if _, err := ev.Eval(ctx, expr, c.data); err != nil {
					t.Fatalf("go eval: %v", err)
				}
			}
			goNs := float64(time.Since(start).Nanoseconds()) / float64(goIter)

			// JS
			js := runJSBench(t, c.query, c.data, c.jsIter, c.jsWarm)

			// WASM
			wasm := runWASMBench(t, c.query, c.data, c.jsIter/5, c.jsWarm/5)

			rows = append(rows, row{
				name:        c.name,
				goNsPerOp:   goNs,
				jsNsPerOp:   js.NsPerOp,
				wasmNsPerOp: wasm.NsPerOp,
				goVsJS:      js.NsPerOp / goNs,
				goVsWASM:    wasm.NsPerOp / goNs,
				jsVsWASM:    wasm.NsPerOp / js.NsPerOp,
			})
			t.Logf("Go: %.0f ns/op | JS: %.0f ns/op | WASM: %.0f ns/op",
				goNs, js.NsPerOp, wasm.NsPerOp)
		})
	}

	// ── Print report ──────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("+--------------------------------------------------------------------------------------------+")
	fmt.Println("|       GoSonata vs JSONata JS vs GoSonata WASM  —  three-way comparison                    |")
	fmt.Println("|  eval only (expr pre-compiled); Go: 5000 iters, JS/WASM: varies                           |")
	fmt.Println("+--------------------------------------------------------------------------------------------+")
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "Case\tGo ns/op\tJS ns/op\tWASM ns/op\tGo vs JS\tGo vs WASM\tJS vs WASM")
	fmt.Fprintln(tw, "----\t--------\t--------\t----------\t---------\t----------\t----------")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%.0f\t%.0f\t%.0f\t%.2fx\t%.2fx\t%.2fx\n",
			r.name,
			r.goNsPerOp, r.jsNsPerOp, r.wasmNsPerOp,
			r.goVsJS, r.goVsWASM, r.jsVsWASM,
		)
	}
	_ = tw.Flush()
	fmt.Println()
	fmt.Println("  Speedup > 1x means the left engine is faster than the right.")
	fmt.Println()
}

// ── TestWASMCorrectness ────────────────────────────────────────────────────────

// TestWASMCorrectness verifies that GoSonata WASM produces identical results
// to both GoSonata native and JSONata JS for all benchmark queries.
//
//	go test -run TestWASMCorrectness -v -count=1 ./tests/comparison/...
func TestWASMCorrectness(t *testing.T) {
	skipIfNoWASM(t)

	type checkCase struct {
		name  string
		query string
		data  interface{}
	}
	cases := []checkCase{
		{"SimplePath/small", "$.name", cmpSmallData},
		{"Filter/medium", "$.users[age > 25].name", cmpMediumData},
		{"Filter/large", "$.users[age > 25].name", cmpLargeData},
		{"Aggregation/medium", "$sum($.users.salary)", cmpMediumData},
		{"Aggregation/large", "$sum($.users.salary)", cmpLargeData},
		{"Transform/medium", `{"count": $count($.users), "avg": $average($.users.salary), "names": $.users.name}`, cmpMediumData},
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

			// Go native result
			goResult, err := ev.Eval(ctx, expr, c.data)
			if err != nil {
				t.Fatalf("go eval: %v", err)
			}
			goJSON, _ := json.Marshal(goResult)
			var goNorm interface{}
			_ = json.Unmarshal(goJSON, &goNorm)
			goFinal, _ := json.Marshal(goNorm)

			// JS result
			jsRaw := runJSEval(t, c.query, c.data)
			var jsNorm interface{}
			_ = json.Unmarshal([]byte(jsRaw), &jsNorm)
			jsFinal, _ := json.Marshal(jsNorm)

			// WASM result
			wasmRaw := runWASMEval(t, c.query, c.data)
			var wasmNorm interface{}
			_ = json.Unmarshal([]byte(wasmRaw), &wasmNorm)
			wasmFinal, _ := json.Marshal(wasmNorm)

			goStr := string(goFinal)
			jsStr := string(jsFinal)
			wasmStr := string(wasmFinal)

			ok := true
			if goStr != wasmStr {
				t.Errorf("Go ≠ WASM:\n  Go  : %s\n  WASM: %s", goStr, wasmStr)
				ok = false
			}
			if jsStr != wasmStr {
				t.Errorf("JS ≠ WASM:\n  JS  : %s\n  WASM: %s", jsStr, wasmStr)
				ok = false
			}
			if ok {
				t.Logf("OK (all three agree): %s", goStr)
			}
		})
	}
}
