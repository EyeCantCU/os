package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func assertPackages(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("got %d packages, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
		return
	}
	for i, pkg := range got {
		if pkg != want[i] {
			t.Errorf("package[%d] = %v, want %v", i, pkg, want[i])
		}
	}
}

func TestParseCueLockFile_GroupsStructure(t *testing.T) {
	// Create a temporary CUE file with groups structure
	content := `package main

pkgLocks: {
	groups: "test-group": components: "test-component": {
		pkgs: [
			"pkg1=1.0-r0",
			"pkg2=2.0-r1",
		]
		dev: [
			"dev-pkg1=1.0-r0",
			"dev-pkg2=2.0-r1",
		]
	}
}`

	tmpFile, err := os.CreateTemp("", "package_lock_*.cue")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Parse the CUE file for x86_64 architecture
	packagesSet := make(map[string]bool)
	err = parseCueLockFile(tmpFile.Name(), "x86_64", packagesSet)
	if err != nil {
		t.Fatalf("parseCueLockFile() error = %v", err)
	}

	// Convert set to sorted slice for comparison
	packages := make([]string, 0, len(packagesSet))
	for pkg := range packagesSet {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	// Verify packages
	want := []string{
		"dev-pkg1=1.0-r0",
		"dev-pkg2=2.0-r1",
		"pkg1=1.0-r0",
		"pkg2=2.0-r1",
	}

	assertPackages(t, packages, want)
}

func TestParseCueLockFile_ImagesStructure(t *testing.T) {
	// Create a temporary CUE file with images structure
	content := `package main

pkgLocks: {
	images: "test-image": {
		pkgs: [
			"img-pkg1=1.0-r0",
			"img-pkg2=2.0-r1",
		]
		dev: [
			"img-dev-pkg1=1.0-r0",
		]
	}
}`

	tmpFile, err := os.CreateTemp("", "package_lock_*.cue")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Parse the CUE file for x86_64 architecture
	packagesSet := make(map[string]bool)
	err = parseCueLockFile(tmpFile.Name(), "x86_64", packagesSet)
	if err != nil {
		t.Fatalf("parseCueLockFile() error = %v", err)
	}

	// Convert set to sorted slice for comparison
	packages := make([]string, 0, len(packagesSet))
	for pkg := range packagesSet {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	// Verify packages
	want := []string{
		"img-dev-pkg1=1.0-r0",
		"img-pkg1=1.0-r0",
		"img-pkg2=2.0-r1",
	}

	assertPackages(t, packages, want)
}

func TestParseCueLockFile_MultipleGroupsAndComponents(t *testing.T) {
	// Create a temporary CUE file with multiple groups and components
	content := `package main

pkgLocks: {
	groups: "group1": components: {
		"component1": {
			pkgs: [
				"pkg1=1.0-r0",
			]
		}
		"component2": {
			pkgs: [
				"pkg2=2.0-r1",
			]
		}
	}
	groups: "group2": components: "component3": {
		pkgs: [
			"pkg3=3.0-r2",
		]
	}
}`

	tmpFile, err := os.CreateTemp("", "package_lock_*.cue")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Parse the CUE file for x86_64 architecture
	packagesSet := make(map[string]bool)
	err = parseCueLockFile(tmpFile.Name(), "x86_64", packagesSet)
	if err != nil {
		t.Fatalf("parseCueLockFile() error = %v", err)
	}

	// Convert set to sorted slice for comparison
	packages := make([]string, 0, len(packagesSet))
	for pkg := range packagesSet {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	// Verify packages
	want := []string{
		"pkg1=1.0-r0",
		"pkg2=2.0-r1",
		"pkg3=3.0-r2",
	}

	assertPackages(t, packages, want)
}

func TestParseCueLockFile_MixedGroupsAndImages(t *testing.T) {
	// Create a temporary CUE file with both groups and images structures
	content := `package main

pkgLocks: {
	groups: "group1": components: "component1": {
		pkgs: [
			"group-pkg1=1.0-r0",
		]
	}
	images: "image1": {
		pkgs: [
			"image-pkg1=2.0-r1",
		]
	}
}`

	tmpFile, err := os.CreateTemp("", "package_lock_*.cue")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Parse the CUE file for x86_64 architecture
	packagesSet := make(map[string]bool)
	err = parseCueLockFile(tmpFile.Name(), "x86_64", packagesSet)
	if err != nil {
		t.Fatalf("parseCueLockFile() error = %v", err)
	}

	// Convert set to sorted slice for comparison
	packages := make([]string, 0, len(packagesSet))
	for pkg := range packagesSet {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	// Verify packages from both structures are included
	want := []string{
		"group-pkg1=1.0-r0",
		"image-pkg1=2.0-r1",
	}

	assertPackages(t, packages, want)
}

func TestParseCueLockFile_InvalidStructure(t *testing.T) {
	// Create a temporary CUE file with no pkgLocks structure
	content := `package main

// No pkgLocks structure
someOtherField: {
	value: "test"
}`

	tmpFile, err := os.CreateTemp("", "package_lock_*.cue")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Parse the CUE file - should succeed but find no packages
	packagesSet := make(map[string]bool)
	err = parseCueLockFile(tmpFile.Name(), "x86_64", packagesSet)
	if err != nil {
		t.Errorf("parseCueLockFile() unexpected error for unrecognized structure: %v", err)
	}
	if len(packagesSet) != 0 {
		t.Errorf("parseCueLockFile() expected no packages, got %d", len(packagesSet))
	}
}

func TestParseCueLockFile_FileNotFound(t *testing.T) {
	// Try to parse a non-existent file
	packagesSet := make(map[string]bool)
	err := parseCueLockFile("/nonexistent/path/package_lock.cue", "x86_64", packagesSet)
	if err == nil {
		t.Errorf("parseCueLockFile() expected error for non-existent file, got nil")
	}
}

func TestCollectCuePackages_Private(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "images-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create images/test-image directory
	imageDir := filepath.Join(tmpDir, "images", "test-image")
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		t.Fatalf("Failed to create image dir: %v", err)
	}

	// Create package_lock.cue file
	content := `package main

pkgLocks: {
	images: "test-image": {
		pkgs: [
			"test-pkg=1.0-r0",
		]
	}
}`

	cuePath := filepath.Join(imageDir, "package_lock.cue")
	if err := os.WriteFile(cuePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write CUE file: %v", err)
	}

	// Collect packages for x86_64
	packages, err := collectCuePackages(tmpDir, true, "x86_64")
	if err != nil {
		t.Fatalf("collectCuePackages() error = %v", err)
	}

	// Verify result - should contain the test package
	if len(packages) != 1 {
		t.Errorf("collectCuePackages() returned %d packages, want 1", len(packages))
	}

	if !packages["test-pkg=1.0-r0"] {
		t.Errorf("collectCuePackages() missing expected package 'test-pkg=1.0-r0', got: %v", packages)
	}
}

func TestCollectCuePackages_Public(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "images-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create public/images/test-image directory
	imageDir := filepath.Join(tmpDir, "public", "images", "test-image")
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		t.Fatalf("Failed to create image dir: %v", err)
	}

	// Create package_lock.cue file
	content := `package main

pkgLocks: {
	images: "test-image": {
		pkgs: [
			"public-pkg=1.0-r0",
		]
	}
}`

	cuePath := filepath.Join(imageDir, "package_lock.cue")
	if err := os.WriteFile(cuePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write CUE file: %v", err)
	}

	// Collect packages (public mode) for x86_64
	packages, err := collectCuePackages(tmpDir, false, "x86_64")
	if err != nil {
		t.Fatalf("collectCuePackages() error = %v", err)
	}

	// Verify result - should contain the test package
	if len(packages) != 1 {
		t.Errorf("collectCuePackages() returned %d packages, want 1", len(packages))
	}

	if !packages["public-pkg=1.0-r0"] {
		t.Errorf("collectCuePackages() missing expected package 'public-pkg=1.0-r0', got: %v", packages)
	}
}

func TestCollectCuePackages_DirectoryNotFound(t *testing.T) {
	// Try to collect from non-existent directory
	_, err := collectCuePackages("/nonexistent/path", true, "x86_64")
	if err == nil {
		t.Errorf("collectCuePackages() expected error for non-existent directory, got nil")
	}
}

func TestParseCueLockFile_ArchDivergent(t *testing.T) {
	// Create a temporary CUE file with arch-divergent structure (Pattern 3)
	content := `package main

pkgLocks: {
	images: "test-image": {
		pkgs: {
			amd64: [
				"amd64-only-pkg=1.0-r0",
			]
			arm64: [
				"arm64-only-pkg=2.0-r1",
			]
			index: [
				"common-pkg1=3.0-r2",
				"common-pkg2=4.0-r3",
			]
		}
		dev: {
			amd64: [
				"amd64-dev-pkg=5.0-r4",
			]
			index: [
				"common-dev-pkg=6.0-r5",
			]
		}
	}
}`

	tmpFile, err := os.CreateTemp("", "package_lock_*.cue")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Test x86_64 architecture - should get amd64 + index packages
	packagesX86Set := make(map[string]bool)
	err = parseCueLockFile(tmpFile.Name(), "x86_64", packagesX86Set)
	if err != nil {
		t.Fatalf("parseCueLockFile(x86_64) error = %v", err)
	}

	// Convert set to sorted slice for comparison
	packagesX86 := make([]string, 0, len(packagesX86Set))
	for pkg := range packagesX86Set {
		packagesX86 = append(packagesX86, pkg)
	}
	sort.Strings(packagesX86)

	wantX86 := []string{
		"amd64-dev-pkg=5.0-r4",
		"amd64-only-pkg=1.0-r0",
		"common-dev-pkg=6.0-r5",
		"common-pkg1=3.0-r2",
		"common-pkg2=4.0-r3",
	}

	assertPackages(t, packagesX86, wantX86)

	// Test aarch64 architecture - should get arm64 + index packages
	packagesAarchSet := make(map[string]bool)
	err = parseCueLockFile(tmpFile.Name(), "aarch64", packagesAarchSet)
	if err != nil {
		t.Fatalf("parseCueLockFile(aarch64) error = %v", err)
	}

	// Convert set to sorted slice for comparison
	packagesAarch := make([]string, 0, len(packagesAarchSet))
	for pkg := range packagesAarchSet {
		packagesAarch = append(packagesAarch, pkg)
	}
	sort.Strings(packagesAarch)

	wantAarch := []string{
		"arm64-only-pkg=2.0-r1",
		"common-dev-pkg=6.0-r5",
		"common-pkg1=3.0-r2",
		"common-pkg2=4.0-r3",
	}

	assertPackages(t, packagesAarch, wantAarch)
}
