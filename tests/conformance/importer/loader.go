package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	TestSuiteBasePath = "thirdy/jsonata/test/test-suite"
	GroupsPath        = "groups"
	DatasetsPath      = "datasets"
)

// Loader carica test cases dalla test suite ufficiale.
type Loader struct {
	basePath string
	datasets map[string]interface{}
}

// NewLoader crea un nuovo loader.
func NewLoader(basePath string) *Loader {
	if basePath == "" {
		basePath = TestSuiteBasePath
	}
	return &Loader{
		basePath: basePath,
		datasets: make(map[string]interface{}),
	}
}

// ListGroups elenca tutti i gruppi disponibili.
func (l *Loader) ListGroups() ([]string, error) {
	groupsPath := filepath.Join(l.basePath, GroupsPath)
	
	entries, err := os.ReadDir(groupsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read groups directory: %w", err)
	}
	
	var groups []string
	for _, entry := range entries {
		if entry.IsDir() {
			groups = append(groups, entry.Name())
		}
	}
	
	sort.Strings(groups)
	return groups, nil
}

// LoadGroup carica tutti i test cases di un gruppo.
func (l *Loader) LoadGroup(groupName string) (*GroupInfo, error) {
	groupPath := filepath.Join(l.basePath, GroupsPath, groupName)
	
	// Verifica che il gruppo esista
	if _, err := os.Stat(groupPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("group not found: %s", groupName)
	}
	
	// Lista tutti i file case*.json
	pattern := filepath.Join(groupPath, "case*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob test files: %w", err)
	}
	
	if len(matches) == 0 {
		return nil, fmt.Errorf("no test cases found in group: %s", groupName)
	}
	
	sort.Strings(matches)
	
	info := &GroupInfo{
		Name:      groupName,
		Path:      groupPath,
		TestCount: len(matches),
		TestCases: make([]TestCase, 0, len(matches)),
	}
	
	// Carica ogni test case
	for _, match := range matches {
		caseNumber := extractCaseNumber(match)
		testCase, err := l.loadTestCase(groupName, match, caseNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", match, err)
		}
		info.TestCases = append(info.TestCases, *testCase)
	}
	
	return info, nil
}

// loadTestCase carica un singolo test case.
func (l *Loader) loadTestCase(groupName, filePath string, caseNumber int) (*TestCase, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	var official OfficialTestCase
	if err := json.Unmarshal(data, &official); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	testCase := &TestCase{
		GroupName:  groupName,
		CaseNumber: caseNumber,
		FileName:   filepath.Base(filePath),
		Expression: official.Expr,
		Bindings:   official.Bindings,
		TimeLimit:  official.Timelimit,
		DepthLimit: official.Depth,
	}
	
	// Resolve input data
	if official.Dataset != nil {
		// Load dataset
		datasetData, err := l.LoadDataset(*official.Dataset)
		if err != nil {
			return nil, fmt.Errorf("failed to load dataset %s: %w", *official.Dataset, err)
		}
		testCase.InputData = datasetData
	} else {
		// Use inline data
		testCase.InputData = official.Data
	}
	
	// Resolve expected result
	if official.Code != nil {
		// Expect error
		testCase.ShouldError = true
		testCase.ErrorCode = *official.Code
		if official.Token != nil {
			testCase.ErrorToken = *official.Token
		}
	} else if official.UndefinedResult != nil && *official.UndefinedResult {
		// Expect undefined
		testCase.IsUndefined = true
		testCase.Expected = nil
	} else {
		// Expect result
		testCase.Expected = official.Result
	}
	
	return testCase, nil
}

// LoadDataset carica un dataset specifico.
func (l *Loader) LoadDataset(datasetName string) (interface{}, error) {
	// Check cache
	if data, ok := l.datasets[datasetName]; ok {
		return data, nil
	}
	
	// Load from file
	datasetPath := filepath.Join(l.basePath, DatasetsPath, datasetName+".json")
	fileData, err := os.ReadFile(datasetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read dataset file: %w", err)
	}
	
	var data interface{}
	if err := json.Unmarshal(fileData, &data); err != nil {
		return nil, fmt.Errorf("failed to parse dataset JSON: %w", err)
	}
	
	// Cache it
	l.datasets[datasetName] = data
	
	return data, nil
}

// GetStats restituisce statistiche sui test disponibili.
func (l *Loader) GetStats() (*ImportStats, error) {
	groups, err := l.ListGroups()
	if err != nil {
		return nil, err
	}
	
	stats := &ImportStats{
		TotalGroups: len(groups),
	}
	
	for _, group := range groups {
		info, err := l.LoadGroup(group)
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("%s: %v", group, err))
			continue
		}
		stats.ProcessedGroups++
		stats.TotalTestCases += info.TestCount
		stats.SuccessfulImport += info.TestCount
	}
	
	return stats, nil
}

// extractCaseNumber estrae il numero del case dal filename.
func extractCaseNumber(filename string) int {
	base := filepath.Base(filename)
	// case001.json -> 001
	numStr := strings.TrimSuffix(strings.TrimPrefix(base, "case"), ".json")
	num, _ := strconv.Atoi(numStr)
	return num
}
