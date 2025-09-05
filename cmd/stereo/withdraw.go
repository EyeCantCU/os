package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"chainguard.dev/apko/pkg/apk/apk"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

func withdrawCmd() *cobra.Command {
	var (
		arch   string
		outDir string
	)

	cmd := &cobra.Command{
		Use:   "withdraw",
		Short: "Withdraw packages from APKINDEX files based on withdrawn-packages.txt",
		Long: `This command downloads the latest APKINDEX.tar.gz files from each repository,
removes packages listed in withdrawn-packages.txt files, and saves the modified
indexes to a local directory.

When no --arch is specified, packages are withdrawn from both x86_64 and aarch64 architectures.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var architectures []string
			if arch != "" {
				architectures = []string{arch}
			} else {
				architectures = []string{"x86_64", "aarch64"}
			}
			return withdraw(cmd.Context(), architectures, outDir)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "", "Architecture to process (default: both x86_64 and aarch64)")
	cmd.Flags().StringVar(&outDir, "output-dir", "withdrawn-indexes", "Output directory for modified APKINDEX files")

	return cmd
}

func withdraw(ctx context.Context, architectures []string, outDir string) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	log.Printf("Withdrawing packages for architectures %v...", architectures)

	// Create output directory
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("creating output directory %s: %w", outDir, err)
	}

	// Process each architecture
	for _, arch := range architectures {
		log.Printf("Processing architecture: %s", arch)

		// Process each repository for this architecture
		for repo, repoURL := range dirToRepo {
			log.Printf("Processing repository: %s (architecture: %s)", repo, arch)

			// Check if withdrawn-packages.txt exists for this repo
			withdrawnFile := filepath.Join(repo, "withdrawn-packages.txt")
			if _, err := os.Stat(withdrawnFile); os.IsNotExist(err) {
				log.Printf("No withdrawn-packages.txt found for %s, skipping", repo)
				continue
			}

			// Load packages to withdraw
			withdrawnPackages, err := loadWithdrawnPackages(withdrawnFile)
			if err != nil {
				return fmt.Errorf("loading withdrawn packages for %s: %w", repo, err)
			}

			if len(withdrawnPackages) == 0 {
				log.Printf("No packages to withdraw for %s", repo)
				continue
			}

			log.Printf("Found %d packages to withdraw from %s (architecture: %s)", len(withdrawnPackages), repo, arch)

			// Download APKINDEX for this architecture
			index, err := fetchAPKIndex(ctx, repoURL, arch)
			if err != nil {
				return fmt.Errorf("downloading APKINDEX for %s/%s: %w", repo, arch, err)
			}

			log.Printf("Downloaded APKINDEX with %d packages from %s (architecture: %s)", len(index.Packages), repo, arch)

			// Remove withdrawn packages
			originalCount := len(index.Packages)
			index.Packages = slices.DeleteFunc(index.Packages, func(pkg *apk.Package) bool {
				pkgFileName := pkg.Name + "-" + pkg.Version + ".apk"
				_, shouldWithdraw := withdrawnPackages[pkgFileName]
				if shouldWithdraw {
					log.Printf("Withdrawing %s from %s/%s", pkgFileName, repo, arch)
					delete(withdrawnPackages, pkgFileName) // Mark as processed
				}
				return shouldWithdraw
			})

			removedCount := originalCount - len(index.Packages)
			log.Printf("Removed %d packages from %s/%s index, %d packages remaining", removedCount, repo, arch, len(index.Packages))

			// Warn about packages that weren't found
			for pkgFileName := range withdrawnPackages {
				log.Printf("Warning: Package %s not found in %s/%s index", pkgFileName, repo, arch)
			}

			// Create repo subdirectory with architecture
			repoDir := filepath.Join(outDir, repo, arch)
			if err := os.MkdirAll(repoDir, 0755); err != nil {
				return fmt.Errorf("creating repo directory %s: %w", repoDir, err)
			}

			// Save modified index to disk
			outputFile := filepath.Join(repoDir, "APKINDEX.tar.gz")
			if err := saveAPKIndex(index, outputFile); err != nil {
				return fmt.Errorf("saving modified APKINDEX for %s/%s: %w", repo, arch, err)
			}

			log.Printf("Saved modified APKINDEX to %s", outputFile)
		}
	}

	log.Printf("Withdraw operation complete for architectures %v. Modified indexes saved to %s/", architectures, outDir)
	return nil
}

func loadWithdrawnPackages(filename string) (map[string]bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", filename, err)
	}
	defer file.Close()

	withdrawnPackages := make(map[string]bool)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue // Skip empty lines
		}
		withdrawnPackages[line] = true
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", filename, err)
	}

	return withdrawnPackages, nil
}

func saveAPKIndex(index *apk.APKIndex, outputFile string) error {
	log.Printf("Saving APKINDEX with %d packages to %s", len(index.Packages), outputFile)

	// Create archive from index
	archive, err := apk.ArchiveFromIndex(index)
	if err != nil {
		return fmt.Errorf("creating archive from index: %w", err)
	}

	// Create output file
	outputFileHandle, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("creating output file %s: %w", outputFile, err)
	}
	defer outputFileHandle.Close()

	// Copy archive directly to output
	bytesWritten, err := io.Copy(outputFileHandle, archive)
	if err != nil {
		return fmt.Errorf("writing archive to %s: %w", outputFile, err)
	}
	log.Printf("Wrote %d bytes to %s", bytesWritten, outputFile)

	return nil
}
