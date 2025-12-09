package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"chainguard.dev/wolfi-vm/vms-test/pkg/artifacts"
	"github.com/spf13/cobra"
)

// MetricRow represents a row in the metrics output
type MetricRow struct {
	Name     string
	Function string
	ID       string
	Value    string
}

func extractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract <results-dir> [output-dir]",
		Short: "Extract and decode artifacts from test results",
		Long: `Extract metrics and files from test artifact JSON files.

This command processes all *-artifacts.json files in the results directory,
decodes base64-encoded file content, and creates a human-readable structure:
  - metrics.txt: Tab-delimited file with all metrics
  - <Name>/<Function>/<FileID>: Decoded file contents

If output-dir is not specified, it defaults to results-dir.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: runExtract,
	}

	return cmd
}

func runExtract(cmd *cobra.Command, args []string) error {
	resultsDir := args[0]
	outputDir := resultsDir
	if len(args) >= 2 {
		outputDir = args[1]
	}

	return extractArtifacts(resultsDir, outputDir)
}

func extractArtifacts(resultsDir, outputDir string) error {
	// Find all *-artifacts.json files in the results directory
	pattern := filepath.Join(resultsDir, "*-artifacts.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob for artifacts: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("no artifact files found in %s", resultsDir)
	}

	var allMetrics []MetricRow
	fileCount := 0

	// Process each artifact file
	for _, artifactFile := range matches {

		info, err := os.Stat(artifactFile)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", artifactFile, err)
		}
		if info.Size() == 0 {
			fmt.Printf("emptty %s: ignoring\n", artifactFile)
			continue
		}

		metrics, files, err := processArtifactFile(artifactFile, outputDir)
		if err != nil {
			return fmt.Errorf("failed to process %s: %w", artifactFile, err)
		}
		allMetrics = append(allMetrics, metrics...)
		fileCount += files
	}

	// Sort metrics by Name, Function, ID
	sort.Slice(allMetrics, func(i, j int) bool {
		if allMetrics[i].Name != allMetrics[j].Name {
			return allMetrics[i].Name < allMetrics[j].Name
		}
		if allMetrics[i].Function != allMetrics[j].Function {
			return allMetrics[i].Function < allMetrics[j].Function
		}
		return allMetrics[i].ID < allMetrics[j].ID
	})

	// Write metrics file
	metricsFile := filepath.Join(outputDir, "metrics.txt")
	if err := writeMetrics(metricsFile, allMetrics); err != nil {
		return fmt.Errorf("failed to write metrics: %w", err)
	}

	fmt.Printf("Extracted artifacts to %s\n", outputDir)
	fmt.Printf("  - metrics.txt (%d metrics)\n", len(allMetrics))
	fmt.Printf("  - %d files extracted\n", fileCount)

	return nil
}

func processArtifactFile(artifactFile, outputDir string) ([]MetricRow, int, error) {
	// Read and parse the artifact file
	data, err := os.ReadFile(artifactFile)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read file: %w", err)
	}

	var testRun artifacts.TestRun
	if err := json.Unmarshal(data, &testRun); err != nil {
		return nil, 0, fmt.Errorf("failed to parse JSON: %w", err)
	}

	var metrics []MetricRow
	fileCount := 0

	// Process each function's artifacts
	for funcName, funcArtifacts := range testRun.Functions {
		// Process metrics
		for _, metric := range funcArtifacts.Metrics {
			metrics = append(metrics, MetricRow{
				Name:     testRun.Name,
				Function: funcName,
				ID:       metric.ID.String(),
				Value:    metric.Value,
			})
		}

		// Process files
		for _, file := range funcArtifacts.Files {
			if len(file.Content) == 0 {
				continue
			}

			// Content is already decoded []byte (JSON unmarshaling handles base64 decoding)
			// Create directory structure: <output>/<Name>/<Function>/<ID>
			fileDir := filepath.Join(outputDir, testRun.Name, funcName)
			if err := os.MkdirAll(fileDir, 0755); err != nil {
				return nil, 0, fmt.Errorf("failed to create directory %s: %w", fileDir, err)
			}

			// Write decoded file
			filePath := filepath.Join(fileDir, file.ID.String())
			if err := os.WriteFile(filePath, file.Content, 0644); err != nil {
				return nil, 0, fmt.Errorf("failed to write file %s: %w", filePath, err)
			}

			fileCount++
		}
	}

	return metrics, fileCount, nil
}

func writeMetrics(filename string, metrics []MetricRow) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	var lines []string
	for _, m := range metrics {
		lines = append(lines, fmt.Sprintf("%s\t%s\t%s\t%s", m.Name, m.Function, m.ID, m.Value))
	}

	_, err = f.WriteString(strings.Join(lines, "\n") + "\n")
	return err
}
