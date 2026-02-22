package evaluator

import (
	"context"
	"fmt"
	"regexp"

	"github.com/sandrolain/gosonata/pkg/types"
)

func (e *Evaluator) evalApply(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Evaluate left side (the data to pipe)
	data, err := e.evalNode(ctx, node.LHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// If RHS is a transform literal, apply the transform to the LHS data
	if node.RHS.Type == types.NodeTransform {
		return e.evalTransformNode(ctx, data, node.RHS, evalCtx)
	}

	// Special case: if data is a function, check for function composition
	isDataFunction := false
	switch data.(type) {
	case *Lambda, *FunctionDef:
		isDataFunction = true
	}

	// If data is a function and RHS evaluates to a function, create composed function
	if isDataFunction {
		// Evaluate RHS to check if it's also a function
		var rhsFunc interface{}
		if node.RHS.Type == types.NodeVariable || node.RHS.Type == types.NodePartial {
			// Variable or partial application that should resolve to a function
			rhsFunc, err = e.evalNode(ctx, node.RHS, evalCtx)
			if err != nil {
				return nil, err
			}
		} else if node.RHS.Type == types.NodeFunction && node.RHS.LHS != nil {
			// Function call through variable
			rhsFunc, err = e.evalNode(ctx, node.RHS, evalCtx)
			if err != nil {
				return nil, err
			}
		}

		// Check if RHS is a function
		if rhsFunc != nil {
			switch rhsFunc.(type) {
			case *Lambda, *FunctionDef:
				// Create function composition: f ~> g creates λx.g(f(x))
				return e.createComposition(data, rhsFunc, evalCtx), nil
			}
		}
	}

	// Check if RHS is a NodeFilter wrapping a function call (e.g., $map($fn)[])
	// In this case, inject data into the inner function call, then apply the filter
	if node.RHS.Type == types.NodeFilter && node.RHS.LHS != nil && node.RHS.LHS.Type == types.NodeFunction {
		innerFnNode := node.RHS.LHS
		filterNode := node.RHS

		// Evaluate the inner function call with data prepended
		var innerResult interface{}
		if innerFnNode.LHS != nil {
			// Variable/lambda call - evaluate the callable and call with data prepended
			callableValue, err := e.evalNode(ctx, innerFnNode.LHS, evalCtx)
			if err != nil {
				return nil, err
			}
			args := make([]interface{}, 0, len(innerFnNode.Arguments)+1)
			args = append(args, data)
			for _, argNode := range innerFnNode.Arguments {
				arg, err := e.evalNode(ctx, argNode, evalCtx)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
			}
			switch fn := callableValue.(type) {
			case *Lambda:
				innerResult, err = e.callLambda(ctx, fn, args)
			case *FunctionDef:
				if len(args) < fn.MinArgs {
					return nil, types.NewError(types.ErrArgumentCountMismatch,
						fmt.Sprintf("function requires at least %d arguments, got %d", fn.MinArgs, len(args)), -1)
				}
				innerResult, err = fn.Impl(ctx, e, evalCtx, args)
			default:
				return nil, fmt.Errorf("expected lambda or function, got %T", callableValue)
			}
			if err != nil {
				return nil, err
			}
		} else if innerFnNode.Value != nil {
			// Named function call
			funcName := innerFnNode.Value.(string)
			fnDef, ok := GetFunction(funcName)
			if !ok {
				return nil, fmt.Errorf("unknown function: %s", funcName)
			}
			args := make([]interface{}, 0, len(innerFnNode.Arguments)+1)
			args = append(args, data)
			for _, argNode := range innerFnNode.Arguments {
				arg, err := e.evalNode(ctx, argNode, evalCtx)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
			}
			if len(args) < fnDef.MinArgs {
				return nil, types.NewError(types.ErrArgumentCountMismatch,
					fmt.Sprintf("function requires at least %d arguments, got %d", fnDef.MinArgs, len(args)), -1)
			}
			innerResult, err = fnDef.Impl(ctx, e, evalCtx, args)
			if err != nil {
				return nil, err
			}
		}

		// Now apply the filter/keep-array operation to the inner result
		// We DON'T call evalFilter(filterNode) because that would re-evaluate filterNode.LHS.
		// Instead, apply the filter directly to innerResult.
		if filterNode.RHS == nil {
			// Empty filter [] means "return as array" (KeepArray)
			arr, err := e.toArray(innerResult)
			if err != nil {
				return nil, err
			}
			if len(arr) == 0 {
				return nil, nil
			}
			return arr, nil
		}
		// Non-empty filter: apply predicate to innerResult
		innerArr, err := e.toArray(innerResult)
		if err != nil {
			return nil, err
		}
		// Apply filter predicate similar to evalFilter but using innerArr directly
		var filterResult []interface{}
		for i, item := range innerArr {
			itemCtx := evalCtx.NewChildContext(item)
			itemCtx.SetBinding("", item) // Set $ to item
			// Evaluate filter predicate
			predVal, err := e.evalNode(ctx, filterNode.RHS, itemCtx)
			if err != nil {
				return nil, err
			}
			// Check if predicate is a number (index)
			if idx, ok := predVal.(float64); ok {
				if int(idx) == i {
					filterResult = append(filterResult, item)
				}
			} else if e.isTruthy(predVal) {
				filterResult = append(filterResult, item)
			}
		}
		if len(filterResult) == 0 {
			return nil, nil
		}
		if len(filterResult) == 1 {
			return filterResult[0], nil
		}
		return filterResult, nil
	}

	// Check if RHS is a function call
	if node.RHS.Type == types.NodeFunction {
		// It's a function call - inject data as first argument
		fnNode := node.RHS

		// If it's a built-in function call (Value contains name)
		if fnNode.Value != nil {
			funcName := fnNode.Value.(string)
			fnDef, ok := GetFunction(funcName)
			if !ok {
				return nil, fmt.Errorf("unknown function: %s", funcName)
			}

			// Evaluate existing arguments
			args := make([]interface{}, 0, len(fnNode.Arguments)+1)
			args = append(args, data) // Prepend piped data

			for _, argNode := range fnNode.Arguments {
				arg, err := e.evalNode(ctx, argNode, evalCtx)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
			}

			// Validate argument count
			if len(args) < fnDef.MinArgs {
				return nil, types.NewError(types.ErrArgumentCountMismatch,
					fmt.Sprintf("function requires at least %d arguments, got %d", fnDef.MinArgs, len(args)), -1)
			}
			if fnDef.MaxArgs != -1 && len(args) > fnDef.MaxArgs {
				return nil, types.NewError(types.ErrArgumentCountMismatch,
					fmt.Sprintf("function accepts at most %d arguments, got %d", fnDef.MaxArgs, len(args)), -1)
			}

			// Call the function
			return fnDef.Impl(ctx, e, evalCtx, args)
		}

		// If it's a lambda/variable function call (LHS contains callable)
		if fnNode.LHS != nil {
			callableValue, err := e.evalNode(ctx, fnNode.LHS, evalCtx)
			if err != nil {
				return nil, err
			}

			// Evaluate arguments
			args := make([]interface{}, 0, len(fnNode.Arguments)+1)
			args = append(args, data) // Prepend piped data

			for _, argNode := range fnNode.Arguments {
				arg, err := e.evalNode(ctx, argNode, evalCtx)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
			}

			// Call based on type
			switch fn := callableValue.(type) {
			case *Lambda:
				return e.callLambda(ctx, fn, args)
			case *FunctionDef:
				return fn.Impl(ctx, e, evalCtx, args)
			default:
				return nil, fmt.Errorf("expected lambda or function, got %T", callableValue)
			}
		}
	}

	// RHS is not a function call - evaluate it and expect a lambda or regex
	fn, err := e.evalNode(ctx, node.RHS, evalCtx)
	if err != nil {
		return nil, err
	}

	// If fn is a regex, apply it to data as a match test
	if regex, ok := fn.(*regexp.Regexp); ok {
		// Convert data to string
		str, ok := data.(string)
		if !ok {
			str = fmt.Sprint(data)
		}
		return regex.MatchString(str), nil
	}

	// If fn is a lambda, call it with data as argument
	if lambda, ok := fn.(*Lambda); ok {
		return e.callLambda(ctx, lambda, []interface{}{data})
	}

	// If fn is a function definition, call it
	if fnDef, ok := fn.(*FunctionDef); ok {
		return fnDef.Impl(ctx, e, evalCtx, []interface{}{data})
	}

	return nil, types.NewError(types.ErrInvokeNonFunction, "right side of ~> must be a function", -1)
}

// createComposition creates a composed function from two functions.
// composition(f, g) returns λx.g(f(x))

func (e *Evaluator) createComposition(leftFn, rightFn interface{}, evalCtx *EvalContext) *Lambda {
	// Create a lambda that accepts one parameter and applies both functions
	bodyNode := types.NewASTNode(types.NodeFunction, 0)

	// The body calls rightFn with the result of calling leftFn
	// First, call leftFn with the parameter
	leftCallNode := types.NewASTNode(types.NodeFunction, 0)
	leftCallNode.LHS = &types.ASTNode{
		Type:  types.NodeVariable,
		Value: "leftFn",
	}
	leftCallNode.Arguments = []*types.ASTNode{
		{
			Type:  types.NodeVariable,
			Value: "1", // Parameter name
		},
	}

	// Then call rightFn with the result
	bodyNode.LHS = &types.ASTNode{
		Type:  types.NodeVariable,
		Value: "rightFn",
	}
	bodyNode.Arguments = []*types.ASTNode{leftCallNode}

	// Create context with both functions bound
	composedCtx := evalCtx.Clone()
	composedCtx.SetBinding("leftFn", leftFn)
	composedCtx.SetBinding("rightFn", rightFn)

	return &Lambda{
		Params: []string{"1"},
		Body:   bodyNode,
		Ctx:    composedCtx,
	}
}

// evalSort evaluates a sort expression (^).
// Syntax: sequence^(sort-key-expression)
// Examples: items^($), data^(>price), results^(<count)
