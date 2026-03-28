package parser

import (
	"encoding/json"
	"strconv"

	"github.com/sandrolain/gosonata/pkg/types"
)

// funcFastKinds maps stdlib function names (with "$" prefix) to their
// [types.FuncFastKind] constant. Only functions that can be evaluated
// zero-copy against a gjson.Result are listed here.
//
// $round is intentionally omitted: JSONata specifies "round half to even"
// (banker's rounding) which requires non-trivial logic.
var funcFastKinds = map[string]types.FuncFastKind{
	"$exists":    types.FuncFastExists,
	"$contains":  types.FuncFastContains,
	"$string":    types.FuncFastString,
	"$boolean":   types.FuncFastBoolean,
	"$number":    types.FuncFastNumber,
	"$keys":      types.FuncFastKeys,
	"$distinct":  types.FuncFastDistinct,
	"$not":       types.FuncFastNot,
	"$lowercase": types.FuncFastLowercase,
	"$uppercase": types.FuncFastUppercase,
	"$trim":      types.FuncFastTrim,
	"$length":    types.FuncFastLength,
	"$type":      types.FuncFastType,
	"$abs":       types.FuncFastAbs,
	"$floor":     types.FuncFastFloor,
	"$ceil":      types.FuncFastCeil,
	"$sqrt":      types.FuncFastSqrt,
	"$count":     types.FuncFastCount,
	"$reverse":   types.FuncFastReverse,
	"$sum":       types.FuncFastSum,
	"$max":       types.FuncFastMax,
	"$min":       types.FuncFastMin,
	"$average":   types.FuncFastAverage,
}

// AnalyzeFastPath performs a compile-time static analysis of the AST rooted
// at node and returns a [types.FastPathInfo] if the expression can be
// evaluated zero-copy against raw JSON bytes.
//
// The function recognises three patterns:
//   - Pure dotted-path navigation ("a.b.c") → [types.FastPathPurePath]
//   - Equality/inequality against a literal ("a.b = \"x\"") → [types.FastPathComparison]
//   - Supported stdlib function on a pure path ("$exists(a.b)") → [types.FastPathFunc]
//
// Returns nil when the expression requires full AST evaluation.
func AnalyzeFastPath(node *types.ASTNode) *types.FastPathInfo {
	if node == nil {
		return nil
	}

	// 1. Try pure-path first (most common hot case).
	if path, ok := collectGJSONPath(node); ok {
		return &types.FastPathInfo{
			Kind:      types.FastPathPurePath,
			GJSONPath: path,
		}
	}

	// 2. Try comparison: "path = literal" or "path != literal".
	if fp := tryCollectComparison(node); fp != nil {
		return fp
	}

	// 3. Try function fast-path: "$func(pure-path)" or "$func(pure-path, literal)".
	if fp := tryCollectFunc(node); fp != nil {
		return fp
	}

	return nil
}

// collectGJSONPath converts a pure path AST subtree into a gjson path string
// (e.g. "Account.Order.Product"). Returns ("", false) if the subtree contains
// any node that gjson cannot represent — wildcards, predicates, variables, etc.
func collectGJSONPath(node *types.ASTNode) (string, bool) {
	if node == nil {
		return "", false
	}

	switch node.Type {
	case types.NodeName:
		// Simple field name. Value is guaranteed to be a string by the parser.
		name := node.StrValue
		if name == "" {
			if s, ok := node.Value.(string); ok {
				name = s
			}
		}
		if name == "" {
			return "", false
		}
		return name, true

	case types.NodePath:
		// Binary path: LHS.RHS — both sides must be pure.
		if node.LHS == nil || node.RHS == nil {
			return "", false
		}
		lhs, ok1 := collectGJSONPath(node.LHS)
		rhs, ok2 := collectGJSONPath(node.RHS)
		if !ok1 || !ok2 {
			return "", false
		}
		if lhs == "" {
			return rhs, true
		}
		return lhs + "." + rhs, true

	default:
		// Any other node type (wildcard, predicate, variable, …) makes the
		// expression ineligible for the fast path.
		return "", false
	}
}

// tryCollectComparison returns a [types.FastPathInfo] if node is a binary
// equality or inequality expression whose left-hand side is a pure path and
// whose right-hand side is a JSON-representable literal (string, number,
// boolean, or null).
func tryCollectComparison(node *types.ASTNode) *types.FastPathInfo {
	if node.Type != types.NodeBinary {
		return nil
	}

	op := node.StrValue
	if op == "" {
		if s, ok := node.Value.(string); ok {
			op = s
		}
	}
	if op != "=" && op != "!=" {
		return nil
	}

	if node.LHS == nil || node.RHS == nil {
		return nil
	}

	path, ok := collectGJSONPath(node.LHS)
	if !ok {
		return nil
	}

	lit, ok := extractJSONLiteral(node.RHS)
	if !ok {
		return nil
	}

	return &types.FastPathInfo{
		Kind:       types.FastPathComparison,
		GJSONPath:  path,
		CompareOp:  op,
		CompareVal: lit,
	}
}

// tryCollectFunc returns a [types.FastPathInfo] if node is a call to a
// supported stdlib function whose first argument is a pure path. An optional
// second argument may be a string literal (e.g. for $contains).
func tryCollectFunc(node *types.ASTNode) *types.FastPathInfo {
	if node.Type != types.NodeFunction {
		return nil
	}

	// The parser stores the function name in node.LHS (a NodeVariable) rather
	// than in node.StrValue, because parseFunctionCall keeps the callee in LHS.
	// The lexer strips the "$" prefix, so "$exists" is stored as "exists".
	if node.LHS == nil {
		return nil
	}
	name := node.LHS.StrValue
	if name == "" {
		if s, ok := node.LHS.Value.(string); ok {
			name = s
		}
	}
	// Re-add the "$" prefix for map lookup (funcFastKinds keys are "$exists" etc.).
	name = "$" + name

	kind, supported := funcFastKinds[name]
	if !supported {
		return nil
	}

	if len(node.Arguments) < 1 {
		return nil
	}

	path, ok := collectGJSONPath(node.Arguments[0])
	if !ok {
		return nil
	}

	fp := &types.FastPathInfo{
		Kind:      types.FastPathFunc,
		GJSONPath: path,
		FuncKind:  kind,
	}

	// Optional second string-literal argument (e.g. the needle for $contains).
	if len(node.Arguments) == 2 {
		if lit, ok2 := extractJSONLiteral(node.Arguments[1]); ok2 {
			fp.FuncArg = lit
		}
	}

	return fp
}

// extractJSONLiteral returns the raw JSON representation of a literal AST
// node (string, number, boolean, or null) and true. Returns ("", false) for
// any non-literal node.
func extractJSONLiteral(node *types.ASTNode) (string, bool) {
	if node == nil {
		return "", false
	}

	switch node.Type {
	case types.NodeString:
		// Encode as a JSON string so the comparison is JSON-comparable.
		b, err := json.Marshal(node.StrValue)
		if err != nil {
			return "", false
		}
		return string(b), true

	case types.NodeNumber:
		return strconv.FormatFloat(node.NumValue, 'f', -1, 64), true

	case types.NodeBoolean:
		// NodeBoolean and NodeNull share the same NodeType constant ("value").
		// We distinguish them by the Go type of Value.
		switch v := node.Value.(type) {
		case bool:
			if v {
				return "true", true
			}
			return "false", true
		case types.Null:
			return "null", true
		}
		return "", false

	default:
		return "", false
	}
}
