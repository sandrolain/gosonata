package evaluator

import (
	"context"
	"fmt"
	"github.com/sandrolain/gosonata/pkg/types"
)

func (e *Evaluator) evalNode(ctx context.Context, node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Track and check evaluation depth (stack-style, matching JSONata JS semantics).
	// Depth is the current nesting level of evalNode calls; it is incremented on entry
	// and decremented on exit so that only the maximum live stack depth is counted.
	if p := getRecurseDepthPtr(ctx); p != nil {
		*p++
		if *p > e.opts.MaxDepth {
			*p--
			return nil, types.NewError(types.ErrUndefinedVariable, "maximum recursion depth exceeded", -1)
		}
		defer func() { *p-- }()
	}

	if node == nil {
		return nil, nil
	}

	// Debug logging
	if e.opts.Debug {
		e.logger.Debug("evaluating node",
			"type", node.Type,
			"value", node.Value,
			"depth", evalCtx.Depth())
	}

	// Dispatch based on node type
	switch node.Type {
	case types.NodeString:
		return e.evalString(node)
	case types.NodeNumber:
		return e.evalNumber(node)
	case "value": // NodeBoolean or NodeNull
		// Keep types.Null as-is during evaluation
		// Will be converted to nil at final return
		return node.Value, nil
	case types.NodeName:
		return e.evalName(node, evalCtx)
	case types.NodeVariable:
		return e.evalVariable(node, evalCtx)
	case types.NodePath:
		return e.evalPath(ctx, node, evalCtx)
	case types.NodeDescendant:
		return e.evalDescendent(ctx, node, evalCtx)
	case types.NodeWildcard:
		return e.evalWildcard(ctx, node, evalCtx)
	case types.NodeRegex:
		return e.evalRegex(node)
	case types.NodeBinary:
		return e.evalBinary(ctx, node, evalCtx)
	case types.NodeUnary:
		return e.evalUnary(ctx, node, evalCtx)
	case types.NodeArray:
		return e.evalArray(ctx, node, evalCtx)
	case types.NodeObject:
		return e.evalObject(ctx, node, evalCtx)
	case types.NodeFilter:
		return e.evalFilter(ctx, node, evalCtx)
	case types.NodeCondition:
		return e.evalCondition(ctx, node, evalCtx)
	case types.NodeFunction:
		return e.evalFunction(ctx, node, evalCtx)
	case types.NodePartial:
		return e.evalPartial(ctx, node, evalCtx)
	case types.NodeLambda:
		return e.evalLambda(node, evalCtx)
	case types.NodeBind:
		return e.evalBind(ctx, node, evalCtx)
	case types.NodeBlock:
		return e.evalBlock(ctx, node, evalCtx)
	case types.NodeSort:
		return e.evalSort(ctx, node, evalCtx)
	case types.NodeTransform:
		// Standalone transform: apply to current context data
		return e.evalTransformNode(ctx, evalCtx.Data(), node, evalCtx)
	case types.NodeParent:
		return e.evalParent(node, evalCtx)
	case types.NodeContext:
		return e.evalContextBind(ctx, node, evalCtx)
	case types.NodeIndex:
		return e.evalIndexBind(ctx, node, evalCtx)
	default:
		return nil, fmt.Errorf("unsupported node type: %s", node.Type)
	}
}

// evalString evaluates a string literal.
