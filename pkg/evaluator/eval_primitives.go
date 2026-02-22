package evaluator

import (
	"fmt"
	"regexp"
	"github.com/sandrolain/gosonata/pkg/types"
)

func (e *Evaluator) evalString(node *types.ASTNode) (interface{}, error) {
	return node.Value, nil
}

// evalNumber evaluates a number literal.

func (e *Evaluator) evalNumber(node *types.ASTNode) (interface{}, error) {
	return node.Value, nil
}

// evalRegex evaluates a regex literal.

func (e *Evaluator) evalRegex(node *types.ASTNode) (interface{}, error) {
	pattern, ok := node.Value.(string)
	if !ok {
		return nil, fmt.Errorf("invalid regex pattern type")
	}

	// Compile the regex pattern (already converted to Go format by lexer)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	return re, nil
}

// evalBoolean evaluates a boolean literal.

func (e *Evaluator) evalBoolean(node *types.ASTNode) (interface{}, error) {
	return node.Value, nil
}

// evalName evaluates a name (field reference).

func (e *Evaluator) evalName(node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	name := node.Value.(string)
	return e.evalNameString(name, evalCtx)
}


func (e *Evaluator) evalNameString(name string, evalCtx *EvalContext) (interface{}, error) {
	data := evalCtx.Data()

	if obj, ok := data.(map[string]interface{}); ok {
		if value, exists := obj[name]; exists {
			// JSON null (nil from encoding/json) becomes types.Null to distinguish from undefined
			if value == nil {
				return types.NullValue, nil
			}
			return value, nil
		}
	}
	if obj, ok := data.(*OrderedObject); ok {
		if value, exists := obj.Get(name); exists {
			if value == nil {
				return types.NullValue, nil
			}
			return value, nil
		}
	}
	if arr, ok := data.([]interface{}); ok {
		result := make([]interface{}, 0, len(arr))
		for _, item := range arr {
			if obj, ok := item.(map[string]interface{}); ok {
				if value, exists := obj[name]; exists {
					if subArr, isArr := value.([]interface{}); isArr {
						result = append(result, subArr...)
					} else {
						result = append(result, value)
					}
				}
			} else if obj, ok := item.(*OrderedObject); ok {
				if value, exists := obj.Get(name); exists {
					if subArr, isArr := value.([]interface{}); isArr {
						result = append(result, subArr...)
					} else {
						result = append(result, value)
					}
				}
			} else if subArr, ok := item.([]interface{}); ok {
				// Nested array: recurse into it
				subCtx := evalCtx.NewChildContext(subArr)
				if value, err := e.evalNameString(name, subCtx); err == nil && value != nil {
					if subArrVal, isArr := value.([]interface{}); isArr {
						result = append(result, subArrVal...)
					} else {
						result = append(result, value)
					}
				}
			}
		}
		if len(result) == 0 {
			return nil, nil
		}
		if len(result) == 1 {
			return result[0], nil
		}
		return result, nil
	}

	return nil, nil
}

// evalVariable evaluates a variable reference.

func (e *Evaluator) evalVariable(node *types.ASTNode, evalCtx *EvalContext) (interface{}, error) {
	varName := node.Value.(string)

	// $ refers to current context
	if varName == "" {
		data := evalCtx.Data()
		return data, nil
	}

	// $$ refers to root context
	if varName == "$" {
		if evalCtx.Root() != nil {
			return evalCtx.Root().Data(), nil
		}
		// Fallback: if no root, return current context (shouldn't happen)
		return evalCtx.Data(), nil
	}

	// Named variable - check bindings
	value, found := evalCtx.GetBinding(varName)
	if !found {
		// If a built-in function exists with this name, return it as a value
		if fnDef, ok := GetFunction(varName); ok {
			return fnDef, nil
		}
		// Per JSONata spec: undefined variables return nil (undefined), not error
		return nil, nil
	}

	return value, nil

	return value, nil
}

// evalPath evaluates a path expression (field navigation).
