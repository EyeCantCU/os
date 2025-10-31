package main

import (
	"regexp"
	"testing"

	"chainguard.dev/apko/pkg/apk/apk"
)

func TestFindNewestVersion(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		want     string
	}{
		{
			name:     "empty slice",
			versions: []string{},
			want:     "",
		},
		{
			name:     "single version",
			versions: []string{"3"},
			want:     "3",
		},
		{
			name:     "simple numeric versions",
			versions: []string{"3", "29", "75"},
			want:     "75",
		},
		{
			name:     "dotted versions",
			versions: []string{"3.11", "3.9", "3.10"},
			want:     "3.11",
		},
		{
			name:     "mixed dotted versions",
			versions: []string{"1.2.3", "1.2.10", "1.2.4"},
			want:     "1.2.10",
		},
		{
			name:     "reverse order",
			versions: []string{"75", "29", "3"},
			want:     "75",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findNewestVersion(tt.versions)
			if got != tt.want {
				t.Errorf("findNewestVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckPackageDependenciesWithVersions(t *testing.T) {
	// Helper to compile patterns
	compilePatterns := func(patterns []string) []*regexp.Regexp {
		compiled := make([]*regexp.Regexp, len(patterns))
		for i, p := range patterns {
			compiled[i] = regexp.MustCompile(p)
		}
		return compiled
	}

	tests := []struct {
		name             string
		pkg              *apk.Package
		patterns         []string
		targetVersions   map[string][]string
		versionProviders map[string]map[string]string
		targetPackage    string
		wantNeedsRebuild bool
		wantIsPinned     bool
	}{
		{
			name: "package needs rebuild - old version",
			pkg: &apk.Package{
				Name:    "test-pkg",
				Version: "1.0",
				Dependencies: []string{
					"so:libssl.so.1",
				},
			},
			patterns: []string{`so:libssl\.so(\.\d+)*`},
			targetVersions: map[string][]string{
				"libssl.so": {"1", "3"},
			},
			versionProviders: map[string]map[string]string{},
			targetPackage:    "openssl",
			wantNeedsRebuild: true,
			wantIsPinned:     false,
		},
		{
			name: "package is pinned to version stream",
			pkg: &apk.Package{
				Name:    "test-pkg",
				Version: "1.0",
				Dependencies: []string{
					"so:libprotobuf.so.29",
				},
			},
			patterns: []string{`so:libprotobuf\.so(\.\d+)*`},
			targetVersions: map[string][]string{
				"libprotobuf.so": {"29", "30"},
			},
			versionProviders: map[string]map[string]string{
				"libprotobuf.so": {
					"29": "protobuf-29.5",
				},
			},
			targetPackage:    "protobuf",
			wantNeedsRebuild: false,
			wantIsPinned:     true,
		},
		{
			name: "package already up to date",
			pkg: &apk.Package{
				Name:    "test-pkg",
				Version: "1.0",
				Dependencies: []string{
					"so:libssl.so.3",
				},
			},
			patterns: []string{`so:libssl\.so(\.\d+)*`},
			targetVersions: map[string][]string{
				"libssl.so": {"1", "3"},
			},
			versionProviders: map[string]map[string]string{},
			targetPackage:    "openssl",
			wantNeedsRebuild: false,
			wantIsPinned:     false,
		},
		{
			name: "package depends on same package version stream - needs rebuild",
			pkg: &apk.Package{
				Name:    "test-pkg",
				Version: "1.0",
				Dependencies: []string{
					"so:libprotobuf.so.29",
				},
			},
			patterns: []string{`so:libprotobuf\.so(\.\d+)*`},
			targetVersions: map[string][]string{
				"libprotobuf.so": {"29", "30"},
			},
			versionProviders: map[string]map[string]string{
				"libprotobuf.so": {
					"29": "protobuf", // Same as target package
				},
			},
			targetPackage:    "protobuf",
			wantNeedsRebuild: true,
			wantIsPinned:     false,
		},
		{
			name: "package with no matching dependencies",
			pkg: &apk.Package{
				Name:    "test-pkg",
				Version: "1.0",
				Dependencies: []string{
					"so:libfoo.so.1",
				},
			},
			patterns: []string{`so:libssl\.so(\.\d+)*`},
			targetVersions: map[string][]string{
				"libssl.so": {"1", "3"},
			},
			versionProviders: map[string]map[string]string{},
			targetPackage:    "openssl",
			wantNeedsRebuild: false,
			wantIsPinned:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiledPatterns := compilePatterns(tt.patterns)

			result := checkPackageDependenciesWithVersions(
				tt.pkg,
				compiledPatterns,
				tt.patterns,
				tt.targetVersions,
				tt.versionProviders,
				tt.targetPackage,
			)

			if result.NeedsRebuild != tt.wantNeedsRebuild {
				t.Errorf("NeedsRebuild = %v, want %v", result.NeedsRebuild, tt.wantNeedsRebuild)
			}
			if result.IsPinned != tt.wantIsPinned {
				t.Errorf("IsPinned = %v, want %v", result.IsPinned, tt.wantIsPinned)
			}
		})
	}
}

func TestExtractSoVersion(t *testing.T) {
	tests := []struct {
		name    string
		libName string
		want    string
	}{
		{
			name:    "simple version",
			libName: "libssl.so.3",
			want:    "3",
		},
		{
			name:    "dotted version",
			libName: "libicu.so.75.1",
			want:    "75.1",
		},
		{
			name:    "no version",
			libName: "libfoo.so",
			want:    "",
		},
		{
			name:    "complex version",
			libName: "libtest.so.1.2.3",
			want:    "1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSoVersion(tt.libName)
			if got != tt.want {
				t.Errorf("extractSoVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveVersionSuffix(t *testing.T) {
	tests := []struct {
		name    string
		libName string
		want    string
	}{
		{
			name:    "simple version",
			libName: "libssl.so.3",
			want:    "libssl.so",
		},
		{
			name:    "dotted version",
			libName: "libicu.so.75.1",
			want:    "libicu.so",
		},
		{
			name:    "no version",
			libName: "libfoo.so",
			want:    "libfoo.so",
		},
		{
			name:    "complex version",
			libName: "libtest.so.1.2.3",
			want:    "libtest.so",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeVersionSuffix(tt.libName)
			if got != tt.want {
				t.Errorf("removeVersionSuffix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsMoreRecent(t *testing.T) {
	tests := []struct {
		name string
		pkg1 *apk.Package
		pkg2 *apk.Package
		want bool
	}{
		{
			name: "pkg1 newer",
			pkg1: &apk.Package{Version: "2.0"},
			pkg2: &apk.Package{Version: "1.0"},
			want: true,
		},
		{
			name: "pkg2 newer",
			pkg1: &apk.Package{Version: "1.0"},
			pkg2: &apk.Package{Version: "2.0"},
			want: false,
		},
		{
			name: "equal versions",
			pkg1: &apk.Package{Version: "1.0"},
			pkg2: &apk.Package{Version: "1.0"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMoreRecent(tt.pkg1, tt.pkg2)
			if got != tt.want {
				t.Errorf("isMoreRecent() = %v, want %v", got, tt.want)
			}
		})
	}
}
