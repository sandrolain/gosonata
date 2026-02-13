package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sandrolain/gosonata/tests/conformance/importer"
)

func main() {
	var (
		groupName = flag.String("group", "", "Group name to import (e.g., comparison-operators)")
		allGroups = flag.Bool("all", false, "Import all groups")
		listOnly  = flag.Bool("list", false, "List available groups without importing")
		stats     = flag.Bool("stats", false, "Show statistics about test suite")
		basePath  = flag.String("base", "thirdy/jsonata/test/test-suite", "Base path to test suite")
		outputDir = flag.String("output", "tests/conformance/imported", "Output directory for generated tests")
	)

	flag.Parse()

	// Get absolute paths
	absBasePath, err := filepath.Abs(*basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving base path: %v\n", err)
		os.Exit(1)
	}

	absOutputDir, err := filepath.Abs(*outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving output directory: %v\n", err)
		os.Exit(1)
	}

	loader := importer.NewLoader(absBasePath)

	// List groups
	if *listOnly {
		if err := listGroups(loader); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Show stats
	if *stats {
		if err := showStats(loader); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	generator, err := importer.NewGenerator(absOutputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create generator: %v\n", err)
		os.Exit(1)
	}

	// Import all groups
	if *allGroups {
		if err := importAllGroups(loader, generator); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Import single group
	if *groupName == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := importGroup(loader, generator, *groupName); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func listGroups(loader *importer.Loader) error {
	groups, err := loader.ListGroups()
	if err != nil {
		return err
	}

	fmt.Printf("Available test groups (%d):\n\n", len(groups))
	for i, group := range groups {
		info, err := loader.LoadGroup(group)
		if err != nil {
			fmt.Printf("%3d. %-40s ERROR: %v\n", i+1, group, err)
			continue
		}
		fmt.Printf("%3d. %-40s (%3d tests)\n", i+1, group, info.TestCount)
	}

	return nil
}

func showStats(loader *importer.Loader) error {
	fmt.Println("Analyzing test suite...")
	
	stats, err := loader.GetStats()
	if err != nil {
		return err
	}

	fmt.Println("\nTest Suite Statistics")
	fmt.Println("=====================")
	fmt.Printf("Total Groups:      %d\n", stats.TotalGroups)
	fmt.Printf("Processed Groups:  %d\n", stats.ProcessedGroups)
	fmt.Printf("Total Test Cases:  %d\n", stats.TotalTestCases)
	fmt.Printf("Successful Import: %d\n", stats.SuccessfulImport)
	fmt.Printf("Failed Import:     %d\n", stats.FailedImport)

	if len(stats.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, err := range stats.Errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	return nil
}

func importGroup(loader *importer.Loader, generator *importer.Generator, groupName string) error {
	fmt.Printf("Importing group: %s\n", groupName)

	// Load group
	info, err := loader.LoadGroup(groupName)
	if err != nil {
		return fmt.Errorf("failed to load group: %w", err)
	}

	fmt.Printf("Loaded %d test cases\n", info.TestCount)

	// Generate test file
	outputPath, err := generator.GenerateGroup(info)
	if err != nil {
		return fmt.Errorf("failed to generate tests: %w", err)
	}

	fmt.Printf("Generated: %s\n", outputPath)
	fmt.Println("✓ Import complete")

	return nil
}

func importAllGroups(loader *importer.Loader, generator *importer.Generator) error {
	groups, err := loader.ListGroups()
	if err != nil {
		return err
	}

	fmt.Printf("Importing all %d groups...\n\n", len(groups))

	var successful, failed int

	for i, groupName := range groups {
		fmt.Printf("[%d/%d] %s... ", i+1, len(groups), groupName)

		info, err := loader.LoadGroup(groupName)
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
			failed++
			continue
		}

		_, err = generator.GenerateGroup(info)
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
			failed++
			continue
		}

		fmt.Printf("OK (%d tests)\n", info.TestCount)
		successful++
	}

	fmt.Printf("\n=====================\n")
	fmt.Printf("Successful: %d\n", successful)
	fmt.Printf("Failed:     %d\n", failed)

	return nil
}
