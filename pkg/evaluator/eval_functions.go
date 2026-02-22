package evaluator

import (
	"context"
	"fmt"

	"github.com/sandrolain/gosonata/pkg/types"
)

func (e *Evaluator) evalFunction(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Check if this is a lambda/variable call (LHS contains lambda or variable) or built-in function call (Value contains name)
	if node.LHS != nil {
		// Lambda or variable call: evaluate first, then call it.
		// Arguments must NOT themselves be in tail position.
		callCtx := withoutTCOTail(ctx)
		callableValue, err := e.evalNode(callCtx, node.LHS, evalCtx)
		if err != nil {
			return nil, err
		}

		// Check what we got
		switch fn := callableValue.(type) {
		case *Lambda:
			// User-defined lambda
			// Evaluate arguments (never in tail position)
			args := make([]interface{}, 0, len(node.Arguments))
			for _, argNode := range node.Arguments {
				arg, err := e.evalNode(callCtx, argNode, evalCtx)
				if err != nil {
					return nil, err
				}
				// Unwrap contextBoundValues before passing to lambdas
				arg = unwrapCVsDeep(arg)
				args = append(args, arg)
			}

			// TCO: if we are in tail position, apply signature validation and return a
			// thunk instead of recursing. The callLambda trampoline will re-execute the
			// body without growing the stack.
			if isTCOTail(ctx) {
				// Apply full signature validation (including auto-wrapping) before thunk.
				if err2 := e.validateAndAdaptLambdaArgs(fn, args); err2 != nil {
					return nil, err2
				}
				return &tcoThunk{lambda: fn, args: args}, nil
			}

			// Normal call
			return e.callLambda(ctx, fn, args)

		case *FunctionDef:
			// Built-in function (from variable like $not)
			// Evaluate arguments
			args := make([]interface{}, 0, len(node.Arguments))
			for _, argNode := range node.Arguments {
				arg, err := e.evalNode(callCtx, argNode, evalCtx)
				if err != nil {
					return nil, err
				}
				// Unwrap contextBoundValues before passing to built-in functions
				arg = unwrapCVsDeep(arg)
				args = append(args, arg)
			}

			// If function accepts context and we have fewer args than required, prepend context
			if fn.AcceptsContext && len(args) < fn.MinArgs {
				contextData := evalCtx.Data()
				args = append([]interface{}{contextData}, args...)
			}

			// Validate argument count
			if len(args) < fn.MinArgs {
				return nil, types.NewError(types.ErrArgumentCountMismatch,
					fmt.Sprintf("function requires at least %d arguments, got %d", fn.MinArgs, len(args)), -1)
			}
			if fn.MaxArgs != -1 && len(args) > fn.MaxArgs {
				return nil, types.NewError(types.ErrArgumentCountMismatch,
					fmt.Sprintf("function accepts at most %d arguments, got %d", fn.MaxArgs, len(args)), -1)
			}

			// Call function
			return fn.Impl(ctx, e, evalCtx, args)

		default:
			// callableValue is nil when the variable is not bound in the eval context.
			// This happens when a custom or built-in function is called via its $name
			// syntax (e.g. $greet(...)) but "greet" has no variable binding.
			// Fall through to the custom/built-in lookup using the variable name.
			if callableValue == nil && node.LHS != nil && node.LHS.Type == types.NodeVariable {
				varName, ok := node.LHS.Value.(string)
				if ok {
					if fnDef, found := e.getCustomFunction(varName); found {
						args := make([]interface{}, 0, len(node.Arguments))
						for _, argNode := range node.Arguments {
							arg, err := e.evalNode(callCtx, argNode, evalCtx)
							if err != nil {
								return nil, err
							}
							arg = unwrapCVsDeep(arg)
							args = append(args, arg)
						}
						return fnDef.Impl(ctx, e, evalCtx, args)
					}
				}
			}
			return nil, fmt.Errorf("expected lambda or function, got %T", callableValue)
		}
	}

	// Built-in / custom function call
	funcName := node.Value.(string)

	// Check custom (user-registered) functions first, then built-ins.
	fnDef, ok := e.getCustomFunction(funcName)
	if !ok {
		fnDef, ok = GetFunction(funcName)
	}
	if !ok {
		return nil, fmt.Errorf("unknown function: %s", funcName)
	}

	// Evaluate arguments
	args := make([]interface{}, 0, len(node.Arguments))
	for _, argNode := range node.Arguments {
		arg, err := e.evalNode(ctx, argNode, evalCtx)
		if err != nil {
			return nil, err
		}
		// Unwrap contextBoundValues: built-in functions must not see internal CV wrappers
		arg = unwrapCVsDeep(arg)
		args = append(args, arg)
	}

	// If function accepts context and we have fewer args than required, prepend context
	if fnDef.AcceptsContext && len(args) < fnDef.MinArgs {
		contextData := evalCtx.Data()
		args = append([]interface{}{contextData}, args...)
	}

	// Validate argument count
	if len(args) < fnDef.MinArgs {
		return nil, types.NewError(types.ErrArgumentCountMismatch,
			fmt.Sprintf("function %s requires at least %d arguments, got %d", funcName, fnDef.MinArgs, len(args)), -1)
	}
	if fnDef.MaxArgs != -1 && len(args) > fnDef.MaxArgs {
		return nil, types.NewError(types.ErrArgumentCountMismatch,
			fmt.Sprintf("function %s accepts at most %d arguments, got %d", funcName, fnDef.MaxArgs, len(args)), -1)
	}

	// Call function
	return fnDef.Impl(ctx, e, evalCtx, args)
}

// evalFunctionWithContextInjection evaluates a lambda call with optional context injection.
// This is used when a lambda is called in a path context (e.g., Age.function($x,$y){...}(arg))
// The contextValue is prepended to the arguments ONLY if the lambda needs more arguments.

func (e *Evaluator) evalFunctionWithContextInjection(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext, contextValue interface{}) (interface{}, error) {
	// node.LHS should be a lambda
	if node.LHS == nil || node.LHS.Type != types.NodeLambda {
		return nil, fmt.Errorf("expected lambda in function call with context injection")
	}

	// Evaluate lambda
	lambdaValue, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	lambda, ok := lambdaValue.(*Lambda)
	if !ok {
		return nil, fmt.Errorf("expected lambda function, got %T", lambdaValue)
	}

	// Evaluate explicit arguments
	explicitArgs := make([]interface{}, 0, len(node.Arguments))
	for _, argNode := range node.Arguments {
		arg, err := e.evalNode(ctx, argNode, evalCtx)
		if err != nil {
			return nil, err
		}
		explicitArgs = append(explicitArgs, arg)
	}

	// Determine if we need to inject context
	// Inject context value as first argument ONLY if we have fewer args than params
	var args []interface{}
	if len(explicitArgs) < len(lambda.Params) {
		// Need context injection
		args = make([]interface{}, 0, len(explicitArgs)+1)
		args = append(args, contextValue)
		args = append(args, explicitArgs...)
	} else {
		// Already have enough args, use them as-is
		args = explicitArgs
	}

	// Call lambda with (possibly injected) context
	return e.callLambda(ctx, lambda, args)
}

// evalLambda creates a lambda function value.

func (e *Evaluator) evalLambda(node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Extract parameter names from Arguments field
	params := make([]string, 0, len(node.Arguments))
	for _, argNode := range node.Arguments {
		if argNode.Type == types.NodeVariable {
			// Parameter is a variable like $x
			paramName := argNode.Value.(string)
			params = append(params, paramName)
		}
	}

	// Parse signature if present
	var sig *Signature
	if node.Signature != "" {
		parsedSig, err := ParseSignature(node.Signature)
		if err != nil {
			// Return S0401 error for invalid signature
			return nil, err
		}
		sig = parsedSig
	}

	// Create new context with lambda's closure context as parent.
	// We store evalCtx directly (not cloned) so that the lambda can see
	// bindings added AFTER lambda creation in the same block scope (enables recursion).
	// callLambda() creates its own clone of this context at call time.
	lambda := &Lambda{
		Params:    params,
		Body:      node.RHS, // Body is in RHS
		Ctx:       evalCtx,
		Signature: sig,
	}

	return lambda, nil
}

// evalPartial creates a partial application lambda.
// When a function is called with placeholder arguments (?), it returns a new
// lambda that accepts values for those placeholders.

func (e *Evaluator) evalPartial(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Count placeholders and build parameter list
	placeholderCount := 0
	for _, arg := range node.Arguments {
		if arg.Type == types.NodePlaceholder {
			placeholderCount++
		}
	}

	if placeholderCount == 0 {
		// No placeholders - should not happen, but treat as regular function call
		return e.evalFunction(ctx, node, evalCtx)
	}

	// Check if partial application is allowed
	// It's only allowed when calling through a variable/lambda (node.LHS != nil)
	// Direct function calls (node.Value is string) are not allowed
	if node.LHS == nil && node.Value != nil {
		// Direct function call with placeholder
		funcName, ok := node.Value.(string)
		if !ok {
			return nil, types.NewError("T1007", "partial application can only be applied to a function", node.Position)
		}

		// Check if function exists
		if _, exists := GetFunction(funcName); !exists {
			return nil, types.NewError("T1008", fmt.Sprintf("attempted partial application of unknown function: %s", funcName), node.Position)
		}

		// Function exists but partial application is not supported for direct calls
		return nil, types.NewError("T1007", "partial application can only be applied to a function", node.Position)
	}

	// When LHS is set, evaluate it to check if it's callable
	if node.LHS != nil {
		lhsVal, err := e.evalNode(ctx, node.LHS, evalCtx)
		if err != nil {
			return nil, err
		}
		switch lhsVal.(type) {
		case *Lambda, *FunctionDef:
			// OK, callable
		default:
			return nil, types.NewError("T1007", "partial application can only be applied to a function", node.Position)
		}
	}

	// Create parameter names for the lambda ($1, $2, $3, ...)
	params := make([]string, placeholderCount)
	for i := 0; i < placeholderCount; i++ {
		params[i] = fmt.Sprintf("%d", i+1)
	}

	// Build the body: a function call with placeholders replaced by variables
	bodyNode := types.NewASTNode(types.NodeFunction, node.Position)
	bodyNode.Value = node.Value
	bodyNode.LHS = node.LHS
	bodyNode.Arguments = make([]*types.ASTNode, len(node.Arguments))

	placeholderIndex := 0
	for i, arg := range node.Arguments {
		if arg.Type == types.NodePlaceholder {
			// Replace placeholder with variable reference
			varNode := types.NewASTNode(types.NodeVariable, arg.Position)
			varNode.Value = params[placeholderIndex]
			bodyNode.Arguments[i] = varNode
			placeholderIndex++
		} else {
			// Keep non-placeholder arguments as-is
			bodyNode.Arguments[i] = arg
		}
	}

	// Create lambda
	lambda := &Lambda{
		Params: params,
		Body:   bodyNode,
		Ctx:    evalCtx.Clone(),
	}

	return lambda, nil
}

// evalBind evaluates an assignment expression.
