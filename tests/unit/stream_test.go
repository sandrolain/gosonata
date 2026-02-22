package unit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/parser"
)

func TestEvalStreamNDJSON(t *testing.T) {
	ndjson := `{"name":"Alice","age":30}
{"name":"Bob","age":25}
{"name":"Charlie","age":35}`

	expr, err := parser.Compile("$.name")
	if err != nil {
		t.Fatal(err)
	}
	ev := evaluator.New()
	ch, err := ev.EvalStream(context.Background(), expr, strings.NewReader(ndjson))
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"Alice", "Bob", "Charlie"}
	i := 0
	for res := range ch {
		if res.Err != nil {
			t.Fatalf("result[%d]: unexpected error: %v", i, res.Err)
		}
		if got, ok := res.Value.(string); !ok || got != want[i] {
			t.Errorf("result[%d]: got %v, want %v", i, res.Value, want[i])
		}
		i++
	}
	if i != len(want) {
		t.Fatalf("expected %d results, got %d", len(want), i)
	}
}

func TestEvalStreamEmpty(t *testing.T) {
	expr, _ := parser.Compile("$.x")
	ev := evaluator.New()
	ch, err := ev.EvalStream(context.Background(), expr, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for range ch {
		count++
	}
	if count != 0 {
		t.Fatalf("expected 0 results for empty input, got %d", count)
	}
}

func TestEvalStreamNilExpression(t *testing.T) {
	ev := evaluator.New()
	_, err := ev.EvalStream(context.Background(), nil, strings.NewReader("{}"))
	if err == nil {
		t.Fatal("expected error for nil expression")
	}
}

func TestEvalStreamContextCancellation(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 500; i++ {
		sb.WriteString("{\"n\":1}\n")
	}
	expr, _ := parser.Compile("$.n")
	ev := evaluator.New()
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := ev.EvalStream(ctx, expr, strings.NewReader(sb.String()))
	if err != nil {
		t.Fatal(err)
	}
	// Read one result then cancel; channel must eventually close.
	<-ch
	cancel()
	for range ch {
	}
}

func TestEvalStreamPerDocumentErrorContinues(t *testing.T) {
	// $number() on a non-numeric string should error per-doc, not stop the stream.
	ndjson := `{"v":42}
{"v":"not-a-number"}
{"v":7}`

	expr, err := parser.Compile("$number($.v)")
	if err != nil {
		t.Fatal(err)
	}
	ev := evaluator.New()
	ch, err := ev.EvalStream(context.Background(), expr, strings.NewReader(ndjson))
	if err != nil {
		t.Fatal(err)
	}
	var results []evaluator.StreamResult
	for r := range ch {
		results = append(results, r)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Errorf("result[0] should succeed: %v", results[0].Err)
	}
	if results[1].Err == nil {
		t.Errorf("result[1] should fail on non-numeric string")
	}
	if results[2].Err != nil {
		t.Errorf("result[2] should succeed: %v", results[2].Err)
	}
}

func TestEvalStreamSingleDocument(t *testing.T) {
	expr, _ := parser.Compile("$.x * 2")
	ev := evaluator.New()
	ch, err := ev.EvalStream(context.Background(), expr, strings.NewReader(`{"x":21}`))
	if err != nil {
		t.Fatal(err)
	}
	var results []evaluator.StreamResult
	for r := range ch {
		results = append(results, r)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	if got, ok := results[0].Value.(float64); !ok || got != 42 {
		t.Fatalf("expected 42, got %v", results[0].Value)
	}
}
