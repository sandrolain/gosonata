package loader

import "encoding/json"

// TestCase represents a single test case from the official suite
type TestCase struct {
	ID              string                 `json:"id"`
	Expr            string                 `json:"expr"`
	ExprFile        string                 `json:"expr-file"` // File to read expression from
	Dataset         *string                `json:"dataset"`   // null or dataset name
	Data            json.RawMessage        `json:"data,omitempty"`
	Bindings        map[string]interface{} `json:"bindings"`
	Result          interface{}            `json:"result,omitempty"`
	UndefinedResult bool                   `json:"undefinedResult"`
	Error           *ErrorInfo             `json:"error,omitempty"`
	Code            string                 `json:"code,omitempty"`   // Error code (direct field when no error object)
	Timelimit       *int                   `json:"timelimit"`
	Depth           *int                   `json:"depth"`
	Unordered       bool                   `json:"unordered"`
	Token           string                 `json:"token,omitempty"`
}

// ErrorInfo represents expected error condition
type ErrorInfo struct {
	Code    string      `json:"code"`
	Message string      `json:"message,omitempty"`
	Cause   interface{} `json:"cause,omitempty"`
}

// TestGroup represents all test cases in a group directory
type TestGroup struct {
	Name  string
	Path  string
	Cases []*TestCase
}

// TestSuite represents the complete test suite with all groups and datasets
type TestSuite struct {
	Groups   []*TestGroup
	Datasets map[string]interface{}
	Total    int
}

// TestResult represents the result of executing a test case
type TestResult struct {
	Passed      bool
	Expected    interface{}
	Actual      interface{}
	Error       error
	ErrorCode   string
	Message     string
	DurationMs  float64
}
