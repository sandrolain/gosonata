package parser

import (
	"fmt"
	"unicode/utf8"

	"github.com/sandrolain/gosonata/pkg/types"
)

const eof = -1

// Lexer converts a JSONata expression into a sequence of tokens.
// The implementation is based on Rob Pike's "Lexical Scanning in Go" technique.
type Lexer struct {
	input   string // Input string being scanned
	length  int    // Length of input string
	start   int    // Start position of current token
	current int    // Current position in input
	width   int    // Width of last rune read
	err     error  // First error encountered
}

// NewLexer creates a new lexer from the provided input string.
// The input is tokenized by successive calls to the Next method.
func NewLexer(input string) *Lexer {
	return &Lexer{
		input:  input,
		length: len(input),
	}
}

// Next returns the next token from the input.
// When the end of the input is reached, Next returns TokenEOF for all subsequent calls.
//
// The allowRegex parameter determines how forward slashes are interpreted.
// Forward slashes in JSONata can be either:
//   - The start of a regular expression (when allowRegex is true)
//   - The division operator (when allowRegex is false)
//
// The parser must track context to determine which interpretation is correct.
func (l *Lexer) Next(allowRegex bool) Token {
	l.skipWhitespace()

	// Check if skipWhitespace encountered an error (e.g., unclosed comment)
	if l.err != nil {
		return l.error(types.ErrCommentNotClosed, l.err.Error())
	}

	ch := l.nextRune()
	if ch == eof {
		return l.eof()
	}

	// Handle regex vs division operator
	if allowRegex && ch == '/' {
		l.ignore()
		return l.scanRegex(ch)
	}

	// Check for two-character symbols first (e.g., !=, <=, ..)
	if rts := lookupSymbol2(ch); rts != nil {
		for _, rt := range rts {
			if l.acceptRune(rt.r) {
				return l.newToken(rt.tt)
			}
		}
	}

	// Check for single-character symbols
	if tt := lookupSymbol1(ch); tt > 0 {
		return l.newToken(tt)
	}

	// String literals (single or double quoted)
	if ch == '"' || ch == '\'' {
		l.ignore()
		return l.scanString(ch)
	}

	// Number literals
	if ch >= '0' && ch <= '9' {
		l.backup()
		return l.scanNumber()
	}

	// Escaped field names (backtick quoted)
	if ch == '`' {
		l.ignore()
		return l.scanEscapedName(ch)
	}

	// Names, variables, keywords, or error
	l.backup()
	return l.scanName()
}

// Error returns the first error encountered during lexing, if any.
func (l *Lexer) Error() error {
	return l.err
}

// scanRegex reads a regular expression from the current position.
// The opening delimiter has already been consumed.
// Format: /pattern/flags where flags can be i, m, s
func (l *Lexer) scanRegex(delim rune) Token {
	var depth int

Loop:
	for {
		switch l.nextRune() {
		case delim:
			if depth == 0 {
				break Loop
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '\\':
			// Consume escaped character
			if r := l.nextRune(); r != eof && r != '\n' {
				break
			}
			fallthrough
		case eof, '\n':
			return l.error(types.ErrRegexNotClosed, "Unterminated regex")
		}
	}

	l.backup()
	t := l.newToken(TokenRegex)
	l.acceptRune(delim)
	l.ignore()

	// Convert JavaScript-style regex flags to Go format
	// e.g., /ab+/i becomes (?i)ab+
	if l.acceptAll(isRegexFlag) {
		flags := l.newToken(TokenType(0))
		t.Value = fmt.Sprintf("(?%s)%s", flags.Value, t.Value)
	}

	return t
}

// scanString reads a string literal from the current position.
// The opening quote has already been consumed.
// Supports both single and double quotes with escape sequences.
func (l *Lexer) scanString(quote rune) Token {
Loop:
	for {
		switch l.nextRune() {
		case quote:
			break Loop
		case '\\':
			// Consume escaped character
			if r := l.nextRune(); r != eof {
				break
			}
			fallthrough
		case eof:
			return l.error(types.ErrStringNotClosed, "Unterminated string literal")
		}
	}

	l.backup()
	t := l.newToken(TokenString)
	l.acceptRune(quote)
	l.ignore()
	return t
}

// scanNumber reads a number literal from the current position.
// Supports integers, decimals, and scientific notation.
// Format: [+-]?[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?
func (l *Lexer) scanNumber() Token {
	// JSON does not support leading zeroes.
	// The integer part is either a single zero, or
	// a non-zero digit followed by zero or more digits.
	if !l.acceptRune('0') {
		l.accept(isNonZeroDigit)
		l.acceptAll(isDigit)
	}

	// Decimal part
	if l.acceptRune('.') {
		if !l.acceptAll(isDigit) {
			// If there are no digits after the decimal point,
			// don't treat the dot as part of the number.
			// It could be part of the range operator (e.g., "1..5").
			l.backup()
			return l.newToken(TokenNumber)
		}
	}

	// Exponent part
	if l.acceptRunes2('e', 'E') {
		l.acceptRunes2('+', '-')
		l.acceptAll(isDigit)
	}

	return l.newToken(TokenNumber)
}

// scanEscapedName reads an escaped field name from the current position.
// The opening backtick has already been consumed.
// Format: `field name with spaces or special chars`
func (l *Lexer) scanEscapedName(quote rune) Token {
Loop:
	for {
		switch l.nextRune() {
		case quote:
			break Loop
		case eof, '\n':
			return l.error(types.ErrUnsupportedEscape, "Unterminated name")
		}
	}

	l.backup()
	t := l.newToken(TokenNameEsc)
	l.acceptRune(quote)
	l.ignore()
	return t
}

// scanName reads a name, variable, or keyword from the current position.
// Names can contain letters, digits, and underscores.
// Variables start with $ (e.g., $var, $$).
// Keywords are: and, or, in, true, false, null
func (l *Lexer) scanName() Token {
	isVar := l.acceptRune('$')
	if isVar {
		l.ignore()
	}

	for {
		ch := l.nextRune()
		if ch == eof {
			break
		}

		// Stop at whitespace
		if isWhitespace(ch) {
			l.backup()
			break
		}

		// Stop at operators
		if lookupSymbol1(ch) > 0 || lookupSymbol2(ch) != nil {
			l.backup()
			break
		}
	}

	t := l.newToken(TokenName)

	if isVar {
		t.Type = TokenVariable
	} else if tt := lookupKeyword(t.Value); tt > 0 {
		t.Type = tt
	}

	return t
}

// Helper methods

func (l *Lexer) eof() Token {
	return Token{
		Type:     TokenEOF,
		Position: l.current,
	}
}

func (l *Lexer) error(code types.ErrorCode, message string) Token {
	t := l.newToken(TokenError)
	l.err = &types.Error{
		Code:     code,
		Message:  message,
		Position: t.Position,
		Token:    t.Value,
	}
	return t
}

func (l *Lexer) newToken(tt TokenType) Token {
	t := Token{
		Type:     tt,
		Value:    l.input[l.start:l.current],
		Position: l.start,
	}
	l.width = 0
	l.start = l.current
	return t
}

func (l *Lexer) nextRune() rune {
	if l.err != nil || l.current >= l.length {
		l.width = 0
		return eof
	}

	r, w := utf8.DecodeRuneInString(l.input[l.current:])
	l.width = w
	l.current += w
	return r
}

func (l *Lexer) backup() {
	l.current -= l.width
}

func (l *Lexer) ignore() {
	l.start = l.current
}

func (l *Lexer) acceptRune(r rune) bool {
	return l.accept(func(c rune) bool {
		return c == r
	})
}

func (l *Lexer) acceptRunes2(r1, r2 rune) bool {
	return l.accept(func(c rune) bool {
		return c == r1 || c == r2
	})
}

func (l *Lexer) accept(isValid func(rune) bool) bool {
	if isValid(l.nextRune()) {
		return true
	}
	l.backup()
	return false
}

func (l *Lexer) acceptAll(isValid func(rune) bool) bool {
	var matched bool
	for l.accept(isValid) {
		matched = true
	}
	return matched
}

func (l *Lexer) skipWhitespace() {
	for {
		// If an error occurred (e.g., unclosed comment), stop
		if l.err != nil {
			return
		}

		// Skip whitespace
		l.acceptAll(isWhitespace)
		l.ignore()

		// Check for comment start /*
		if l.acceptRune('/') {
			if l.acceptRune('*') {
				// Found comment start /*
				// Scan until we find */
				for {
					ch := l.nextRune()
					if ch == eof {
						l.err = &types.Error{
							Code:     types.ErrCommentNotClosed,
							Message:  "Unclosed comment",
							Position: l.current,
						}
						return
					}
					if ch == '*' {
						if l.acceptRune('/') {
							// Found comment end */
							break
						}
					}
				}
				l.ignore()
			} else {
				// Not a comment, backup
				l.backup()
				break
			}
		} else {
			// No '/', no comment
			break
		}
	}
}

// Character classification functions

func isWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\v':
		return true
	default:
		return false
	}
}

func isRegexFlag(r rune) bool {
	switch r {
	case 'i', 'm', 's':
		return true
	default:
		return false
	}
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isNonZeroDigit(r rune) bool {
	return r >= '1' && r <= '9'
}
