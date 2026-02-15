package loader

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

// LoadAllDatasets loads all datasets into a map
func LoadAllDatasets(datasetsDir string) (map[string]interface{}, error) {
	datasets := make(map[string]interface{})

	// Check if datasets directory exists
	if _, err := ioutil.ReadDir(datasetsDir); err != nil {
		// Datasets directory may not exist or be empty
		return datasets, nil
	}

	entries, err := ioutil.ReadDir(datasetsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read datasets directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		datasetName := entry.Name()[:len(entry.Name())-5] // Remove .json

		data, err := ioutil.ReadFile(filepath.Join(datasetsDir, entry.Name()))
		if err != nil {
			fmt.Printf("Warning: Failed to load dataset %s: %v\n", entry.Name(), err)
			continue
		}

		var dataset interface{}
		if err := json.Unmarshal(data, &dataset); err != nil {
			fmt.Printf("Warning: Failed to unmarshal dataset %s: %v\n", entry.Name(), err)
			continue
		}

		datasets[datasetName] = dataset
	}

	return datasets, nil
}

// GetData resolves test case data (either inline or from dataset)
func GetData(testCase *TestCase, datasets map[string]interface{}) (interface{}, error) {
	// Case 1: Inline data provided
	if testCase.Data != nil && len(testCase.Data) > 0 {
		var data interface{}
		if err := json.Unmarshal(testCase.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal inline data: %w", err)
		}
		return data, nil
	}

	// Case 2: Dataset reference
	if testCase.Dataset != nil {
		if *testCase.Dataset == "" {
			return nil, nil // null dataset means undefined data
		}

		data, ok := datasets[*testCase.Dataset]
		if !ok {
			return nil, fmt.Errorf("dataset not found: %s", *testCase.Dataset)
		}
		return data, nil
	}

	// Case 3: No data
	return nil, nil
}
