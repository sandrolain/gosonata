package loader

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
)

// LoadAllTestGroups loads all test groups from the suite directory
func LoadAllTestGroups(suiteDir string) ([]*TestGroup, error) {
	groupsPath := filepath.Join(suiteDir, "groups")

	entries, err := ioutil.ReadDir(groupsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read groups directory: %w", err)
	}

	var groups []*TestGroup

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		group, err := LoadTestGroup(groupsPath, entry.Name())
		if err != nil {
			// Skip on error but log
			fmt.Fprintf(os.Stderr, "Warning: Failed to load group %s: %v\n", entry.Name(), err)
			continue
		}

		groups = append(groups, group)
	}

	// Sort by group name for consistent output
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	return groups, nil
}

// LoadTestGroup loads all test cases from a single group directory
func LoadTestGroup(groupsPath, groupName string) (*TestGroup, error) {
	groupPath := filepath.Join(groupsPath, groupName)

	entries, err := ioutil.ReadDir(groupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read group directory: %w", err)
	}

	group := &TestGroup{
		Name: groupName,
		Path: groupPath,
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			caseID := entry.Name()[:len(entry.Name())-5] // Remove .json

			testCase, err := LoadTestCase(groupPath, caseID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to load case %s/%s: %v\n",
					groupName, entry.Name(), err)
				continue
			}

			testCase.ID = caseID
			group.Cases = append(group.Cases, testCase)
		}
	}

	// Sort cases by ID
	sort.Slice(group.Cases, func(i, j int) bool {
		return group.Cases[i].ID < group.Cases[j].ID
	})

	return group, nil
}

// LoadTestCase loads a single test case JSON file
// The file can contain either a single TestCase object or an array of TestCase objects
func LoadTestCase(groupPath, caseID string) (*TestCase, error) {
	caseFile := filepath.Join(groupPath, caseID+".json")

	data, err := ioutil.ReadFile(caseFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Try to unmarshal as single TestCase first
	var testCase TestCase
	if err := json.Unmarshal(data, &testCase); err == nil {
		return &testCase, nil
	}

	// If that fails, try to unmarshal as array and take first item
	var testCases []TestCase
	if err := json.Unmarshal(data, &testCases); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON as object or array: %w", err)
	}

	if len(testCases) == 0 {
		return nil, fmt.Errorf("empty test case array")
	}

	return &testCases[0], nil
}

// LoadSuite loads the entire suite (groups + datasets)
func LoadSuite(suiteDir string) (*TestSuite, error) {
	groups, err := LoadAllTestGroups(suiteDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load groups: %w", err)
	}

	datasets, err := LoadAllDatasets(filepath.Join(suiteDir, "datasets"))
	if err != nil {
		return nil, fmt.Errorf("failed to load datasets: %w", err)
	}

	total := 0
	for _, g := range groups {
		total += len(g.Cases)
	}

	return &TestSuite{
		Groups:   groups,
		Datasets: datasets,
		Total:    total,
	}, nil
}
