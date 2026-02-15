package parser

// TokenType represents the type of a lexical token.
type TokenType uint8

const (
	// Special tokens
	TokenEOF TokenType = iota
	TokenError

	// Literals
	TokenString   // "hello" or 'hello'
	TokenNumber   // 123, 3.14, 1e-10
	TokenBoolean  // true, false
	TokenNull     // null
	TokenName     // fieldName
	TokenNameEsc  // `field name with spaces`
	TokenVariable // $var, $$
	TokenRegex    // /pattern/flags

	// Grouping symbols
	TokenBracketOpen  // [
	TokenBracketClose // ]
	TokenBraceOpen    // {
	TokenBraceClose   // }
	TokenParenOpen    // (
	TokenParenClose   // )

	// Basic symbols
	TokenDot       // .
	TokenComma     // ,
	TokenColon     // :
	TokenSemicolon // ;
	TokenCondition // ?

	// Arithmetic operators
	TokenPlus  // +
	TokenMinus // -
	TokenMult  // *
	TokenDiv   // /
	TokenMod   // %

	// Other operators
	TokenPipe   // |
	TokenSort   // ^
	TokenConcat // &

	// Comparison operators
	TokenEqual        // =
	TokenNotEqual     // !=
	TokenLess         // <
	TokenLessEqual    // <=
	TokenGreater      // >
	TokenGreaterEqual // >=

	// Special operators
	TokenRange      // ..
	TokenApply      // ~>
	TokenAssign     // :=
	TokenDescendent // **
	TokenCoalesce   // ??

	// Keyword operators
	TokenAnd // and
	TokenOr  // or
	TokenIn  // in
)

// String returns a string representation of the token type.
func (tt TokenType) String() string {
	switch tt {
	case TokenEOF:
		return "(eof)"
	case TokenError:
		return "(error)"
	case TokenString:
		return "(string)"
	case TokenNumber:
		return "(number)"
	case TokenBoolean:
		return "(boolean)"
	case TokenNull:
		return "(null)"
	case TokenName, TokenNameEsc:
		return "(name)"
	case TokenVariable:
		return "(variable)"
	case TokenRegex:
		return "(regex)"
	case TokenBracketOpen:
		return "["
	case TokenBracketClose:
		return "]"
	case TokenBraceOpen:
		return "{"
	case TokenBraceClose:
		return "}"
	case TokenParenOpen:
		return "("
	case TokenParenClose:
		return ")"
	case TokenDot:
		return "."
	case TokenComma:
		return ","
	case TokenColon:
		return ":"
	case TokenSemicolon:
		return ";"
	case TokenCondition:
		return "?"
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
	case TokenPipe:
		return "|"
	case TokenSort:
		return "^"
	case TokenConcat:
		return "&"
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
	case TokenRange:
		return ".."
	case TokenApply:
		return "~>"
	case TokenAssign:
		return ":="
	case TokenDescendent:
		return "**"
	case TokenCoalesce:
		return "??"
	case TokenAnd:
		return "and"
	case TokenOr:
		return "or"
	case TokenIn:
		return "in"
	default:
		return "(unknown)"
	}
}

// Token represents a lexical token in a JSONata expression.
type Token struct {
	Type     TokenType // Type of the token
	Value    string    // Literal value of the token
	Position int       // Starting position in the input string
}

// symbols1 maps single-character symbols to token types.
var symbols1 = [...]TokenType{
	'[': TokenBracketOpen,
	']': TokenBracketClose,
	'{': TokenBraceOpen,
	'}': TokenBraceClose,
	'(': TokenParenOpen,
	')': TokenParenClose,
	'.': TokenDot,
	',': TokenComma,
	';': TokenSemicolon,
	':': TokenColon,
	'?': TokenCondition,
	'+': TokenPlus,
	'-': TokenMinus,
	'*': TokenMult,
	'/': TokenDiv,
	'%': TokenMod,
	'|': TokenPipe,
	'=': TokenEqual,
	'<': TokenLess,
	'>': TokenGreater,
	'^': TokenSort,
	'&': TokenConcat,
}

// runeTokenType pairs a rune with its corresponding token type.
type runeTokenType struct {
	r  rune
	tt TokenType
}

// symbols2 maps two-character symbol sequences to token types.
// The key is the first character of the sequence.
var symbols2 = [...][]runeTokenType{
	'!': {{'=', TokenNotEqual}},
	'<': {{'=', TokenLessEqual}},
	'>': {{'=', TokenGreaterEqual}},
	'.': {{'.', TokenRange}},
	'~': {{'>', TokenApply}},
	':': {{'=', TokenAssign}},
	'*': {{'*', TokenDescendent}},
	'?': {{'?', TokenCoalesce}},
}

const (
	symbol1Count = rune(len(symbols1))
	symbol2Count = rune(len(symbols2))
)

// lookupSymbol1 returns the token type for a single-character symbol.
// Returns 0 if the rune is not a valid symbol.
func lookupSymbol1(r rune) TokenType {
	if r < 0 || r >= symbol1Count {
		return 0
	}
	return symbols1[r]
}

// lookupSymbol2 returns possible two-character symbol completions.
// Returns nil if the rune cannot start a two-character symbol.
func lookupSymbol2(r rune) []runeTokenType {
	if r < 0 || r >= symbol2Count {
		return nil
	}
	return symbols2[r]
}

// lookupKeyword returns the token type for a keyword.
// Returns 0 if the string is not a recognized keyword.
func lookupKeyword(s string) TokenType {
	switch s {
	case "and":
		return TokenAnd
	case "or":
		return TokenOr
	case "in":
		return TokenIn
	case "true", "false":
		return TokenBoolean
	case "null":
		return TokenNull
	default:
		return 0
	}
}
