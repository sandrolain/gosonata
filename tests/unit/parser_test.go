package unit_test

import (
	"testing"

	"github.com/sandrolain/gosonata/pkg/parser"
	"github.com/sandrolain/gosonata/pkg/types"
)

// Helper functions

func parseExpr(t *testing.T, input string) *types.ASTNode {
	t.Helper()
	expr, err := parser.Parse(input)
	if err != nil {
		t.Fatalf("Failed to parse %q: %v", input, err)
	}
	return expr.AST()
}

func expectError(t *testing.T, input string) {
	t.Helper()
	_, err := parser.Parse(input)
	if err == nil {
		t.Fatalf("Expected error parsing %q but got none", input)
	}
}

func checkNode(t *testing.T, node *types.ASTNode, expectedType types.NodeType, expectedValue interface{}) {
	t.Helper()
	if node == nil {
		t.Fatal("Node is nil")
	}
	if node.Type != expectedType {
		t.Errorf("Expected node type %s, got %s", expectedType, node.Type)
	}
	if expectedValue != nil && node.Value != expectedValue {
		t.Errorf("Expected value %v, got %v", expectedValue, node.Value)
	}
}

// Literal tests

func TestParseLiterals(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		nodeType types.NodeType
		value    interface{}
	}{
		{"string", `"hello"`, types.NodeString, "hello"},
		{"empty string", `""`, types.NodeString, ""},
		{"number int", "42", types.NodeNumber, 42.0},
		{"number float", "3.14", types.NodeNumber, 3.14},
		{"number scientific", "1e10", types.NodeNumber, 1e10},
		{"boolean true", "true", types.NodeBoolean, true},
		{"boolean false", "false", types.NodeBoolean, false},
		{"null", "null", types.NodeNull, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			checkNode(t, node, tt.nodeType, tt.value)
		})
	}
}

func TestParseVariables(t *testing.T) {
	tests := []struct {
		name  string
		input string
		value string
	}{
		{"context", "$", ""},
		{"parent context", "$$", "$"},
		{"named variable", "$name", "name"},
		{"complex name", "$myVariable123", "myVariable123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			checkNode(t, node, types.NodeVariable, tt.value)
		})
	}
}

func TestParseNames(t *testing.T) {
	tests := []struct {
		name  string
		input string
		value string
	}{
		{"simple name", "field", "field"},
		{"with underscore", "field_name", "field_name"},
		{"with number", "field123", "field123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			checkNode(t, node, types.NodeName, tt.value)
		})
	}
}

// Operator tests

func TestParseBinaryOperators(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		operator string
	}{
		// Arithmetic
		{"addition", "1 + 2", "+"},
		{"subtraction", "5 - 3", "-"},
		{"multiplication", "4 * 3", "*"},
		{"division", "10 / 2", "/"},
		{"modulo", "10 % 3", "%"},

		// Comparison
		{"equality", "a = b", "="},
		{"inequality", "a != b", "!="},
		{"less than", "a < b", "<"},
		{"less equal", "a <= b", "<="},
		{"greater than", "a > b", ">"},
		{"greater equal", "a >= b", ">="},

		// Logical
		{"and", "a and b", "and"},
		{"or", "a or b", "or"},
		{"in", "a in b", "in"},

		// String
		{"concatenation", `"a" & "b"`, "&"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			checkNode(t, node, types.NodeBinary, tt.operator)
			if node.LHS == nil {
				t.Error("Left-hand side is nil")
			}
			if node.RHS == nil {
				t.Error("Right-hand side is nil")
			}
		})
	}
}

func TestParseUnaryOperators(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		operator string
	}{
		{"negation", "-5", "-"},
		{"negation expression", "-(a + b)", "-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			checkNode(t, node, types.NodeUnary, tt.operator)
			if node.LHS == nil {
				t.Error("Expression is nil")
			}
		})
	}
}

// Path tests

func TestParsePaths(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple path", "a.b"},
		{"nested path", "a.b.c"},
		{"path with variable", "$.name"},
		{"deep path", "a.b.c.d.e"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			// Check it's a path node
			for node.Type == types.NodePath {
				if node.LHS == nil {
					t.Error("Left side of path is nil")
				}
				if node.RHS == nil {
					t.Error("Right side of path is nil")
				}
				node = node.LHS
			}
		})
	}
}

// Grouping tests

func TestParseGrouping(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple grouping", "(1 + 2)"},
		{"nested grouping", "((a + b) * c)"},
		{"multiple operations", "(1 + 2) * (3 + 4)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			if node == nil {
				t.Fatal("Node is nil")
			}
			// The node itself should not be a grouping (grouping is just for precedence)
		})
	}
}

// Constructor tests

func TestParseArrayConstructor(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedSize int
	}{
		{"empty array", "[]", 0},
		{"single element", "[1]", 1},
		{"multiple elements", "[1, 2, 3]", 3},
		{"mixed types", `[1, "two", true]`, 3},
		{"nested arrays", "[[1, 2], [3, 4]]", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			checkNode(t, node, types.NodeArray, nil)
			if len(node.Expressions) != tt.expectedSize {
				t.Errorf("Expected %d elements, got %d", tt.expectedSize, len(node.Expressions))
			}
		})
	}
}

func TestParseObjectConstructor(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedSize int
	}{
		{"empty object", "{}", 0},
		{"single property", `{"name": "value"}`, 1},
		{"multiple properties", `{"a": 1, "b": 2}`, 2},
		{"mixed values", `{"num": 42, "str": "text", "bool": true}`, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			checkNode(t, node, types.NodeObject, nil)
			if len(node.Expressions) != tt.expectedSize {
				t.Errorf("Expected %d properties, got %d", tt.expectedSize, len(node.Expressions))
			}
		})
	}
}

// Filter tests

func TestParseFilters(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple filter", "items[price > 100]"},
		{"filter on path", "data.items[active = true]"},
		{"complex condition", "items[price > 100 and quantity < 50]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			// Should have a filter node somewhere
			found := false
			var check func(*types.ASTNode)
			check = func(n *types.ASTNode) {
				if n == nil {
					return
				}
				if n.Type == types.NodeFilter {
					found = true
					return
				}
				check(n.LHS)
				check(n.RHS)
				for _, e := range n.Expressions {
					check(e)
				}
			}
			check(node)
			if !found {
				t.Error("Filter node not found in AST")
			}
		})
	}
}

// Precedence tests

func TestOperatorPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		checkAST func(t *testing.T, node *types.ASTNode)
	}{
		{
			name:  "multiplication before addition",
			input: "1 + 2 * 3",
			checkAST: func(t *testing.T, node *types.ASTNode) {
				// Should be: (1 + (2 * 3))
				// Root is +
				if node.Type != types.NodeBinary || node.Value != "+" {
					t.Errorf("Root should be +, got %v", node.Value)
				}
				// Right side should be *
				if node.RHS.Type != types.NodeBinary || node.RHS.Value != "*" {
					t.Errorf("Right side should be *, got %v", node.RHS.Value)
				}
			},
		},
		{
			name:  "division before subtraction",
			input: "10 - 4 / 2",
			checkAST: func(t *testing.T, node *types.ASTNode) {
				// Should be: (10 - (4 / 2))
				// Root is -
				if node.Type != types.NodeBinary || node.Value != "-" {
					t.Errorf("Root should be -, got %v", node.Value)
				}
				// Right side should be /
				if node.RHS.Type != types.NodeBinary || node.RHS.Value != "/" {
					t.Errorf("Right side should be /, got %v", node.RHS.Value)
				}
			},
		},
		{
			name:  "comparison before and",
			input: "a > 5 and b < 10",
			checkAST: func(t *testing.T, node *types.ASTNode) {
				// Should be: ((a > 5) and (b < 10))
				// Root is and
				if node.Type != types.NodeBinary || node.Value != "and" {
					t.Errorf("Root should be and, got %v", node.Value)
				}
				// Both sides should be comparison
				if node.LHS.Type != types.NodeBinary || node.LHS.Value != ">" {
					t.Errorf("Left side should be >, got %v", node.LHS.Value)
				}
				if node.RHS.Type != types.NodeBinary || node.RHS.Value != "<" {
					t.Errorf("Right side should be <, got %v", node.RHS.Value)
				}
			},
		},
		{
			name:  "grouping overrides precedence",
			input: "(1 + 2) * 3",
			checkAST: func(t *testing.T, node *types.ASTNode) {
				// Should be: ((1 + 2) * 3)
				// Root is *
				if node.Type != types.NodeBinary || node.Value != "*" {
					t.Errorf("Root should be *, got %v", node.Value)
				}
				// Left side should be +
				if node.LHS.Type != types.NodeBinary || node.LHS.Value != "+" {
					t.Errorf("Left side should be +, got %v", node.LHS.Value)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			tt.checkAST(t, node)
		})
	}
}

// Complex expression tests

func TestParseComplexExpressions(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"nested paths with filter", "data.items[price > 100].name"},
		{"arithmetic in filter", "items[price * quantity > 1000]"},
		{"object with expressions", `{"total": price * quantity, "tax": price * 0.1}`},
		{"array of expressions", "[a + b, c * d, e / f]"},
		{"mixed operators", "a + b * c - d / e"},
		{"logical with comparison", "(a > 5 and b < 10) or c = 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			if node == nil {
				t.Fatal("Node is nil")
			}
			// Just verify it parses without error
		})
	}
}

// Error tests

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unclosed paren", "(1 + 2"},
		{"unclosed bracket", "[1, 2"},
		{"unclosed brace", "{\"a\": 1"},
		{"missing operand", "1 +"},
		{"invalid token", "@#$%"},
		{"empty input", ""},
		{"incomplete path", "a."},
		{"incomplete filter", "items["},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectError(t, tt.input)
		})
	}
}

// Edge cases

func TestParseEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"whitespace only", "   "},
		{"multiple spaces", "1    +    2"},
		{"newlines", "1\n+\n2"},
		{"tabs", "1\t+\t2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should either parse or error, but not panic
			_, _ = parser.Parse(tt.input)
		})
	}
}

// Additional coverage tests

func TestParseNumberVariations(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"negative int", "-42", true},
		{"negative float", "-3.14", true},
		{"negative scientific", "-1e10", true},
		{"positive sign", "+42", false}, // + is not a unary operator in JSONata
		{"zero", "0", true},
		{"negative zero", "-0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				node := parseExpr(t, tt.input)
				if node == nil {
					t.Fatal("Node is nil")
				}
			} else {
				// Should either parse differently or error
				_, _ = parser.Parse(tt.input)
			}
		})
	}
}

func TestParseComplexConstructors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"array with trailing comma", "[1, 2,]"},
		{"object with trailing comma", `{"a": 1,}`},
		{"nested empty arrays", "[[[]]]"},
		{"nested empty objects", "{{}}"},
		{"object with computed keys", `{a: 1, b: 2}`},
		{"array with expressions", "[a + b, c * d]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Some of these should error, some should parse
			_, _ = parser.Parse(tt.input)
		})
	}
}

func TestParseComplexFilters(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"multiple filters", "items[price > 100][quantity < 50]"},
		{"filter with path", "data.items[item.price > 100]"},
		{"filter with nested expression", "items[(price + tax) > 100]"},
		{"filter on variable", "$var[price > 100]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			if node == nil {
				t.Fatal("Node is nil")
			}
		})
	}
}

func TestParseOperatorAssociativity(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		checkAST func(t *testing.T, node *types.ASTNode)
	}{
		{
			name:  "left associative addition",
			input: "1 + 2 + 3",
			checkAST: func(t *testing.T, node *types.ASTNode) {
				// Should be: ((1 + 2) + 3)
				if node.Type != types.NodeBinary || node.Value != "+" {
					t.Errorf("Root should be +, got %v", node.Value)
				}
				if node.LHS.Type != types.NodeBinary || node.LHS.Value != "+" {
					t.Errorf("Left side should be +, got %v", node.LHS.Value)
				}
			},
		},
		{
			name:  "left associative subtraction",
			input: "10 - 5 - 2",
			checkAST: func(t *testing.T, node *types.ASTNode) {
				// Should be: ((10 - 5) - 2)
				if node.Type != types.NodeBinary || node.Value != "-" {
					t.Errorf("Root should be -, got %v", node.Value)
				}
				if node.LHS.Type != types.NodeBinary || node.LHS.Value != "-" {
					t.Errorf("Left side should be -, got %v", node.LHS.Value)
				}
			},
		},
		{
			name:  "chained comparisons",
			input: "a = b and b = c",
			checkAST: func(t *testing.T, node *types.ASTNode) {
				// Should be: ((a = b) and (b = c))
				if node.Type != types.NodeBinary || node.Value != "and" {
					t.Errorf("Root should be and, got %v", node.Value)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			tt.checkAST(t, node)
		})
	}
}

func TestParseErrorRecovery(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"double operator", "1 + + 2"},
		{"operator at end", "1 + 2 *"},
		{"mismatched brackets", "[1, 2}"},
		{"unclosed string in array", `["unclosed]`},
		{"invalid filter", "items[[price > 100]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectError(t, tt.input)
		})
	}
}

func TestParseSpecialCharacters(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unicode in string", `"hello 世界"`},
		{"escaped quotes", `"hello \"world\""`},
		{"escaped backslash", `"path\\to\\file"`},
		{"newline in string", `"line1\nline2"`},
		{"tab in string", `"col1\tcol2"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			checkNode(t, node, types.NodeString, nil) // Don't check exact value due to escaping
		})
	}
}

// Function call tests

func TestParseFunctionCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		funcName string
		argCount int
	}{
		{"no arguments", "sum()", "sum", 0},
		{"single argument", "abs(-5)", "abs", 1},
		{"multiple arguments", "power(2, 8)", "power", 2},
		{"nested call", "sum(abs(-5), abs(-3))", "sum", 2},
		{"with path", "map(items, getName)", "map", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			checkNode(t, node, types.NodeFunction, tt.funcName)
			if len(node.Arguments) != tt.argCount {
				t.Errorf("Expected %d arguments, got %d", tt.argCount, len(node.Arguments))
			}
		})
	}
}

func TestParseComplexFunctionCalls(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"string argument", `upper("hello")`},
		{"expression argument", "sum(a + b, c * d)"},
		{"array argument", "sum([1, 2, 3])"},
		{"object argument", `merge({"a": 1}, {"b": 2})`},
		{"chained calls", "upper(lower(name))"},
		{"lambda argument", "map(items, function($x) { $x * 2 })"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			if node.Type != types.NodeFunction {
				t.Errorf("Expected function node, got %s", node.Type)
			}
		})
	}
}

// Conditional expression tests

func TestParseConditionals(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple condition", "a > 5 ? 'high' : 'low'"},
		{"with expressions", "price > 100 ? price * 0.9 : price"},
		{"nested condition", "a > 5 ? (b > 10 ? 'very high' : 'high') : 'low'"},
		{"with function", "exists(value) ? value : 'default'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			checkNode(t, node, types.NodeCondition, nil)
			if node.LHS == nil {
				t.Error("Condition (LHS) is nil")
			}
			if node.RHS == nil {
				t.Error("Then branch (RHS) is nil")
			}
			if len(node.Expressions) == 0 || node.Expressions[0] == nil {
				t.Error("Else branch is nil")
			}
		})
	}
}

func TestParseConditionalPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		checkAST func(t *testing.T, node *types.ASTNode)
	}{
		{
			name:  "condition binds loosely",
			input: "a + b > 5 ? 'high' : 'low'",
			checkAST: func(t *testing.T, node *types.ASTNode) {
				// Root should be condition
				if node.Type != types.NodeCondition {
					t.Errorf("Root should be condition, got %s", node.Type)
				}
				// LHS should be comparison
				if node.LHS.Type != types.NodeBinary || node.LHS.Value != ">" {
					t.Errorf("Condition should be >, got %v", node.LHS.Value)
				}
			},
		},
		{
			name:  "multiple conditions",
			input: "a ? b : c ? d : e",
			checkAST: func(t *testing.T, node *types.ASTNode) {
				// Should be: a ? b : (c ? d : e)
				if node.Type != types.NodeCondition {
					t.Errorf("Root should be condition, got %s", node.Type)
				}
				// Else branch should be another condition
				if node.Expressions[0].Type != types.NodeCondition {
					t.Errorf("Else should be condition, got %s", node.Expressions[0].Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			tt.checkAST(t, node)
		})
	}
}

// Lambda function tests

func TestParseLambda(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		paramCount int
	}{
		{"no parameters", "function() { 'value' }", 0},
		{"single parameter", "function($x) { $x * 2 }", 1},
		{"multiple parameters", "function($a, $b) { $a + $b }", 2},
		{"complex body", "function($x) { $x > 5 ? $x * 2 : $x }", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			checkNode(t, node, types.NodeLambda, nil)
			if len(node.Arguments) != tt.paramCount {
				t.Errorf("Expected %d parameters, got %d", tt.paramCount, len(node.Arguments))
			}
			if node.RHS == nil {
				t.Error("Lambda body is nil")
			}
		})
	}
}

func TestParseLambdaInContext(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"in function call", "map(items, function($x) { $x * 2 })"},
		{"in array", "[function($x) { $x + 1 }, function($x) { $x - 1 }]"},
		{"in conditional", "hasFunc ? function($x) { $x * 2 } : function($x) { $x }"},
		{"nested lambda", "function($x) { function($y) { $x + $y } }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			if node == nil {
				t.Fatal("Node is nil")
			}
			// Just verify it parses without error
		})
	}
}

// Range operator tests

func TestParseRange(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple range", "1..10"},
		{"with variables", "$start..$end"},
		{"with expressions", "(a + 1)..(b * 2)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			// Range is represented as binary operator
			if node.Type != types.NodeBinary || node.Value != ".." {
				t.Errorf("Expected range operator, got %s with value %v", node.Type, node.Value)
			}
			if node.LHS == nil || node.RHS == nil {
				t.Error("Range operands are nil")
			}
		})
	}
}

func TestParseRangeInArray(t *testing.T) {
	input := "[1..5, 10..15]"
	node := parseExpr(t, input)

	// Should be an array
	if node.Type != types.NodeArray {
		t.Fatalf("Expected array, got %s", node.Type)
	}

	// Should have 2 elements
	if len(node.Expressions) != 2 {
		t.Fatalf("Expected 2 elements, got %d", len(node.Expressions))
	}

	// Both elements should be range operators
	for i, expr := range node.Expressions {
		if expr.Type != types.NodeBinary || expr.Value != ".." {
			t.Errorf("Element %d: expected range operator, got %s with value %v", i, expr.Type, expr.Value)
		}
	}
}

// Apply operator tests

func TestParseApply(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple apply", "data ~> $sum"},
		{"with function", "items ~> $map(function($x) { $x * 2 })"},
		{"chained apply", "data ~> $filter(exists) ~> $sort"},
		{"with path", "$.items ~> $count"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			// Apply is represented as binary operator
			found := false
			var check func(*types.ASTNode)
			check = func(n *types.ASTNode) {
				if n == nil {
					return
				}
				if n.Type == types.NodeBinary && n.Value == "~>" {
					found = true
					return
				}
				check(n.LHS)
				check(n.RHS)
				for _, e := range n.Expressions {
					check(e)
				}
			}
			check(node)
			if !found {
				t.Error("Apply operator not found in AST")
			}
		})
	}
}

// Assignment tests

func TestParseAssignment(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		varName string
	}{
		{"simple assignment", "$x := 5", "x"},
		{"with expression", "$result := a + b", "result"},
		{"with function", "$sum := $sum(values)", "sum"},
		{"context assignment", "$ := data", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			checkNode(t, node, types.NodeBind, tt.varName)
			if node.LHS == nil || node.RHS == nil {
				t.Error("Assignment operands are nil")
			}
		})
	}
}

func TestParseAssignmentErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"non-variable LHS", "5 := 10"},
		{"name LHS", "name := value"},
		{"expression LHS", "(a + b) := 10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectError(t, tt.input)
		})
	}
}

// Advanced features integration tests

func TestParseAdvancedCombinations(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"function with conditional", "map(items, function($x) { $x > 5 ? $x * 2 : $x })"},
		{"conditional with range", "useRange ? 1..10 : [1, 2, 3]"},
		{"apply with lambda", "data ~> function($d) { $d.items }"},
		{"assignment in conditional", "needsInit ? $val := 0 : $val"},
		{"nested functions", "filter(map(items, function($x) { $x * 2 }), function($x) { $x > 10 })"},
		{"all operators", "$result := data.items[price > 100] ~> map(function($x) { $x.quantity }) ~> sum()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			if node == nil {
				t.Fatal("Node is nil")
			}
			// Just verify it parses without error
		})
	}
}

func TestParseAdvancedPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		checkAST func(t *testing.T, node *types.ASTNode)
	}{
		{
			name:  "range vs arithmetic",
			input: "1 + 2..5 + 6",
			checkAST: func(t *testing.T, node *types.ASTNode) {
				// Should be: (1 + 2)..(5 + 6)
				// Range has lower precedence than +
				if node.Type != types.NodeBinary || node.Value != ".." {
					t.Errorf("Root should be .., got %v", node.Value)
				}
			},
		},
		{
			name:  "apply vs other operators",
			input: "a + b ~> func",
			checkAST: func(t *testing.T, node *types.ASTNode) {
				// Should be: (a + b) ~> func
				// Apply has very low precedence
				if node.Type != types.NodeBinary || node.Value != "~>" {
					t.Errorf("Root should be ~>, got %v", node.Value)
				}
			},
		},
		{
			name:  "assignment lowest precedence",
			input: "$x := a + b * c",
			checkAST: func(t *testing.T, node *types.ASTNode) {
				// Should be: $x := (a + (b * c))
				if node.Type != types.NodeBind {
					t.Errorf("Root should be assignment, got %s", node.Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseExpr(t, tt.input)
			tt.checkAST(t, node)
		})
	}
}
