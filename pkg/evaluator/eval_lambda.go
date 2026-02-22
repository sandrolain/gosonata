package evaluator

import (
	"context"
	"fmt"
)

func (e *Evaluator) callLambda(ctx context.Context, lambda *Lambda, args []interface{}) (interface{}, error) {
	// Check for undefined arguments - if any argument is undefined (nil),
	// the result is undefined per JSONata spec
	for _, arg := range args {
		if arg == nil {
			return nil, nil // undefined propagates
		}
	}

	// Validate signature if present
	if lambda.Signature != nil {
		// Count required parameters (non-optional)
		requiredCount := 0
		for _, param := range lambda.Signature.Params {
			if !param.Optional {
				requiredCount++
			}
		}

		// Check argument count (must be between required and total params)
		if len(args) < requiredCount || len(args) > len(lambda.Signature.Params) {
			if requiredCount == len(lambda.Signature.Params) {
				return nil, fmt.Errorf("lambda expects %d arguments, got %d", len(lambda.Signature.Params), len(args))
			} else {
				return nil, fmt.Errorf("lambda expects %d-%d arguments, got %d", requiredCount, len(lambda.Signature.Params), len(args))
			}
		}

		// Apply auto-wrapping and validate each argument
		for i := range args {
			param := lambda.Signature.Params[i]

			// Auto-wrap: if parameter expects array but arg is not array, wrap it
			if param.Type == TypeArray {
				if _, isArray := args[i].([]interface{}); !isArray {
					args[i] = []interface{}{args[i]}
				}
			}

			// Validate argument against parameter type
			if err := param.ValidateArgument(args[i]); err != nil {
				return nil, err
			}
		}
	} else {
		// No signature - validate argument count: allow fewer args (missing ones default to nil)
		if len(args) > len(lambda.Params) {
			return nil, fmt.Errorf("lambda expects %d arguments, got %d", len(lambda.Params), len(args))
		}
	}

	// Create new context with lambda's closure context as parent.
	// Clone (not CloneDeeper) - recursion depth is tracked via the shared *int pointer in context.
	lambdaCtx := lambda.Ctx.Clone()

	// Bind parameters
	for i, param := range lambda.Params {
		if i < len(args) {
			lambdaCtx.SetBinding(param, args[i])
		}
		// Optional parameters without args remain unbound
	}

	// Evaluate body using TCO trampolining.
	// We mark the body context as "tail position" so that tail calls (lambda calls in
	// tail position) return a tcoThunk instead of recursing. The trampoline loop below
	// then re-executes without growing the Go call stack or the depth counter.
	tcoCtx := withTCOTail(ctx)
	var result interface{}
	var err error
	for {
		result, err = e.evalNode(tcoCtx, lambda.Body, lambdaCtx)
		if err != nil {
			return nil, err
		}
		thunk, isThunk := result.(*tcoThunk)
		if !isThunk {
			break
		}
		// Trampoline: re-bind parameters and re-evaluate body without growing the stack.
		lambda = thunk.lambda
		args = thunk.args
		lambdaCtx = lambda.Ctx.Clone()
		for i, param := range lambda.Params {
			if i < len(args) {
				lambdaCtx.SetBinding(param, args[i])
			}
		}
	}
	return result, nil
}

// validateLambdaArgs validates argument count for a lambda (used before creating a TCO thunk).

func (e *Evaluator) validateLambdaArgs(lambda *Lambda, args []interface{}) error {
	if lambda.Signature != nil {
		requiredCount := 0
		for _, param := range lambda.Signature.Params {
			if !param.Optional {
				requiredCount++
			}
		}
		if len(args) < requiredCount || len(args) > len(lambda.Signature.Params) {
			if requiredCount == len(lambda.Signature.Params) {
				return fmt.Errorf("lambda expects %d arguments, got %d", len(lambda.Signature.Params), len(args))
			}
			return fmt.Errorf("lambda expects %d-%d arguments, got %d", requiredCount, len(lambda.Signature.Params), len(args))
		}
	} else {
		if len(args) > len(lambda.Params) {
			return fmt.Errorf("lambda expects %d arguments, got %d", len(lambda.Params), len(args))
		}
	}
	return nil
}

// validateAndAdaptLambdaArgs performs full signature validation including auto-wrapping.
// args slice is mutated in-place (auto-wrapping may change element types).

func (e *Evaluator) validateAndAdaptLambdaArgs(lambda *Lambda, args []interface{}) error {
	if lambda.Signature != nil {
		requiredCount := 0
		for _, param := range lambda.Signature.Params {
			if !param.Optional {
				requiredCount++
			}
		}
		if len(args) < requiredCount || len(args) > len(lambda.Signature.Params) {
			if requiredCount == len(lambda.Signature.Params) {
				return fmt.Errorf("lambda expects %d arguments, got %d", len(lambda.Signature.Params), len(args))
			}
			return fmt.Errorf("lambda expects %d-%d arguments, got %d", requiredCount, len(lambda.Signature.Params), len(args))
		}
		for i := range args {
			if i >= len(lambda.Signature.Params) {
				break
			}
			param := lambda.Signature.Params[i]
			if param.Type == TypeArray {
				if _, isArray := args[i].([]interface{}); !isArray {
					args[i] = []interface{}{args[i]}
				}
			}
			if err := param.ValidateArgument(args[i]); err != nil {
				return err
			}
		}
	} else {
		if len(args) > len(lambda.Params) {
			return fmt.Errorf("lambda expects %d arguments, got %d", len(lambda.Params), len(args))
		}
	}
	return nil
}

// Helper functions

// isTruthy determines if a value is truthy.
