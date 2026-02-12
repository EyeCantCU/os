package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"chainguard.dev/apko/pkg/apk/apk"
	"cloud.google.com/go/storage"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
)

// isPublicImage determines if an image repository URL represents a public image.
// Public images are hosted at cgr.dev/chainguard/<image-name>.
// Everything else (cgr.dev/chainguard-private/*, cgr.dev/custom-images/*, etc.) is private.
func isPublicImage(repoURL string) bool {
	return strings.HasPrefix(repoURL, "cgr.dev/chainguard/")
}

func imageDependenciesCmd() *cobra.Command {
	var (
		private      bool
		arch         string
		useWithdrawn bool
		withdrawnDir string
		useGCS       string
		threshold    int
	)

	cmd := &cobra.Command{
		Use:   "image-dependencies",
		Short: "Pre-compute image dependencies from terraform JSON plan",
		Long: `This command reads terraform JSON from stdin (or a GCS bucket), parses all apko_build configurations,
and resolves them to lists of APK packages used by each image. The results are written
to JSON files in the resolved/images/ directory.

When no --arch is specified, dependencies are computed for both x86_64 and aarch64 architectures,
respecting any archs constraints in apko configurations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var architectures []string
			if arch != "" {
				architectures = []string{arch}
			} else {
				architectures = []string{"x86_64", "aarch64"}
			}
			return imageDependencies(cmd.Context(), private, architectures, useWithdrawn, withdrawnDir, useGCS, threshold)
		},
	}

	cmd.Flags().BoolVar(&private, "private", false, "Set for images-private (includes enterprise-packages)")
	cmd.Flags().StringVar(&arch, "arch", "", "Architecture to evaluate (default: both x86_64 and aarch64)")
	cmd.Flags().BoolVar(&useWithdrawn, "use-withdrawn", false, "Use withdrawn APKINDEX files instead of live repositories")
	cmd.Flags().StringVar(&withdrawnDir, "withdrawn-dir", "withdrawn-indexes", "Directory containing withdrawn APKINDEX files")
	cmd.Flags().StringVar(&useGCS, "use-gcs", "", "GCS bucket path to read terraform plans from (e.g., gs://bucket-name/path)")
	cmd.Flags().IntVar(&threshold, "threshold", 800, "Number of most recent terraform plan files to consider from GCS")

	return cmd
}

func imageDependencies(ctx context.Context, private bool, architectures []string, useWithdrawn bool, withdrawnDir string, useGCS string, threshold int) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	if useWithdrawn {
		log.Printf("Pre-computing image dependencies for architectures %v using withdrawn indexes from %s...", architectures, withdrawnDir)
	} else {
		log.Printf("Pre-computing image dependencies for architectures %v...", architectures)
	}

	var imageSet string
	if private {
		imageSet = "private"
	} else {
		imageSet = "public"
	}

	// Get input source (either stdin or GCS)
	var input io.Reader
	var err error
	if useGCS != "" {
		log.Printf("Fetching terraform plans from GCS: %s (threshold: %d files)...", useGCS, threshold)
		input, err = createGCSReader(ctx, useGCS, threshold)
		if err != nil {
			return fmt.Errorf("creating GCS reader: %w", err)
		}
	} else {
		log.Printf("Parsing terraform plan from stdin...")
		input = os.Stdin
	}

	// Parse terraform JSON to get image configurations
	configs, err := walk(ctx, input)
	if err != nil {
		return fmt.Errorf("parsing terraform plan: %w", err)
	}

	log.Printf("Found %d apko_build configurations", len(configs))

	// Process each architecture
	for _, arch := range architectures {
		log.Printf("Processing architecture: %s", arch)

		// Create output directories - use withdrawn-test prefix when testing with withdrawn indexes
		var resolvedDir, unresolvedDir string
		if useWithdrawn {
			resolvedDir = filepath.Join("withdrawn-test", "resolved", "images", arch)
			unresolvedDir = filepath.Join("withdrawn-test", "unresolved", "images", arch)
		} else {
			resolvedDir = filepath.Join("resolved", "images", arch)
			unresolvedDir = filepath.Join("unresolved", "images", arch)
		}

		if err := os.MkdirAll(resolvedDir, 0755); err != nil {
			return fmt.Errorf("creating resolved images directory: %w", err)
		}

		if err := os.MkdirAll(unresolvedDir, 0755); err != nil {
			return fmt.Errorf("creating unresolved images directory: %w", err)
		}

		cache := apk.NewCache(true)

		var buildRepos map[string][]string
		if useWithdrawn {
			// Use local withdrawn indexes instead of remote repositories
			withdrawnRepos := dirToWithdrawnRepo(withdrawnDir)
			buildRepos = map[string][]string{
				"public":  []string{withdrawnRepos["os"], withdrawnRepos["extra-packages"]},
				"private": []string{withdrawnRepos["os"], withdrawnRepos["extra-packages"], withdrawnRepos["enterprise-packages"]},
			}
		} else {
			// Use normal remote repositories
			buildRepos = map[string][]string{
				"public":  []string{dirToRepo["os"], dirToRepo["extra-packages"]},
				"private": []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]},
			}
		}

		outputFile := filepath.Join(resolvedDir, fmt.Sprintf("%s.json", imageSet))
		unresolvedFile := filepath.Join(unresolvedDir, fmt.Sprintf("%s.json", imageSet))

		// Collect all unique APK packages across all images
		allPackages := make(map[string]bool) // package -> true if used
		unresolvedImages := make([]struct {
			Address string `json:"address"`
			Error   string `json:"error"`
		}, 0)

		// Create subdirectory for detailed image info
		detailDir := filepath.Join(resolvedDir, imageSet)
		if err := os.MkdirAll(detailDir, 0755); err != nil {
			return fmt.Errorf("creating detail directory %s: %w", detailDir, err)
		}

		var mu sync.Mutex

		log.Printf("Resolving image dependencies for %s images (architecture: %s)...", imageSet, arch)

		// Process images in parallel
		var g errgroup.Group
		g.SetLimit(runtime.GOMAXPROCS(0))

		for addr, info := range configs {
			g.Go(func() error {
				// Capture variables for closure
				currentAddr := addr
				currentInfo := info
				currentArch := arch

				// Determine if this image is public or private based on repo URL
				imageIsPublic := currentInfo.repo != "" && isPublicImage(currentInfo.repo)

				// Skip images that don't match the requested imageSet
				if imageSet == "public" && !imageIsPublic {
					log.Printf("Skipping %s: private image (repo: %s)", currentAddr, currentInfo.repo)
					return nil
				}
				if imageSet == "private" && imageIsPublic {
					log.Printf("Skipping %s: public image (repo: %s)", currentAddr, currentInfo.repo)
					return nil
				}

				// Check if this image supports the current architecture
				if !supportsApkoArchitecture(currentInfo.config, currentArch) {
					log.Printf("Skipping %s: does not support architecture %s", currentAddr, currentArch)
					return nil
				}

				log.Printf("Resolving dependencies for image: %s (repo: %s, architecture: %s)", currentAddr, currentInfo.repo, currentArch)

				// Create a copy of the configuration to avoid mutation by lockImageDependencies
				cfgCopy := *currentInfo.config

				// Resolve image to APK packages
				packages, err := lockImageDependencies(ctx, &cfgCopy, cache, buildRepos[imageSet], currentArch)
				if err != nil {
					log.Printf("Error resolving image dependencies for %s (architecture: %s): %v", currentAddr, currentArch, err)

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

				// Save individual image dependencies to detailed JSON file
				sortedPkgs := make([]string, len(packages))
				copy(sortedPkgs, packages)
				sort.Strings(sortedPkgs)

				// Clean the address to create a safe filename
				safeAddr := strings.ReplaceAll(strings.ReplaceAll(currentAddr, "/", "_"), ":", "_")
				imageDetailFile := filepath.Join(detailDir, fmt.Sprintf("%s.json", safeAddr))
				imageData := struct {
					Address       string   `json:"address"`
					Repo          string   `json:"repo"`
					RepositorySet string   `json:"repository_set"`
					Architecture  string   `json:"architecture"`
					Dependencies  []string `json:"dependencies"`
				}{
					Address:       currentAddr,
					Repo:          currentInfo.repo,
					RepositorySet: imageSet,
					Architecture:  currentArch,
					Dependencies:  sortedPkgs,
				}

				if err := func() error {
					file, err := os.Create(imageDetailFile)
					if err != nil {
						return err
					}
					defer file.Close()

					encoder := json.NewEncoder(file)
					encoder.SetIndent("", "  ")
					return encoder.Encode(imageData)
				}(); err != nil {
					log.Printf("Warning: failed to write detailed dependencies for %s: %v", currentAddr, err)
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
			return fmt.Errorf("error processing image dependencies (architecture: %s): %w", arch, err)
		}

		// Convert set to sorted slice
		packageList := make([]string, 0, len(allPackages))
		for pkg := range allPackages {
			packageList = append(packageList, pkg)
		}
		sort.Strings(packageList)

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

		log.Printf("Successfully wrote image dependencies for %s (architecture: %s) to %s", imageSet, arch, outputFile)

		// Write unresolved images to JSON file if there are any
		if len(unresolvedImages) > 0 {
			// Sort unresolved images by address for consistent output
			sort.Slice(unresolvedImages, func(i, j int) bool {
				return unresolvedImages[i].Address < unresolvedImages[j].Address
			})

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
					log.Printf("Successfully wrote unresolved images for %s (architecture: %s) to %s", imageSet, arch, unresolvedFile)
				}
				unresolvedFileHandle.Close()
			}
		}
	}

	log.Printf("Image dependency pre-computation complete for architectures %v", architectures)
	return nil
}

// createGCSReader creates an io.Reader that streams terraform plan files from GCS.
// It lists objects matching .tfplan.json, sorts by creation time (newest first),
// takes the most recent 'threshold' files, reverses them (oldest first), and
// streams their contents concatenated together.
func createGCSReader(ctx context.Context, gcsPath string, threshold int) (io.Reader, error) {
	// Parse GCS path (gs://bucket/prefix)
	if !strings.HasPrefix(gcsPath, "gs://") {
		return nil, fmt.Errorf("GCS path must start with gs://, got: %s", gcsPath)
	}

	pathWithoutScheme := strings.TrimPrefix(gcsPath, "gs://")
	parts := strings.SplitN(pathWithoutScheme, "/", 2)
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid GCS path: %s", gcsPath)
	}

	bucketName := parts[0]
	var prefix string
	if len(parts) > 1 {
		prefix = parts[1]
		// Ensure prefix ends with / for directory-style matching (but only if non-empty)
		if prefix != "" && !strings.HasSuffix(prefix, "/") {
			prefix = prefix + "/"
		}
	}

	// Create GCS client
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating GCS client: %w", err)
	}

	bucket := client.Bucket(bucketName)

	// List all objects with .tfplan.json suffix
	log.Printf("Listing objects from gs://%s/%s...", bucketName, prefix)

	var objects []*storage.ObjectAttrs
	query := &storage.Query{
		Prefix: prefix,
	}

	count := 0
	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("iterating objects: %w", err)
		}
		if strings.HasSuffix(attrs.Name, ".tfplan.json") {
			objects = append(objects, attrs)
			count++
		}
		// Only care about up to threshold image plans
		if count >= threshold {
			break
		}
	}

	log.Printf("Found %d .tfplan.json files in GCS", len(objects))

	if len(objects) == 0 {
		client.Close()
		return nil, fmt.Errorf("no .tfplan.json files found in %s", gcsPath)
	}

	// Sort by creation time, newest first
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].Created.After(objects[j].Created)
	})

	// Reverse to get oldest first (like tac)
	for i, j := 0, len(objects)-1; i < j; i, j = i+1, j-1 {
		objects[i], objects[j] = objects[j], objects[i]
	}

	log.Printf("Processing %d terraform plan files from GCS (oldest to newest)", len(objects))

	// Create a multi-reader that streams all files
	return &gcsMultiReader{
		ctx:     ctx,
		bucket:  bucket,
		objects: objects,
		client:  client,
	}, nil
}

// gcsMultiReader implements io.Reader to stream multiple GCS objects sequentially
type gcsMultiReader struct {
	ctx     context.Context
	bucket  *storage.BucketHandle
	objects []*storage.ObjectAttrs
	client  *storage.Client

	currentIndex  int
	currentReader io.ReadCloser
}

func (r *gcsMultiReader) Read(p []byte) (n int, err error) {
	for {
		// If we have a current reader, try to read from it
		if r.currentReader != nil {
			n, err = r.currentReader.Read(p)
			if err == nil {
				return n, nil
			}

			// If we hit EOF, close current reader and move to next file
			if err == io.EOF {
				r.currentReader.Close()
				r.currentReader = nil
				r.currentIndex++
				// Continue to next file
			} else {
				// Other error, return it
				return n, err
			}
		}

		// Check if we've processed all objects
		if r.currentIndex >= len(r.objects) {
			if r.client != nil {
				r.client.Close()
				r.client = nil
			}
			return 0, io.EOF
		}

		// Open next object
		obj := r.objects[r.currentIndex]
		log.Printf("Reading terraform plan %d/%d: gs://%s/%s (created: %s)",
			r.currentIndex+1, len(r.objects), r.bucket.BucketName(), obj.Name, obj.Created.Format("2006-01-02 15:04:05"))

		reader, err := r.bucket.Object(obj.Name).NewReader(r.ctx)
		if err != nil {
			return 0, fmt.Errorf("opening GCS object %s: %w", obj.Name, err)
		}

		r.currentReader = reader
	}
}
