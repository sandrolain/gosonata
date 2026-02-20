package types

import "fmt"

// ErrorCode represents a JSONata error code.
type ErrorCode string

// Error codes based on JSONata reference implementation.
const (
	// S0xxx: Parser/Syntax errors
	ErrStringNotClosed    ErrorCode = "S0101"
	ErrNumberOutOfRange   ErrorCode = "S0102"
	ErrUnsupportedEscape  ErrorCode = "S0103"
	ErrUnexpectedEnd      ErrorCode = "S0104"
	ErrCommentNotClosed   ErrorCode = "S0106"
	ErrSyntaxError        ErrorCode = "S0201"
	ErrExpectedToken      ErrorCode = "S0202"
	ErrExpectedKeyword    ErrorCode = "S0203"
	ErrInvalidPathStep    ErrorCode = "S0213" // digit field name after dot
	ErrContextVarIllegal  ErrorCode = "S0214" // @ or # not followed by $var
	ErrContextAfterFilter ErrorCode = "S0215" // @ cannot follow a filter predicate
	ErrContextAfterSort   ErrorCode = "S0216" // @ cannot follow an order-by clause
	ErrInvalidParentUse   ErrorCode = "S0217" // parent operator (%) in invalid context
	ErrEmptyRegex         ErrorCode = "S0301"
	ErrRegexNotClosed     ErrorCode = "S0302"
	// T0xxx: Type errors
	ErrArgumentCountMismatch ErrorCode = "T0410"
	ErrCannotConvertNumber   ErrorCode = "T1001"
	ErrCannotConvertString   ErrorCode = "T1002"
	ErrInvalidTypeOperation  ErrorCode = "T1003"

	// T2xxx: Operator type errors
	ErrLeftSideAssignment    ErrorCode = "T2001"
	ErrRangeStartNotInteger  ErrorCode = "T2003"
	ErrRangeEndNotInteger    ErrorCode = "T2004"
	ErrSortNotComparable     ErrorCode = "T2007"
	ErrSortMixedTypes        ErrorCode = "T2008"
	ErrTransformUpdateNotObj ErrorCode = "T2011"
	ErrTransformDeleteNotArr ErrorCode = "T2012"

	// D0xxx: Evaluation errors
	ErrNumberTooLarge         ErrorCode = "D1001"
	ErrInvokeNonFunction      ErrorCode = "D1002"
	ErrZeroLengthMatch        ErrorCode = "D1004"
	ErrLeftSideRange          ErrorCode = "D2001"
	ErrRangeTooLarge          ErrorCode = "D2014"
	ErrSerializeNonFinite     ErrorCode = "D3001"
	ErrRecursiveDefinition    ErrorCode = "D3010"
	ErrReplacementNotString   ErrorCode = "D3012"
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
