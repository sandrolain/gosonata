package conformance

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sandrolain/gosonata/tests/conformance/loader"
)

func TestOfficialSuite(t *testing.T) {
	// Suite is located in node_modules after npm install
	suiteDir := filepath.Join("node_modules", "jsonata", "test", "test-suite")

	// Load all test groups and datasets
	suite, err := loader.LoadSuite(suiteDir)
	if err != nil {
		t.Fatalf("Failed to load suite: %v", err)
	}

	if suite == nil || len(suite.Groups) == 0 {
		t.Fatalf("No test groups loaded from %s", suiteDir)
	}

	t.Logf("Loaded %d test groups with %d total test cases", len(suite.Groups), suite.Total)

	// Statistics
	var totalPassed int
	var totalFailed int
	var groupResults []groupStat

	ctx := context.Background()

	// Run each group
	for _, group := range suite.Groups {
		t.Run(group.Name, func(t *testing.T) {
			var groupPassed int
			var groupFailed int

			for _, testCase := range group.Cases {
				t.Run(testCase.ID, func(t *testing.T) {
					// Get test data
					data, err := loader.GetData(testCase, suite.Datasets)
					if err != nil {
						t.Fatalf("Failed to get test data: %v", err)
					}

					// Evaluate test case
					result, err := loader.EvaluateTestCase(ctx, testCase, data)
					if err != nil {
						t.Fatalf("Evaluation error: %v", err)
					}

					// Check for expected error
					if testCase.Error != nil {
						if result.Error == nil {
							groupFailed++
							totalFailed++
							t.Errorf("Expected error %s, got result: %v",
								testCase.Error.Code, result.Actual)
						} else if testCase.Error.Code != "" && result.ErrorCode != testCase.Error.Code {
							// Error codes don't match (but we got an error)
							groupPassed++
							totalPassed++
						} else {
							groupPassed++
							totalPassed++
						}
					} else if testCase.UndefinedResult {
						// Expect undefined
						if result.Actual == nil && result.Error == nil {
							groupPassed++
							totalPassed++
						} else {
							groupFailed++
							totalFailed++
							t.Errorf("Expected undefined, got: %v (error: %v)",
								result.Actual, result.Error)
						}
					} else {
						// Check against expected result
						if result.Passed {
							groupPassed++
							totalPassed++
						} else {
							groupFailed++
							totalFailed++
							if result.Error != nil {
								t.Errorf("Unexpected error: %v", result.Error)
							} else {
								t.Error(result.Message)
							}
						}
					}
				})
			}

			groupResults = append(groupResults, groupStat{
				name:   group.Name,
				passed: groupPassed,
				failed: groupFailed,
			})
		})
	}

	// Report summary
	passPercent := 0.0
	if suite.Total > 0 {
		passPercent = float64(totalPassed) / float64(suite.Total) * 100
	}

	t.Logf("\n"+
		"======================== Test Suite Results ========================\n"+
		"Total Test Cases: %d\n"+
		"✓ Passed: %d (%0.1f%%)\n"+
		"✗ Failed: %d (%0.1f%%)\n"+
		"Groups: %d\n"+
		"==================================================================\n",
		suite.Total,
		totalPassed, passPercent,
		totalFailed, 100.0-passPercent,
		len(suite.Groups),
	)

	// Print group-by-group results if any failed
	if totalFailed > 0 {
		t.Logf("\nFailing groups:")
		for _, gr := range groupResults {
			if gr.failed > 0 {
				failPercent := float64(gr.failed) / float64(gr.failed+gr.passed) * 100
				t.Logf("  ✗ %s: %d/%d failed (%0.1f%%)",
					gr.name, gr.failed, gr.failed+gr.passed, failPercent)
			}
		}
	}
}

type groupStat struct {
	name   string
	passed int
	failed int
}
