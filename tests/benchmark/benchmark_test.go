// Package benchmark provides performance benchmarks for GoSonata.
//
// Run all benchmarks:
//
//	go test -bench=. -benchmem ./tests/benchmark/...
//
// Run specific category:
//
//	go test -bench=BenchmarkParse -benchmem ./tests/benchmark/...
//	go test -bench=BenchmarkEval -benchmem ./tests/benchmark/...
package benchmark_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/parser"
	"github.com/sandrolain/gosonata/pkg/types"
)

// ---------------------------------------------------------------------------
// Test data
// ---------------------------------------------------------------------------

var (
	// smallData - ~100 bytes
	smallData = map[string]interface{}{
		"name":   "John Doe",
		"age":    30,
		"active": true,
		"score":  95.5,
	}

	// mediumData - ~1 KB, 10 users
	mediumData interface{}

	// largeData - ~10 KB, 100 users
	largeData interface{}

	// xlData - ~100 KB, 1000 users
	xlData interface{}

	// serialized JSON versions
	smallJSON  []byte
	mediumJSON []byte
	largeJSON  []byte
)

func init() {
	departments := []string{"Engineering", "Sales", "Marketing", "HR", "Finance"}

	buildDataset := func(n int) interface{} {
		users := make([]map[string]interface{}, n)
		for i := 0; i < n; i++ {
			users[i] = map[string]interface{}{
				"id":         i + 1,
				"name":       fmt.Sprintf("User%d", i+1),
				"age":        20 + (i % 40),
				"department": departments[i%5],
				"salary":     70000 + (i * 1000),
				"active":     i%2 == 0,
				"projects": []string{
					fmt.Sprintf("Project%d", i),
					fmt.Sprintf("Project%d", i+1),
				},
			}
		}
		return map[string]interface{}{"users": users}
	}

	mediumData = buildDataset(10)
	largeData = buildDataset(100)
	xlData = buildDataset(1000)

	smallJSON, _ = json.Marshal(smallData)
	mediumJSON, _ = json.Marshal(mediumData)
	largeJSON, _ = json.Marshal(largeData)
}

// sharedEval is safe for concurrent use.
var sharedEval = evaluator.New()

func runEval(b *testing.B, expr *types.Expression, data interface{}) {
	b.Helper()
	ctx := context.Background()
	_, err := sharedEval.Eval(ctx, expr, data)
	if err != nil {
		b.Fatal(err)
	}
}

func mustParse(expr string) *types.Expression {
	e, err := parser.Parse(expr)
	if err != nil {
		panic(fmt.Sprintf("mustParse(%q): %v", expr, err))
	}
	return e
}

// ---------------------------------------------------------------------------
// Parser benchmarks
// ---------------------------------------------------------------------------

func BenchmarkParseSimplePath(b *testing.B) {
	expr := "$.name"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.Parse(expr)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseComplexPath(b *testing.B) {
	expr := "$.users[age > 30 and department = 'Engineering'].{name: name, salary: salary}"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.Parse(expr)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseWithFunctions(b *testing.B) {
	expr := "$sum(users[active = true].salary) / $count(users[active = true])"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.Parse(expr)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseNestedLambda(b *testing.B) {
	expr := `users.$filter(function($v) { $v.salary > $average($$[department = $v.department].salary) })`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.Parse(expr)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseTransformation(b *testing.B) {
	expr := `{
		"summary": {
			"totalUsers": $count(users),
			"departments": users{department: $count()},
			"avgSalary": $average(users.salary)
		}
	}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.Parse(expr)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Evaluation – simple path
// ---------------------------------------------------------------------------

func BenchmarkEvalSimplePath_Small(b *testing.B) {
	expr := mustParse("$.name")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, smallData)
	}
}

func BenchmarkEvalSimplePath_Medium(b *testing.B) {
	expr := mustParse("$.users[0].name")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, mediumData)
	}
}

// ---------------------------------------------------------------------------
// Evaluation – filter
// ---------------------------------------------------------------------------

func BenchmarkEvalFilter_Medium(b *testing.B) {
	expr := mustParse("$.users[age > 30].name")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, mediumData)
	}
}

func BenchmarkEvalFilter_Large(b *testing.B) {
	expr := mustParse("$.users[age > 30 and department = 'Engineering'].name")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, largeData)
	}
}

func BenchmarkEvalFilter_XL(b *testing.B) {
	expr := mustParse("$.users[age > 30 and department = 'Engineering'].name")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, xlData)
	}
}

// ---------------------------------------------------------------------------
// Evaluation – aggregation
// ---------------------------------------------------------------------------

func BenchmarkEvalAggregation_Medium(b *testing.B) {
	expr := mustParse("$sum($.users.salary)")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, mediumData)
	}
}

func BenchmarkEvalAggregation_Large(b *testing.B) {
	expr := mustParse("$sum($.users[active = true].salary)")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, largeData)
	}
}

func BenchmarkEvalAggregation_XL(b *testing.B) {
	expr := mustParse("$average($.users[department = 'Engineering'].salary)")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, xlData)
	}
}

// ---------------------------------------------------------------------------
// Evaluation – object transformation
// ---------------------------------------------------------------------------

func BenchmarkEvalTransform_Medium(b *testing.B) {
	expr := mustParse(`{
		"count": $count($.users),
		"avg": $average($.users.salary),
		"max": $max($.users.salary),
		"names": $.users.name
	}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, mediumData)
	}
}

func BenchmarkEvalTransform_Large(b *testing.B) {
	expr := mustParse(`{
		"count": $count($.users),
		"avg": $average($.users.salary),
		"byDept": $.users{department: $count()}
	}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, largeData)
	}
}

func BenchmarkEvalTransform_XL(b *testing.B) {
	expr := mustParse(`{
		"count": $count($.users),
		"avg": $average($.users.salary),
		"byDept": $.users{department: $count()}
	}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, xlData)
	}
}

// ---------------------------------------------------------------------------
// Evaluation – string operations
// ---------------------------------------------------------------------------

func BenchmarkEvalStringJoin(b *testing.B) {
	expr := mustParse("$join($.users.name, ', ')")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, mediumData)
	}
}

func BenchmarkEvalStringConcat(b *testing.B) {
	expr := mustParse("$.users.(name & ' (' & department & ')')")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, mediumData)
	}
}

// ---------------------------------------------------------------------------
// Evaluation – sorting
// ---------------------------------------------------------------------------

func BenchmarkEvalSort_Medium(b *testing.B) {
	expr := mustParse("$sort($.users, function($a, $b) { $a.salary > $b.salary })")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, mediumData)
	}
}

func BenchmarkEvalSort_Large(b *testing.B) {
	expr := mustParse("$sort($.users, function($a, $b) { $a.salary > $b.salary })")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, largeData)
	}
}

// ---------------------------------------------------------------------------
// Full pipeline (compile + eval)
// ---------------------------------------------------------------------------

func BenchmarkCompileAndEvalSimple(b *testing.B) {
	expr := "$.name"
	ev := evaluator.New()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p, err := parser.Parse(expr)
		if err != nil {
			b.Fatal(err)
		}
		_, err = ev.Eval(ctx, p, smallData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileAndEvalComplex(b *testing.B) {
	expr := "$.users[age > 30 and department = 'Engineering'].{name: name, salary: salary}"
	ev := evaluator.New()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p, err := parser.Parse(expr)
		if err != nil {
			b.Fatal(err)
		}
		_, err = ev.Eval(ctx, p, largeData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// JSON unmarshal + eval
// ---------------------------------------------------------------------------

func BenchmarkEvalFromJSON_Medium(b *testing.B) {
	expr := mustParse("$.users[age > 30].name")
	ev := evaluator.New()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var data interface{}
		if err := json.Unmarshal(mediumJSON, &data); err != nil {
			b.Fatal(err)
		}
		_, err := ev.Eval(ctx, expr, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvalFromJSON_Large(b *testing.B) {
	expr := mustParse("$.users[age > 30 and department = 'Engineering'].name")
	ev := evaluator.New()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var data interface{}
		if err := json.Unmarshal(largeJSON, &data); err != nil {
			b.Fatal(err)
		}
		_, err := ev.Eval(ctx, expr, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Arithmetic
// ---------------------------------------------------------------------------

func BenchmarkEvalArithmetic(b *testing.B) {
	expr := mustParse("(1 + 2) * 3 / 4 - 5 % 3")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, nil)
	}
}

func BenchmarkEvalArithmeticWithData(b *testing.B) {
	expr := mustParse("$.age * 2 + 10")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEval(b, expr, smallData)
	}
}

// ---------------------------------------------------------------------------
// Concurrent evaluation
// ---------------------------------------------------------------------------

func BenchmarkEvalConcurrent_Large(b *testing.B) {
	expr := mustParse("$.users[age > 30].name")
	ev := evaluator.New()
	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := ev.Eval(ctx, expr, largeData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEvalConcurrent_XL(b *testing.B) {
	expr := mustParse("$average($.users[department = 'Engineering'].salary)")
	ev := evaluator.New()
	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := ev.Eval(ctx, expr, xlData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
