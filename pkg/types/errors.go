package types

import "fmt"

// ErrorCode represents a JSONata error code.
type ErrorCode string

// Error codes based on JSONata reference implementation.
const (
	// S0xxx: Parser/Syntax errors
	ErrStringNotClosed   ErrorCode = "S0101"
	ErrNumberOutOfRange  ErrorCode = "S0102"
	ErrUnsupportedEscape ErrorCode = "S0103"
	ErrUnexpectedEnd     ErrorCode = "S0104"
	ErrCommentNotClosed  ErrorCode = "S0106"
	ErrSyntaxError       ErrorCode = "S0201"
	ErrExpectedToken     ErrorCode = "S0202"
	ErrExpectedKeyword   ErrorCode = "S0203"
	ErrEmptyRegex        ErrorCode = "S0301"
	ErrRegexNotClosed    ErrorCode = "S0302"

	// T0xxx: Type errors
	ErrArgumentCountMismatch ErrorCode = "T0410"
	ErrCannotConvertNumber   ErrorCode = "T1001"
	ErrCannotConvertString   ErrorCode = "T1002"
	ErrInvalidTypeOperation  ErrorCode = "T1003"
	ErrLeftSideAssignment    ErrorCode = "T2001"

	// D0xxx: Evaluation errors
	ErrNumberTooLarge         ErrorCode = "D1001"
	ErrInvokeNonFunction      ErrorCode = "D1002"
	ErrLeftSideRange          ErrorCode = "D2001"
	ErrSerializeNonFinite     ErrorCode = "D3001"
	ErrRecursiveDefinition    ErrorCode = "D3010"
	ErrStackOverflow          ErrorCode = "D3020"
	ErrReduceInsufficientArgs ErrorCode = "D3050"
	ErrTypeMismatch           ErrorCode = "D3070"
	ErrSingleMultipleMatches  ErrorCode = "D3138"
	ErrSingleNoMatch          ErrorCode = "D3139"
	ErrEncodeURISurrogate     ErrorCode = "D3140"

	// U0xxx: Runtime errors
	ErrUndefinedVariable ErrorCode = "U1001"
	ErrUndefinedFunction ErrorCode = "U1002"
)

// Error represents a structured JSONata error.
type Error struct {
	Code     ErrorCode
	Message  string
	Position int
	Token    string
	Err      error
}

// NewError creates a new JSONata error.
func NewError(code ErrorCode, message string, position int) *Error {
	return &Error{
		Code:     code,
		Message:  message,
		Position: position,
	}
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Position >= 0 {
		return fmt.Sprintf("%s at position %d: %s", e.Code, e.Position, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error.
func (e *Error) Unwrap() error {
	return e.Err
}

// WithToken adds token information to the error.
func (e *Error) WithToken(token string) *Error {
	e.Token = token
	return e
}

// WithCause wraps another error.
func (e *Error) WithCause(err error) *Error {
	e.Err = err
	return e
}
