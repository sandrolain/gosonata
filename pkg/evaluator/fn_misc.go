package evaluator

import (
	"context"
	"fmt"

	"github.com/sandrolain/gosonata/pkg/parser"
)

func fnError(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	message := "$error() function evaluated"
	if len(args) > 0 && args[0] != nil {
		message = fmt.Sprint(args[0])
	}
	return nil, fmt.Errorf("D3137: %s", message)
}

// fnAssert asserts a condition, throws error if false.
// Signature: $assert(condition [, message])
// The condition must be a boolean; null and numbers return T0410 error

func fnAssert(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("T0410: $assert() requires at least 1 argument")
	}

	// Validate that first argument is a boolean
	// null and numbers are not valid conditions
	if args[0] != nil {
		if _, ok := args[0].(bool); !ok {
			// Non-boolean values are not valid conditions
			return nil, fmt.Errorf("T0410: $assert() requires condition to be boolean")
		}
	} else {
		// null is not a valid condition
		return nil, fmt.Errorf("T0410: $assert() requires condition to be boolean")
	}

	// At this point, args[0] is a boolean
	condition := args[0].(bool)

	// Extract message
	message := "$assert() statement failed"
	if len(args) > 1 && args[1] != nil {
		message = fmt.Sprint(args[1])
	}

	if !condition {
		return nil, fmt.Errorf("D3141: %s", message)
	}
	return nil, nil
}

// --- Regex Functions ---

// fnMatch finds regex matches and returns array of match objects.
// Signature: $match(str, pattern [, limit])

func fnEval(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// Undefined input → undefined
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}

	exprStr, ok := args[0].(string)
	if !ok {
		return nil, nil
	}

	// Parse the expression string
	parsed, err := parser.Parse(exprStr)
	if err != nil {
		return nil, err
	}

	// If bindings/context are provided as second arg, use as data context
	if len(args) >= 2 && args[1] != nil {
		// Second argument is the data context for the evaluated expression
		return e.Eval(ctx, parsed, args[1])
	}

	// Evaluate in the current data context, inheriting current bindings
	return e.Eval(ctx, parsed, evalCtx.Data())
}
