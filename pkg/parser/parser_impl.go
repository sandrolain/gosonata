package parser

import (
	"fmt"
	"strconv"

	"github.com/sandrolain/gosonata/pkg/types"
)

// Parser implements a recursive descent parser for JSONata expressions.
// It uses Pratt's "Top Down Operator Precedence" algorithm to handle
// operator precedence correctly.
type Parser struct {
	lexer   *Lexer
	current Token
	prev    Token
	errors  []error
	opts    CompileOptions
}

// NewParser creates a new parser for the given input string.
func NewParser(input string, opts ...CompileOption) *Parser {
	options := CompileOptions{
		EnableRecovery: false,
		MaxDepth:       100,
	}
	for _, opt := range opts {
		opt(&options)
	}

	p := &Parser{
		lexer: NewLexer(input),
		opts:  options,
	}

	// Read the first token
	p.advance()

	return p
}

// Parse parses the entire expression and returns the root AST node.
func (p *Parser) Parse() (*types.Expression, error) {
	if p.current.Type == TokenEOF {
		return nil, p.error("S0201", "Empty expression")
	}

	node, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}

	if p.current.Type != TokenEOF {
		return nil, p.error("S0201", fmt.Sprintf("Unexpected token: %s", p.current.Value))
	}

	return types.NewExpression(node, p.lexer.input), nil
}

// Operator precedence table (binding power)
// Higher values bind more tightly
var precedence = map[TokenType]int{
	TokenOr:           25, // or
	TokenAnd:          30, // and
	TokenEqual:        40, // =
	TokenNotEqual:     40, // !=
	TokenLess:         40, // <
	TokenLessEqual:    40, // <=
	TokenGreater:      40, // >
	TokenGreaterEqual: 40, // >=
	TokenIn:           40, // in
	TokenConcat:       50, // &
	TokenRange:        45, // ..
	TokenApply:        20, // ~>
	TokenAssign:       10, // :=
	TokenPlus:         50, // +
	TokenMinus:        50, // -
	TokenMult:         60, // *
	TokenDiv:          60, // /
	TokenMod:          60, // %
	TokenCondition:    15, // ?
	TokenSort:         70, // ^ (sort operator)
	TokenDot:          75, // .
	TokenBracketOpen:  80, // [
	TokenBraceOpen:    80, // {
	TokenParenOpen:    80, // (
}

// getPrecedence returns the precedence of a token type.
func (p *Parser) getPrecedence(tt TokenType) int {
	if prec, ok := precedence[tt]; ok {
		return prec
	}
	return 0
}

// advance moves to the next token.
func (p *Parser) advance() {
	p.prev = p.current
	// For regex: only allow after operators that expect a value on the right
	allowRegex := p.isRegexContext()
	p.current = p.lexer.Next(allowRegex)
}

// isRegexContext determines if we're in a context where a regex is expected.
// Regexes appear after: =, !=, ~>, and at the start of an expression.
func (p *Parser) isRegexContext() bool {
	switch p.current.Type {
	case TokenEqual, TokenNotEqual, TokenApply:
		return true
	case TokenEOF:
		return true // Start of expression
	default:
		return false
	}
}

// expect checks if the current token matches the expected type and advances.
func (p *Parser) expect(tt TokenType) error {
	if p.current.Type != tt {
		return p.error("S0202", fmt.Sprintf("Expected %s but got %s", tt.String(), p.current.Type.String()))
	}
	p.advance()
	return nil
}

// error creates a parser error.
func (p *Parser) error(code types.ErrorCode, message string) error {
	err := &types.Error{
		Code:     code,
		Message:  message,
		Position: p.current.Position,
		Token:    p.current.Value,
	}
	p.errors = append(p.errors, err)
	return err
}

// parseExpression parses an expression with operator precedence.
// rbp is the right binding power (minimum precedence).
func (p *Parser) parseExpression(rbp int) (*types.ASTNode, error) {
	// Parse prefix expression (nud - null denotation)
	left, err := p.parsePrefix()
	if err != nil {
		return nil, err
	}

	// Parse infix expressions while precedence allows (led - left denotation)
	for rbp < p.getPrecedence(p.current.Type) {
		left, err = p.parseInfix(left)
		if err != nil {
			return nil, err
		}
	}

	return left, nil
}

// parsePrefix parses a prefix expression (nud - null denotation).
// These are expressions that don't require a left-hand side.
func (p *Parser) parsePrefix() (*types.ASTNode, error) {
	token := p.current

	switch token.Type {
	case TokenString:
		return p.parseString()
	case TokenNumber:
		return p.parseNumber()
	case TokenBoolean:
		return p.parseBoolean()
	case TokenNull:
		return p.parseNull()
	case TokenName, TokenNameEsc:
		// Check if it's a lambda (function keyword or λ character)
		if token.Value == "function" || token.Value == "λ" {
			return p.parseLambda()
		}
		return p.parseName()
	case TokenVariable:
		return p.parseVariable()
	case TokenMinus:
		return p.parseUnaryMinus()
	case TokenLess, TokenGreater:
		// These can be unary operators (e.g., <$ means ascending sort, >$ means descending)
		return p.parseUnaryComparison()
	case TokenMod:
		// % in prefix position is the parent operator
		return p.parseParent()
	case TokenParenOpen:
		return p.parseGrouping()
	case TokenBracketOpen:
		return p.parseArrayConstructor()
	case TokenBraceOpen:
		return p.parseObjectConstructor()
	case TokenAnd, TokenOr, TokenIn:
		// Keywords can also be used as field names (e.g., 'and', 'or', 'in')
		// Treat them as names when in prefix position
		return p.parseNameFromKeyword()
	default:
		return nil, p.error("S0201", fmt.Sprintf("Unexpected token: %s", token.Type.String()))
	}
}

// parseInfix parses an infix expression (led - left denotation).
// These are expressions that require a left-hand side.
func (p *Parser) parseInfix(left *types.ASTNode) (*types.ASTNode, error) {
	token := p.current

	switch token.Type {
	case TokenDot:
		return p.parsePath(left)
	case TokenBracketOpen:
		return p.parseFilter(left)
	case TokenBraceOpen:
		return p.parseObjectConstructorWithLeft(left)
	case TokenParenOpen:
		// Function call: name(...) or $var(...)
		if left.Type == types.NodeName || left.Type == types.NodeVariable {
			return p.parseFunctionCall(left)
		}
		return nil, p.error("S0201", "Unexpected '(' after non-function expression")
	case TokenCondition:
		return p.parseConditional(left)
	case TokenRange:
		return p.parseRange(left)
	case TokenApply:
		return p.parseApply(left)
	case TokenSort:
		return p.parseSort(left)
	case TokenAssign:
		return p.parseAssignment(left)
	case TokenPlus, TokenMinus, TokenMult, TokenDiv, TokenMod,
		TokenEqual, TokenNotEqual, TokenLess, TokenLessEqual,
		TokenGreater, TokenGreaterEqual, TokenConcat,
		TokenAnd, TokenOr, TokenIn:
		return p.parseBinaryOp(left)
	default:
		return nil, p.error("S0201", fmt.Sprintf("Unexpected infix token: %s", token.Type.String()))
	}
}

// parseString parses a string literal.
func (p *Parser) parseString() (*types.ASTNode, error) {
	node := types.NewASTNode(types.NodeString, p.current.Position)
	node.Value = p.current.Value
	p.advance()
	return node, nil
}

// parseNumber parses a number literal.
func (p *Parser) parseNumber() (*types.ASTNode, error) {
	node := types.NewASTNode(types.NodeNumber, p.current.Position)

	// Parse the number
	val, err := strconv.ParseFloat(p.current.Value, 64)
	if err != nil {
		return nil, p.error("S0102", fmt.Sprintf("Invalid number: %s", p.current.Value))
	}

	node.Value = val
	p.advance()
	return node, nil
}

// parseBoolean parses a boolean literal.
func (p *Parser) parseBoolean() (*types.ASTNode, error) {
	node := types.NewASTNode(types.NodeBoolean, p.current.Position)
	node.Value = p.current.Value == "true"
	p.advance()
	return node, nil
}

// parseNull parses a null literal.
func (p *Parser) parseNull() (*types.ASTNode, error) {
	node := types.NewASTNode(types.NodeNull, p.current.Position)
	node.Value = types.NullValue
	p.advance()
	return node, nil
}

// parseName parses a field name.
func (p *Parser) parseName() (*types.ASTNode, error) {
	node := types.NewASTNode(types.NodeName, p.current.Position)
	node.Value = p.current.Value
	p.advance()
	return node, nil
}

// parseNameFromKeyword treats a keyword token (and, or, in) as a field name.
// This allows keywords to be used as identifiers in certain contexts.
func (p *Parser) parseNameFromKeyword() (*types.ASTNode, error) {
	node := types.NewASTNode(types.NodeName, p.current.Position)
	// Use the string representation of the keyword as the name
	node.Value = p.current.Type.String()
	p.advance()
	return node, nil
}

// parseVariable parses a variable reference.
func (p *Parser) parseVariable() (*types.ASTNode, error) {
	node := types.NewASTNode(types.NodeVariable, p.current.Position)
	node.Value = p.current.Value // Empty string for $, or variable name
	p.advance()
	return node, nil
}

// parseUnaryMinus parses a unary minus operator.
func (p *Parser) parseUnaryMinus() (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance()

	// Parse the expression with high precedence
	expr, err := p.parseExpression(70)
	if err != nil {
		return nil, err
	}

	node := types.NewASTNode(types.NodeUnary, pos)
	node.Value = "-"
	node.LHS = expr

	return node, nil
}

// parseUnaryComparison parses unary comparison operators (< and >) used in sort context.
// Examples: <$ (ascending sort), >$ (descending sort)
func (p *Parser) parseUnaryComparison() (*types.ASTNode, error) {
	pos := p.current.Position
	op := p.current.Value // "<" or ">"
	p.advance()

	// Parse the expression with high precedence
	expr, err := p.parseExpression(70)
	if err != nil {
		return nil, err
	}

	node := types.NewASTNode(types.NodeUnary, pos)
	node.Value = op
	node.LHS = expr

	return node, nil
}

// parseParent parses the parent operator (%).
// Syntax: % or %.%.%... to reach different levels
// Examples: %.field, %.%.field (grandparent), etc.
func (p *Parser) parseParent() (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance() // Skip '%'

	node := types.NewASTNode(types.NodeParent, pos)
	node.Value = "%"

	// Count consecutive % to allow %.% for grandparent, %.%.% for great-grandparent, etc.
	// We'll let parseInfix handle chaining: %.%. creates NodePath with NodeParent as left side

	return node, nil
}

// parseGrouping parses a parenthesized expression or block.
// A block is one or more expressions separated by semicolons.
// Each block creates a new lexical scope for variable bindings.
// Single expressions without semicolons are returned directly (pure grouping).
func (p *Parser) parseGrouping() (*types.ASTNode, error) {
	startPos := p.current.Position
	p.advance() // Skip '('

	if p.current.Type == TokenParenClose {
		// Empty grouping () represents empty sequence (undefined)
		node := types.NewASTNode(types.NodeNull, p.current.Position)
		node.Value = nil
		p.advance()
		return node, nil
	}

	var exprs []*types.ASTNode
	hasSemicolon := false

	// Parse expressions separated by semicolons
	for p.current.Type != TokenParenClose {
		expr, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)

		// If no semicolon, we're done with the sequence
		if p.current.Type != TokenSemicolon {
			break
		}
		hasSemicolon = true
		p.advance() // Skip ';'
	}

	if err := p.expect(TokenParenClose); err != nil {
		return nil, err
	}

	// If there's only one expression and no semicolons, we still need to check
	// if the expression requires a new scope (like variable assignment).
	// According to JSONata semantics, parentheses isolate variable assignments
	// even with a single expression, so we need to wrap in a block.
	if len(exprs) == 1 && !hasSemicolon {
		// Check if the expression is a bind (assignment) - if so, wrap in block
		if exprs[0].Type == types.NodeBind {
			// Wrap in block to create a new scope
			blockNode := types.NewASTNode(types.NodeBlock, startPos)
			blockNode.Expressions = exprs
			return blockNode, nil
		}
		// For other single expressions, return directly (pure grouping)
		return exprs[0], nil
	}

	// Otherwise, wrap in a block node to establish a lexical scope
	blockNode := types.NewASTNode(types.NodeBlock, startPos)
	blockNode.Expressions = exprs

	return blockNode, nil
}

// parseArrayConstructor parses an array constructor [...].
func (p *Parser) parseArrayConstructor() (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance() // Skip '['

	node := types.NewASTNode(types.NodeArray, pos)
	node.Expressions = []*types.ASTNode{}

	if p.current.Type == TokenBracketClose {
		p.advance()
		return node, nil
	}

	for {
		expr, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		node.Expressions = append(node.Expressions, expr)

		if p.current.Type == TokenBracketClose {
			p.advance()
			break
		}

		if err := p.expect(TokenComma); err != nil {
			return nil, err
		}
	}

	return node, nil
}

// parseObjectConstructor parses an object constructor {...}.
func (p *Parser) parseObjectConstructor() (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance() // Skip '{'

	node := types.NewASTNode(types.NodeObject, pos)
	node.Expressions = []*types.ASTNode{}

	if p.current.Type == TokenBraceClose {
		p.advance()
		return node, nil
	}

	for {
		// Parse key expression
		key, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}

		if err := p.expect(TokenColon); err != nil {
			return nil, err
		}

		// Parse value expression
		value, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}

		// Create a pair node (we use a binary node with special marker)
		pair := types.NewASTNode(types.NodeBinary, key.Position)
		pair.Value = ":"
		pair.LHS = key
		pair.RHS = value

		node.Expressions = append(node.Expressions, pair)

		if p.current.Type == TokenBraceClose {
			p.advance()
			break
		}

		if err := p.expect(TokenComma); err != nil {
			return nil, err
		}
	}

	return node, nil
}

// parseObjectConstructorWithLeft parses an object constructor with a left expression.
// Syntax: expr{key: value}
func (p *Parser) parseObjectConstructorWithLeft(left *types.ASTNode) (*types.ASTNode, error) {
	node, err := p.parseObjectConstructor()
	if err != nil {
		return nil, err
	}
	node.LHS = left
	node.IsGrouping = true // Mark as infix grouping
	return node, nil
}

// parsePath parses a path expression (field navigation).
func (p *Parser) parsePath(left *types.ASTNode) (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance() // Skip '.'

	// Parse the right-hand side
	right, err := p.parseExpression(precedence[TokenDot])
	if err != nil {
		return nil, err
	}

	node := types.NewASTNode(types.NodePath, pos)
	node.LHS = left
	node.RHS = right

	// Propagate KeepArray flag from left side (e.g., from $[] syntax)
	//This ensures array structure is preserved through path chains
	if left.KeepArray {
		node.KeepArray = true
	}

	return node, nil
}

// parseFilter parses a filter/predicate expression or array accessor.
func (p *Parser) parseFilter(left *types.ASTNode) (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance() // Skip '['

	// Check for empty brackets [] - means flatten/iterate
	if p.current.Type == TokenBracketClose {
		p.advance() // Skip ']'
		// Empty filter means "return all items" - use NodeFilter with nil RHS
		node := types.NewASTNode(types.NodeFilter, pos)
		node.LHS = left
		node.RHS = nil        // nil filter means "return all"
		node.KeepArray = true // Preserve array structure through path evaluation
		return node, nil
	}

	// Parse the filter expression
	filter, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}

	if err := p.expect(TokenBracketClose); err != nil {
		return nil, err
	}

	node := types.NewASTNode(types.NodeFilter, pos)
	node.LHS = left
	node.RHS = filter

	return node, nil
}

// parseBinaryOp parses a binary operator expression.
func (p *Parser) parseBinaryOp(left *types.ASTNode) (*types.ASTNode, error) {
	op := p.current
	prec := p.getPrecedence(op.Type)
	p.advance()

	// Parse the right-hand side with appropriate precedence
	right, err := p.parseExpression(prec)
	if err != nil {
		return nil, err
	}

	node := types.NewASTNode(types.NodeBinary, op.Position)
	node.Value = operatorString(op.Type)
	node.LHS = left
	node.RHS = right

	return node, nil
}

// parseFunctionCall parses a function call expression.
// Called when we see name followed by '('.
func (p *Parser) parseFunctionCall(nameNode *types.ASTNode) (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance() // Skip '('

	node := types.NewASTNode(types.NodeFunction, pos)
	node.Value = nameNode.Value // Function name
	node.Arguments = []*types.ASTNode{}

	// Parse arguments
	if p.current.Type != TokenParenClose {
		for {
			arg, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			node.Arguments = append(node.Arguments, arg)

			if p.current.Type == TokenParenClose {
				break
			}

			if err := p.expect(TokenComma); err != nil {
				return nil, err
			}
		}
	}

	if err := p.expect(TokenParenClose); err != nil {
		return nil, err
	}

	return node, nil
}

// parseConditional parses a conditional (ternary) expression.
// Syntax: condition ? then_expr : else_expr
func (p *Parser) parseConditional(condition *types.ASTNode) (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance() // Skip '?'

	// Parse 'then' expression
	thenExpr, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}

	// Expect ':'
	if err := p.expect(TokenColon); err != nil {
		return nil, err
	}

	// Parse 'else' expression (right-associative, so use precedence - 1)
	elseExpr, err := p.parseExpression(precedence[TokenCondition] - 1)
	if err != nil {
		return nil, err
	}

	node := types.NewASTNode(types.NodeCondition, pos)
	node.LHS = condition                          // Condition
	node.RHS = thenExpr                           // Then branch
	node.Expressions = []*types.ASTNode{elseExpr} // Else branch

	return node, nil
}

// parseLambda parses a lambda function expression.
// Syntax: function($param1, $param2) { body }
func (p *Parser) parseLambda() (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance() // Skip 'function' keyword

	node := types.NewASTNode(types.NodeLambda, pos)
	node.Arguments = []*types.ASTNode{} // Parameters

	// Expect '('
	if err := p.expect(TokenParenOpen); err != nil {
		return nil, err
	}

	// Parse parameters (variables)
	if p.current.Type != TokenParenClose {
		for {
			if p.current.Type != TokenVariable {
				return nil, p.error("S0201", "Expected variable in lambda parameter list")
			}

			param := types.NewASTNode(types.NodeVariable, p.current.Position)
			param.Value = p.current.Value
			node.Arguments = append(node.Arguments, param)
			p.advance()

			if p.current.Type == TokenParenClose {
				break
			}

			if err := p.expect(TokenComma); err != nil {
				return nil, err
			}
		}
	}

	p.advance() // Skip ')'

	// Expect '{'
	if err := p.expect(TokenBraceOpen); err != nil {
		return nil, err
	}

	// Parse body expression
	body, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}
	node.RHS = body

	// Expect '}'
	if err := p.expect(TokenBraceClose); err != nil {
		return nil, err
	}

	return node, nil
}

// parseRange parses a range expression.
// Syntax: start..end
func (p *Parser) parseRange(left *types.ASTNode) (*types.ASTNode, error) {
	pos := p.current.Position
	prec := p.getPrecedence(TokenRange)
	p.advance() // Skip '..'

	// Parse right-hand side
	right, err := p.parseExpression(prec)
	if err != nil {
		return nil, err
	}

	node := types.NewASTNode(types.NodeBinary, pos)
	node.Value = ".."
	node.LHS = left
	node.RHS = right

	return node, nil
}

// parseApply parses an apply expression.
// Syntax: expr ~> function
func (p *Parser) parseApply(left *types.ASTNode) (*types.ASTNode, error) {
	pos := p.current.Position
	prec := p.getPrecedence(TokenApply)
	p.advance() // Skip '~>'

	// Parse right-hand side (function to apply)
	right, err := p.parseExpression(prec)
	if err != nil {
		return nil, err
	}

	node := types.NewASTNode(types.NodeBinary, pos)
	node.Value = "~>"
	node.LHS = left
	node.RHS = right

	return node, nil
}

// parseSort parses a sort expression.
// Syntax: expr^(sort-key)
// Examples: Price^($), items^(>quantity), data^(<count)
func (p *Parser) parseSort(left *types.ASTNode) (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance() // Skip '^'

	// Expect opening parenthesis
	if p.current.Type != TokenParenOpen {
		return nil, p.error("S0201", "Expected '(' after '^' operator")
	}
	p.advance() // Skip '('

	// Parse the sort key expression
	sortKey, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}

	// Expect closing parenthesis
	if p.current.Type != TokenParenClose {
		return nil, p.error("S0201", "Expected ')' in sort expression")
	}
	p.advance() // Skip ')'

	// Create a NodeSort node with left expression and sort key
	node := types.NewASTNode(types.NodeSort, pos)
	node.LHS = left    // The sequence to sort
	node.RHS = sortKey // The sort key expression
	node.Value = "^"

	return node, nil
}

// parseAssignment parses an assignment expression.
// Syntax: $var := value
// Right-associative: $a := $b := 5 is ($a := ($b := 5))
func (p *Parser) parseAssignment(left *types.ASTNode) (*types.ASTNode, error) {
	if left.Type != types.NodeVariable {
		return nil, p.error("S0201", "Left-hand side of assignment must be a variable")
	}

	pos := p.current.Position
	prec := p.getPrecedence(TokenAssign)
	p.advance() // Skip ':='

	// Parse right-hand side with lower precedence to allow right-associativity
	// Using prec-1 allows chained assignments like $a := $b := 5
	right, err := p.parseExpression(prec - 1)
	if err != nil {
		return nil, err
	}

	node := types.NewASTNode(types.NodeBind, pos)
	node.Value = left.Value // Variable name
	node.LHS = left         // Variable node
	node.RHS = right        // Value expression

	return node, nil
}

// operatorString returns the string representation of an operator token.

func operatorString(tt TokenType) string {
	switch tt {
	case TokenPlus:
		return "+"
	case TokenMinus:
		return "-"
	case TokenMult:
		return "*"
	case TokenDiv:
		return "/"
	case TokenMod:
		return "%"
	case TokenEqual:
		return "="
	case TokenNotEqual:
		return "!="
	case TokenLess:
		return "<"
	case TokenLessEqual:
		return "<="
	case TokenGreater:
		return ">"
	case TokenGreaterEqual:
		return ">="
	case TokenConcat:
		return "&"
	case TokenAnd:
		return "and"
	case TokenOr:
		return "or"
	case TokenIn:
		return "in"
	case TokenRange:
		return ".."
	case TokenApply:
		return "~>"
	default:
		return tt.String()
	}
}
