package unit_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/parser"
)

func TestCustomFunctionBasic(t *testing.T) {
	greet := func(ctx context.Context, args ...interface{}) (interface{}, error) {
		name, _ := args[0].(string)
		return "Hello, " + name + "!", nil
	}
	expr, err := parser.Compile(`$greet($.name)`)
	if err != nil {
		t.Fatal(err)
	}
	ev := evaluator.New(evaluator.WithCustomFunction("greet", "", greet))
	result, err := ev.Eval(context.Background(), expr, map[string]interface{}{"name": "World"})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := result.(string); !ok || got != "Hello, World!" {
		t.Fatalf(`expected "Hello, World!", got %v`, result)
	}
}

func TestCustomFunctionMultipleArgs(t *testing.T) {
	add := func(ctx context.Context, args ...interface{}) (interface{}, error) {
		a, _ := args[0].(float64)
		b, _ := args[1].(float64)
		return a + b, nil
	}
	expr, err := parser.Compile(`$add(3, 4)`)
	if err != nil {
		t.Fatal(err)
	}
	ev := evaluator.New(evaluator.WithCustomFunction("add", "", add))
	result, err := ev.Eval(context.Background(), expr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := result.(float64); !ok || got != 7 {
		t.Fatalf("expected 7, got %v", result)
	}
}

func TestCustomFunctionReturnsError(t *testing.T) {
	fail := func(ctx context.Context, args ...interface{}) (interface{}, error) {
		return nil, fmt.Errorf("intentional error")
	}
	expr, err := parser.Compile(`$fail()`)
	if err != nil {
		t.Fatal(err)
	}
	ev := evaluator.New(evaluator.WithCustomFunction("fail", "", fail))
	_, err = ev.Eval(context.Background(), expr, nil)
	if err == nil {
		t.Fatal("expected error from custom function")
	}
}

func TestCustomFunctionMultipleRegistrations(t *testing.T) {
	double := func(ctx context.Context, args ...interface{}) (interface{}, error) {
		n, _ := args[0].(float64)
		return n * 2, nil
	}
	square := func(ctx context.Context, args ...interface{}) (interface{}, error) {
		n, _ := args[0].(float64)
		return n * n, nil
	}
	expr, err := parser.Compile(`$double($square(3))`)
	if err != nil {
		t.Fatal(err)
	}
	ev := evaluator.New(
		evaluator.WithCustomFunction("double", "", double),
		evaluator.WithCustomFunction("square", "", square),
	)
	result, err := ev.Eval(context.Background(), expr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := result.(float64); !ok || got != 18 {
		t.Fatalf("expected 18 (double(square(3)) = double(9) = 18), got %v", result)
	}
}

func TestCustomFunctionNotFoundWithoutRegistration(t *testing.T) {
	expr, err := parser.Compile(`$unregistered()`)
	if err != nil {
		t.Fatal(err)
	}
	ev := evaluator.New() // no custom functions registered
	_, err = ev.Eval(context.Background(), expr, nil)
	if err == nil {
		t.Fatal("expected error for unregistered function")
	}
}

func TestCustomFunctionContextPropagation(t *testing.T) {
	type ctxKey string
	key := ctxKey("testval")

	peek := func(ctx context.Context, args ...interface{}) (interface{}, error) {
		if v, ok := ctx.Value(key).(string); ok {
			return v, nil
		}
		return "missing", nil
	}
	expr, err := parser.Compile(`$peek()`)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), key, "injected")
	ev := evaluator.New(evaluator.WithCustomFunction("peek", "", peek))
	result, err := ev.Eval(ctx, expr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := result.(string); !ok || got != "injected" {
		t.Fatalf("expected context value to propagate, got %v", result)
	}
}
