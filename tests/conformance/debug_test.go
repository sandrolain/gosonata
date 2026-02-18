package conformance_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/parser"
)

func TestDebugSift(t *testing.T) {
	data, _ := os.ReadFile("node_modules/jsonata/test/test-suite/datasets/dataset1.json")
	var input interface{}
	json.Unmarshal(data, &input)

	eval := evaluator.New()
	ctx := context.Background()

	for _, expr := range []string{`**[*]`, `**[*].$sift(λ($v){$v.Postcode})`} {
		ast, err := parser.Parse(expr)
		if err != nil {
			t.Logf("compile error for %s: %v", expr, err)
			continue
		}
		result, err := eval.EvalWithBindings(ctx, ast, input, nil)
		if err != nil {
			t.Logf("eval error for %s: %v", expr, err)
			continue
		}
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("=== %s ===\n%s\n\n", expr, string(out))
	}
}
