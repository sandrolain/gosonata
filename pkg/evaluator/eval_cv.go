package evaluator

type contextBoundValue struct {
	value     interface{}            // current context data (used as $ for evaluation)
	parent    interface{}            // preceding context data (used by @ to rewind context)
	bindings  map[string]interface{} // inherited variable bindings ($var → value)
	parentObj interface{}            // the containing object for % operator (distinct from @ semantics)
}

// extractBoundItem unpacks a contextBoundValue, returning value and bindings (empty map if plain).

func extractBoundItem(item interface{}) (value interface{}, bindings map[string]interface{}) {
	if cv, ok := item.(*contextBoundValue); ok {
		b := cv.bindings
		if b == nil {
			b = map[string]interface{}{}
		}
		return cv.value, b
	}
	return item, nil
}

// mergeBoundBindings merges parentBindings into a result item, wrapping or upgrading its cv.
// parentBindings take lower priority than child's own bindings.

func mergeBoundBindings(item interface{}, parentBindings map[string]interface{}, parentValue interface{}) interface{} {
	if len(parentBindings) == 0 {
		return item
	}
	if cv, ok := item.(*contextBoundValue); ok {
		merged := make(map[string]interface{}, len(parentBindings)+len(cv.bindings))
		for k, v := range parentBindings {
			merged[k] = v
		}
		for k, v := range cv.bindings { // child overrides
			merged[k] = v
		}
		return &contextBoundValue{value: cv.value, parent: cv.parent, bindings: merged}
	}
	// Wrap plain value with parent bindings
	return &contextBoundValue{value: item, parent: parentValue, bindings: copyBindings(parentBindings)}
}

// copyBindings makes a shallow copy of a binding map.

func copyBindings(b map[string]interface{}) map[string]interface{} {
	if len(b) == 0 {
		return nil
	}
	c := make(map[string]interface{}, len(b))
	for k, v := range b {
		c[k] = v
	}
	return c
}

// applyBindingsToCtx sets all bindings onto an EvalContext.

func applyBindingsToCtx(ctx *EvalContext, bindings map[string]interface{}) {
	for k, v := range bindings {
		ctx.SetBinding(k, v)
	}
}

// unwrapCVsDeep recursively extracts plain values from contextBoundValues.
// This is used when CVs must be invisible to operators (equality, arithmetic, etc.)
// and at the final return point of evaluation.

func unwrapCVsDeep(v interface{}) interface{} {
	switch val := v.(type) {
	case *contextBoundValue:
		return unwrapCVsDeep(val.value)
	case []interface{}:
		// Check if any items (at any depth) need unwrapping
		needsUnwrap := false
		for _, item := range val {
			switch item.(type) {
			case *contextBoundValue, []interface{}, *OrderedObject:
				needsUnwrap = true
			}
			if needsUnwrap {
				break
			}
		}
		if !needsUnwrap {
			return val
		}
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = unwrapCVsDeep(item)
		}
		return result
	case *OrderedObject:
		// Unwrap CVs inside OrderedObject values
		for k, ov := range val.Values {
			unwrapped := unwrapCVsDeep(ov)
			val.Values[k] = unwrapped
		}
		return val
	default:
		return v
	}
}

// evalNode evaluates an AST node in the given context.
// recurseDepthKey stores a *int pointer so depth can be incremented/decremented (stack-style)
// matching JSONata JS semantics where depth is the maximum current call stack depth.
