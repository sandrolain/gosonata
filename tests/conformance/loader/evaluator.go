package loader

import (
	"context"
	"fmt"
	"time"

	"github.com/sandrolain/gosonata/pkg/evaluator"
	"github.com/sandrolain/gosonata/pkg/parser"
)

// EvaluateTestCase executes a test case against GoSonata
func EvaluateTestCase(ctx context.Context, testCase *TestCase, data interface{}) (*TestResult, error) {
	start := time.Now()

	// Apply timeout if specified
	if testCase.Timelimit != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx,
			time.Duration(*testCase.Timelimit)*time.Millisecond)
		defer cancel()
	}

	// Parse expression
	expr, err := parser.Parse(testCase.Expr)
	if err != nil {
		return &TestResult{
			Passed:     false,
			Error:      err,
			Message:    fmt.Sprintf("parse error: %v", err),
			DurationMs: time.Since(start).Seconds() * 1000,
		}, nil
	}

	// Create evaluator
	eval := evaluator.New()

	// Evaluate expression with bindings
	result, err := eval.EvalWithBindings(ctx, expr, data, testCase.Bindings)

	duration := time.Since(start)

	// Prepare result
	testResult := &TestResult{
		DurationMs: duration.Seconds() * 1000,
	}

	if err != nil {
		// Error occurred
		testResult.Passed = false
		testResult.Error = err
		testResult.Message = err.Error()

		// Try to extract error code from GoSonata error
		if errObj, ok := err.(interface{ Code() string }); ok {
			testResult.ErrorCode = errObj.Code()
		}

		return testResult, nil
	}

	// Success - compare result
	testResult.Actual = result
	testResult.Expected = testCase.Result
	passed, msg := CompareResults(result, testCase.Result, *testCase)
	testResult.Passed = passed
	if !passed {
		testResult.Message = msg
	}

	return testResult, nil
}
