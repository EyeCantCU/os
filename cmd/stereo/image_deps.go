package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"chainguard.dev/apko/pkg/apk/apk"
	apko_build "chainguard.dev/apko/pkg/build"
	apko_types "chainguard.dev/apko/pkg/build/types"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func imageDependenciesCmd() *cobra.Command {
	var (
		private bool
		arch    string
	)

	cmd := &cobra.Command{
		Use:   "image-dependencies",
		Short: "Pre-compute image dependencies from terraform JSON plan",
		Long: `This command reads terraform JSON from stdin, parses all apko_build configurations,
and resolves them to lists of APK packages used by each image. The results are written
to JSON files in the resolved/images/ directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return imageDependencies(cmd.Context(), private, arch)
		},
	}

	cmd.Flags().BoolVar(&private, "private", false, "Set for images-private (includes enterprise-packages)")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture to evaluate (default: x86_64)")

	return cmd
}

func imageDependencies(ctx context.Context, private bool, arch string) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	log.Printf("Pre-computing image dependencies for architecture %s...", arch)

	// Create resolved/images and unresolved/images directories if they don't exist
	resolvedDir := filepath.Join("resolved", "images")
	if err := os.MkdirAll(resolvedDir, 0755); err != nil {
		return fmt.Errorf("creating resolved/images directory: %w", err)
	}

	unresolvedDir := filepath.Join("unresolved", "images")
	if err := os.MkdirAll(unresolvedDir, 0755); err != nil {
		return fmt.Errorf("creating unresolved/images directory: %w", err)
	}

	cache := apk.NewCache(true)

	buildRepos := map[string][]string{
		"public":  []string{dirToRepo["os"], dirToRepo["extra-packages"]},
		"private": []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]},
	}

	var imageSet string
	if private {
		imageSet = "private"
	} else {
		imageSet = "public"
	}

	// Parse terraform JSON from stdin to get image configurations
	log.Printf("Parsing terraform plan from stdin...")
	configs, err := walk(ctx, os.Stdin)
	if err != nil {
		return fmt.Errorf("parsing terraform plan: %w", err)
	}

	log.Printf("Found %d apko_build configurations", len(configs))

	outputFile := filepath.Join(resolvedDir, fmt.Sprintf("%s.json", imageSet))
	unresolvedFile := filepath.Join(unresolvedDir, fmt.Sprintf("%s.json", imageSet))

	// Collect all unique APK packages across all images
	allPackages := make(map[string]bool) // package -> true if used
	unresolvedImages := make([]struct {
		Address string `json:"address"`
		Error   string `json:"error"`
	}, 0)
	var mu sync.Mutex

	log.Printf("Resolving image dependencies for %s images...", imageSet)

	// Process images in parallel
	var g errgroup.Group
	g.SetLimit(runtime.GOMAXPROCS(0))

	for addr, cfg := range configs {
		g.Go(func() error {
			// Capture variables for closure
			currentAddr := addr
			currentCfg := cfg

			log.Printf("Resolving dependencies for image: %s", currentAddr)

			// Resolve image to APK packages
			packages, err := lockImageDependencies(ctx, currentCfg, cache, buildRepos[imageSet], arch)
			if err != nil {
				log.Printf("Error resolving image dependencies for %s: %v", currentAddr, err)

				// Add to unresolved images list
				mu.Lock()
				unresolvedImages = append(unresolvedImages, struct {
					Address string `json:"address"`
					Error   string `json:"error"`
				}{
					Address: currentAddr,
					Error:   err.Error(),
				})
				mu.Unlock()

				return nil // Don't fail the entire operation for one image
			}

			// Add all packages to the set (with mutex protection)
			mu.Lock()
			for _, pkg := range packages {
				allPackages[pkg] = true
			}
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error processing image dependencies: %w", err)
	}

	// Convert set to sorted slice
	packageList := make([]string, 0, len(allPackages))
	for pkg := range allPackages {
		packageList = append(packageList, pkg)
	}

	// Create JSON structure
	data := struct {
		RepositorySet     string   `json:"repository_set"`
		Architecture      string   `json:"architecture"`
		ImageDependencies []string `json:"image_dependencies"`
	}{
		RepositorySet:     imageSet,
		Architecture:      arch,
		ImageDependencies: packageList,
	}

	// Write to JSON file
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("creating output file %s: %w", outputFile, err)
	}
	defer file.Close()

	log.Printf("Writing %d image dependencies to %s", len(packageList), outputFile)

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("encoding JSON to %s: %w", outputFile, err)
	}

	log.Printf("Successfully wrote image dependencies for %s to %s", imageSet, outputFile)

	// Write unresolved images to JSON file if there are any
	if len(unresolvedImages) > 0 {
		unresolvedData := struct {
			RepositorySet    string `json:"repository_set"`
			Architecture     string `json:"architecture"`
			UnresolvedImages []struct {
				Address string `json:"address"`
				Error   string `json:"error"`
			} `json:"unresolved_images"`
		}{
			RepositorySet:    imageSet,
			Architecture:     arch,
			UnresolvedImages: unresolvedImages,
		}

		unresolvedFileHandle, err := os.Create(unresolvedFile)
		if err != nil {
			log.Printf("Warning: Could not create unresolved images file %s: %v", unresolvedFile, err)
		} else {
			log.Printf("Writing %d unresolved images to %s", len(unresolvedImages), unresolvedFile)

			encoder := json.NewEncoder(unresolvedFileHandle)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(unresolvedData); err != nil {
				log.Printf("Warning: Error encoding unresolved images JSON to %s: %v", unresolvedFile, err)
			} else {
				log.Printf("Successfully wrote unresolved images for %s to %s", imageSet, unresolvedFile)
			}
			unresolvedFileHandle.Close()
		}
	}

	log.Printf("Image dependency pre-computation complete. Results written to %s/", resolvedDir)
	return nil
}

func lockImageDependencies(ctx context.Context, cfg *apko_types.ImageConfiguration, cache *apk.Cache, apkRepos []string, arch string) ([]string, error) {
	// Work around LockImageConfiguration assuming multi-arch.
	cfg.Archs = []apko_types.Architecture{apko_types.Architecture(arch)}

	opts := []apko_build.Option{
		apko_build.WithImageConfiguration(*cfg),
		apko_build.WithExtraBuildRepos(apkRepos),
		apko_build.WithArch(apko_types.Architecture(arch)),
		// TODO: Allow offline.
		apko_build.WithCache("", false, cache),
		// TODO: Fix that.
		apko_build.WithIgnoreSignatures(true),
	}

	configs, _, err := apko_build.LockImageConfiguration(ctx, *cfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to lock image configuration: %w", err)
	}

	locked, ok := configs["index"]
	if !ok {
		return nil, fmt.Errorf("missing locked config")
	}

	return locked.Contents.Packages, nil
}
