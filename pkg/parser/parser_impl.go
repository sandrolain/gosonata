package parser

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"

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
	// Check for lexer errors (e.g., unclosed comment)
	if p.current.Type == TokenError {
		return nil, p.lexer.Error()
	}

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
	TokenCoalesce:     26, // ?? (coalescing, between or and and)
	TokenDefault:      26, // ?: (default operator, same as coalesce)
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
	TokenDescendent:   75, // ** (descendant operator, same as dot)
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
// Regexes appear after: =, !=, ~>, commas, opening brackets/parens, and at the start.
func (p *Parser) isRegexContext() bool {
	switch p.current.Type {
	case TokenEqual, TokenNotEqual, TokenApply:
		return true
	case TokenComma, TokenParenOpen, TokenBracketOpen:
		return true // Arguments and array elements
	case TokenColon:
		return true // Object values
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
	case TokenDescendent:
		// ** in prefix position means descendent from current context ($.**)
		return p.parseDescendentPrefix()
	case TokenMult:
		// * in prefix position means wildcard (all fields/values)
		return p.parseWildcard()
	case TokenRegex:
		return p.parseRegex()
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
	case TokenDescendent:
		return p.parseDescendent(left)
	case TokenBracketOpen:
		return p.parseFilter(left)
	case TokenBraceOpen:
		return p.parseObjectConstructorWithLeft(left)
	case TokenParenOpen:
		// Function call: any expression that could evaluate to a function can be called
		return p.parseFunctionCall(left)
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
		TokenAnd, TokenOr, TokenIn, TokenCoalesce, TokenDefault:
		return p.parseBinaryOp(left)
	default:
		return nil, p.error("S0201", fmt.Sprintf("Unexpected infix token: %s", token.Type.String()))
	}
}

// unescapeString processes escape sequences in a string literal.
// Handles standard escapes (\n, \t, etc.) and Unicode escapes (\uXXXX).
// Also handles UTF-16 surrogate pairs for characters outside the BMP.
func unescapeString(s string) (string, error) {
	if !strings.Contains(s, "\\") {
		return s, nil // Fast path: no escapes
	}

	var result strings.Builder
	result.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			result.WriteByte(s[i])
			continue
		}

		i++ // Skip backslash
		if i >= len(s) {
			return "", fmt.Errorf("Invalid escape sequence at end of string")
		}

		switch s[i] {
		case 'n':
			result.WriteByte('\n')
		case 't':
			result.WriteByte('\t')
		case 'r':
			result.WriteByte('\r')
		case 'b':
			result.WriteByte('\b')
		case 'f':
			result.WriteByte('\f')
		case '\\':
			result.WriteByte('\\')
		case '"':
			result.WriteByte('"')
		case '\'':
			result.WriteByte('\'')
		case '/':
			result.WriteByte('/')
		case 'u':
			// Unicode escape: \uXXXX
			if i+4 >= len(s) {
				return "", fmt.Errorf("Invalid \\u escape: not enough characters")
			}
			hex := s[i+1 : i+5]
			codePoint, err := strconv.ParseUint(hex, 16, 16)
			if err != nil {
				return "", fmt.Errorf("Invalid \\u escape: %s", hex)
			}
			i += 4

			r := rune(codePoint)

			// Check for UTF-16 surrogate pair (high surrogate: 0xD800-0xDBFF)
			if r >= 0xD800 && r <= 0xDBFF {
				// This is a high surrogate, expect a low surrogate next
				if i+6 >= len(s) || s[i+1] != '\\' || s[i+2] != 'u' {
					// Invalid surrogate pair
					result.WriteRune(r)
				} else {
					// Read low surrogate
					lowHex := s[i+3 : i+7]
					lowCodePoint, err := strconv.ParseUint(lowHex, 16, 16)
					if err != nil {
						result.WriteRune(r)
					} else {
						lowSurrogate := rune(lowCodePoint)
						if lowSurrogate >= 0xDC00 && lowSurrogate <= 0xDFFF {
							// Valid surrogate pair, decode to code point
							codePoint := utf16.Decode([]uint16{uint16(r), uint16(lowSurrogate)})
							if len(codePoint) > 0 {
								result.WriteRune(codePoint[0])
								i += 6 // Skip \uXXXX
							} else {
								result.WriteRune(r)
							}
						} else {
							// Not a low surrogate
							result.WriteRune(r)
						}
					}
				}
			} else {
				// Normal Unicode character
				result.WriteRune(r)
			}
		default:
			// Unknown escape - this is an error in JSONata
			return "", fmt.Errorf("Invalid escape sequence: \\%c", s[i])
		}
	}

	return result.String(), nil
}

// parseString parses a string literal.
func (p *Parser) parseString() (*types.ASTNode, error) {
	node := types.NewASTNode(types.NodeString, p.current.Position)

	// Process escape sequences
	unescaped, err := unescapeString(p.current.Value)
	if err != nil {
		return nil, p.error("S0103", fmt.Sprintf("Invalid string literal: %v", err))
	}

	node.Value = unescaped
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

// parseDescendent parses a descendent expression (recursive field navigation).
func (p *Parser) parseDescendent(left *types.ASTNode) (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance() // Skip '**'

	// Skip optional '.' after '**' (e.g., foo.**.blah is same as foo.**blah)
	if p.current.Type == TokenDot {
		p.advance()
	}

	// Parse the right-hand side
	right, err := p.parseExpression(precedence[TokenDescendent])
	if err != nil {
		return nil, err
	}

	node := types.NewASTNode(types.NodeDescendant, pos)
	node.LHS = left
	node.RHS = right

	// Propagate KeepArray flag from left side
	if left.KeepArray {
		node.KeepArray = true
	}

	return node, nil
}

// parseDescendentPrefix parses a descendent operator in prefix position.
// ** in prefix means search all descendants of the current context.
func (p *Parser) parseDescendentPrefix() (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance() // Skip '**'

	// Skip optional '.' after '**' (e.g., **.blah is same as **blah)
	if p.current.Type == TokenDot {
		p.advance()
	}

	// Create implicit current context reference ($)
	left := types.NewASTNode(types.NodeVariable, pos)
	left.Value = ""

	// If there's a right-hand side, parse it
	var right *types.ASTNode
	var err error

	// Check if the next token can be part of the path (NOT [ which is a filter)
	if p.current.Type != TokenEOF &&
		p.current.Type != TokenSemicolon &&
		p.current.Type != TokenParenClose &&
		p.current.Type != TokenBracketClose &&
		p.current.Type != TokenBracketOpen &&
		p.current.Type != TokenBraceClose &&
		p.current.Type != TokenComma &&
		p.current.Type != TokenDot {
		// Parse the right-hand side with descendent precedence
		right, err = p.parseExpression(precedence[TokenDescendent])
		if err != nil {
			return nil, err
		}
	}

	node := types.NewASTNode(types.NodeDescendant, pos)
	node.LHS = left
	node.RHS = right

	return node, nil
}

// parseWildcard parses a wildcard operator (*).
// The wildcard returns all values in an object or all elements in an array.
func (p *Parser) parseWildcard() (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance() // Skip '*'

	node := types.NewASTNode(types.NodeWildcard, pos)
	return node, nil
}

// parseRegex parses a regular expression literal.
func (p *Parser) parseRegex() (*types.ASTNode, error) {
	node := types.NewASTNode(types.NodeRegex, p.current.Position)
	node.Value = p.current.Value // Pattern with flags already converted by lexer
	p.advance()
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

	// For named function calls (name), use Value to store the name for built-in lookup.
	// For all other expressions (lambda, variable, function-result, etc.), use LHS.
	if nameNode.Type == types.NodeName {
		node.Value = nameNode.Value // Function name (string)
	} else {
		node.LHS = nameNode // Lambda, variable, function-result, or other expression to call
	}

	node.Arguments = []*types.ASTNode{}
	hasPlaceholder := false

	// Parse arguments
	if p.current.Type != TokenParenClose {
		for {
			// Check for placeholder ? in argument position
			if p.current.Type == TokenCondition {
				// '?' is a placeholder in function argument context
				placeholder := types.NewASTNode(types.NodePlaceholder, p.current.Position)
				node.Arguments = append(node.Arguments, placeholder)
				hasPlaceholder = true
				p.advance() // Skip '?'
			} else {
				arg, err := p.parseExpression(0)
				if err != nil {
					return nil, err
				}
				node.Arguments = append(node.Arguments, arg)
			}

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

	// If function has placeholders, convert to partial application
	if hasPlaceholder {
		node.Type = types.NodePartial
	}

	return node, nil
}

// parseConditional parses a conditional (ternary) expression.
// Syntax: condition ? then_expr : else_expr
// The else part is optional: condition ? then_expr
func (p *Parser) parseConditional(condition *types.ASTNode) (*types.ASTNode, error) {
	pos := p.current.Position
	p.advance() // Skip '?'

	// Parse 'then' expression
	thenExpr, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}

	node := types.NewASTNode(types.NodeCondition, pos)
	node.LHS = condition // Condition
	node.RHS = thenExpr  // Then branch

	// Check if there's an else part (optional)
	if p.current.Type == TokenColon {
		p.advance() // Skip ':'

		// Parse 'else' expression (right-associative, so use precedence - 1)
		elseExpr, err := p.parseExpression(precedence[TokenCondition] - 1)
		if err != nil {
			return nil, err
		}
		node.Expressions = []*types.ASTNode{elseExpr} // Else branch
	} else {
		// No else part: equivalent to undefined
		node.Expressions = nil
	}

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

	// Check for optional function signature
	// Syntax: <type1-type2-...:returnType>
	// Example: <n-n:n> means two number params, returns number
	var signatureStr string
	if p.current.Type == TokenLess {
		sigStart := "<"
		p.advance() // Skip '<'

		// Consume all tokens until '>' and build signature string
		depth := 1
		for depth > 0 && p.current.Type != TokenEOF {
			if p.current.Type == TokenLess {
				depth++
				sigStart += "<"
			} else if p.current.Type == TokenGreater {
				depth--
				if depth > 0 {
					sigStart += ">"
				}
			} else {
				// Add token value to signature string
				sigStart += p.current.Value
			}
			if depth > 0 {
				p.advance()
			}
		}

		if p.current.Type != TokenGreater {
			return nil, p.error("S0202", "Expected '>' to close function signature")
		}

		sigStart += ">"
		signatureStr = sigStart
		node.Signature = signatureStr
		p.advance() // Skip '>'
	}

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
	case TokenCoalesce:
		return "??"
	default:
		return tt.String()
	}
}
