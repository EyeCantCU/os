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
	"chainguard.dev/melange/pkg/sign"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

func withdrawCmd() *cobra.Command {
	var (
		arch       string
		outDir     string
		signingKey string
	)

	cmd := &cobra.Command{
		Use:   "withdraw",
		Short: "Withdraw packages from APKINDEX files based on withdrawn-packages.txt",
		Long: `This command downloads the latest APKINDEX.tar.gz files from each repository,
removes packages listed in withdrawn-packages.txt files, and saves the modified
indexes to a local directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withdraw(cmd.Context(), arch, outDir, signingKey)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture to process (default: x86_64)")
	cmd.Flags().StringVar(&outDir, "output-dir", "withdrawn-indexes", "Output directory for modified APKINDEX files")
	cmd.Flags().StringVar(&signingKey, "signing-key", "melange.rsa", "The signing key to use for signing the modified APKINDEX")

	return cmd
}

func withdraw(ctx context.Context, arch, outDir, signingKey string) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	log.Printf("Withdrawing packages for architecture %s...", arch)

	// Create output directory
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("creating output directory %s: %w", outDir, err)
	}

	// Process each repository
	for repo, repoURL := range dirToRepo {
		log.Printf("Processing repository: %s", repo)

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

		log.Printf("Found %d packages to withdraw from %s", len(withdrawnPackages), repo)

		// Download APKINDEX
		index, err := fetchAPKIndex(ctx, repoURL, arch)
		if err != nil {
			return fmt.Errorf("downloading APKINDEX for %s: %w", repo, err)
		}

		log.Printf("Downloaded APKINDEX with %d packages from %s", len(index.Packages), repo)

		// Remove withdrawn packages
		originalCount := len(index.Packages)
		index.Packages = slices.DeleteFunc(index.Packages, func(pkg *apk.Package) bool {
			pkgFileName := pkg.Name + "-" + pkg.Version + ".apk"
			_, shouldWithdraw := withdrawnPackages[pkgFileName]
			if shouldWithdraw {
				log.Printf("Withdrawing %s", pkgFileName)
				delete(withdrawnPackages, pkgFileName) // Mark as processed
			}
			return shouldWithdraw
		})

		removedCount := originalCount - len(index.Packages)
		log.Printf("Removed %d packages from %s index, %d packages remaining", removedCount, repo, len(index.Packages))

		// Warn about packages that weren't found
		for pkgFileName := range withdrawnPackages {
			log.Printf("Warning: Package %s not found in %s index", pkgFileName, repo)
		}

		// Create repo subdirectory
		repoDir := filepath.Join(outDir, repo, arch)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			return fmt.Errorf("creating repo directory %s: %w", repoDir, err)
		}

		// Save modified index to disk
		outputFile := filepath.Join(repoDir, "APKINDEX.tar.gz")
		if err := saveAPKIndex(ctx, index, outputFile, signingKey); err != nil {
			return fmt.Errorf("saving modified APKINDEX for %s: %w", repo, err)
		}

		log.Printf("Saved modified APKINDEX to %s", outputFile)
	}

	log.Printf("Withdraw operation complete. Modified indexes saved to %s/", outDir)
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

func saveAPKIndex(ctx context.Context, index *apk.APKIndex, outputFile, signingKey string) error {
	log.Printf("Saving APKINDEX with %d packages to %s", len(index.Packages), outputFile)

	// Create archive from index
	archive, err := apk.ArchiveFromIndex(index)
	if err != nil {
		return fmt.Errorf("creating archive from index: %w", err)
	}

	// Create temporary file for signing (no extension like wolfictl)
	tmp, err := os.CreateTemp("", "stereo-withdraw")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmp.Name()) // Clean up temp file

	// Write archive to temp file
	bytesWritten, err := io.Copy(tmp, archive)
	if err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	log.Printf("Wrote %d bytes to temp file %s", bytesWritten, tmp.Name())

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Sign the index
	log.Printf("Signing index with key %s", signingKey)
	if err := sign.SignIndex(ctx, signingKey, tmp.Name()); err != nil {
		return fmt.Errorf("signing index: %w", err)
	}

	// Open the signed file
	signed, err := os.Open(tmp.Name())
	if err != nil {
		return fmt.Errorf("opening signed temp file %s: %w", tmp.Name(), err)
	}
	defer signed.Close()

	// Get signed file size for debugging
	signedInfo, err := signed.Stat()
	if err != nil {
		log.Printf("Warning: could not stat signed file: %v", err)
	} else {
		log.Printf("Signed file size: %d bytes", signedInfo.Size())
	}

	// Create final output file
	outputFileHandle, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("creating output file %s: %w", outputFile, err)
	}
	defer outputFileHandle.Close()

	// Copy signed archive to final output
	finalBytesWritten, err := io.Copy(outputFileHandle, signed)
	if err != nil {
		return fmt.Errorf("writing signed archive to %s: %w", outputFile, err)
	}
	log.Printf("Wrote %d bytes to final output file %s", finalBytesWritten, outputFile)

	return nil
}
