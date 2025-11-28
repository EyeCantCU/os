package awspub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractEntityID(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantEntity  string
		wantError   bool
		errorMsg    string
	}{
		{
			name: "valid product with entity ID",
			yamlContent: `awspub:
  images:
    ami-12345:
      marketplace:
        entity_id: prod-xp3s4znrj2exs`,
			wantEntity: "prod-xp3s4znrj2exs",
			wantError:  false,
		},
		{
			name: "product with placeholder entity ID",
			yamlContent: `awspub:
  images:
    ami-12345:
      marketplace:
        entity_id: prod-XXXXXXXXXXXX`,
			wantEntity: "",
			wantError:  false,
		},
		{
			name: "product without marketplace config",
			yamlContent: `awspub:
  images:
    ami-12345:
      some_other_field: value`,
			wantEntity: "",
			wantError:  false,
		},
		{
			name: "multiple images error",
			yamlContent: `awspub:
  images:
    ami-12345:
      marketplace:
        entity_id: prod-abc
    ami-67890:
      marketplace:
        entity_id: prod-def`,
			wantEntity: "",
			wantError:  true,
			errorMsg:   "multiple images not supported",
		},
		{
			name: "invalid YAML",
			yamlContent: `awspub:
  images
    ami-12345 bad yaml`,
			wantEntity: "",
			wantError:  true,
			errorMsg:   "error unmarshaling YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entityID, err := ExtractEntityID([]byte(tt.yamlContent))

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if entityID != tt.wantEntity {
				t.Errorf("entity ID = %q, want %q", entityID, tt.wantEntity)
			}
		})
	}
}

func TestProductsFromDir(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create x86_64 and aarch64 directories
	x86Dir := filepath.Join(tmpDir, "x86_64")
	armDir := filepath.Join(tmpDir, "aarch64")

	if err := os.MkdirAll(x86Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(armDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test YAML files
	testFiles := []struct {
		path    string
		content string
	}{
		{
			filepath.Join(x86Dir, "aws-base.yaml"),
			`awspub:
  images:
    ami-x86-base:
      marketplace:
        entity_id: prod-base-x86`,
		},
		{
			filepath.Join(armDir, "aws-base.yaml"),
			`awspub:
  images:
    ami-arm-base:
      marketplace:
        entity_id: prod-base-arm`,
		},
		{
			filepath.Join(x86Dir, "aws-docker.yaml"),
			`awspub:
  images:
    ami-x86-docker:
      marketplace:
        entity_id: prod-docker-x86`,
		},
		{
			filepath.Join(x86Dir, "aws-unpublished.yaml"),
			`awspub:
  images:
    ami-unpub:
      some_field: value`,
		},
	}

	for _, tf := range testFiles {
		if err := os.WriteFile(tf.path, []byte(tf.content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Test ProductsFromDir
	products, err := ProductsFromDir(tmpDir)
	if err != nil {
		t.Fatalf("ProductsFromDir failed: %v", err)
	}

	// Verify the product map structure
	if len(products) != 3 {
		t.Errorf("expected 3 products, got %d", len(products))
	}

	// Check aws-base has both architectures
	if base, ok := products["aws-base"]; ok {
		if _, hasX86 := base["x86_64"]; !hasX86 {
			t.Error("aws-base missing x86_64 architecture")
		}
		if _, hasArm := base["aarch64"]; !hasArm {
			t.Error("aws-base missing aarch64 architecture")
		}
		if base["x86_64"].EntityID != "prod-base-x86" {
			t.Errorf("aws-base x86_64 entity = %q, want %q", base["x86_64"].EntityID, "prod-base-x86")
		}
	} else {
		t.Error("aws-base not found in products")
	}

	// Check aws-docker has only x86_64
	if docker, ok := products["aws-docker"]; ok {
		if _, hasX86 := docker["x86_64"]; !hasX86 {
			t.Error("aws-docker missing x86_64 architecture")
		}
		if _, hasArm := docker["aarch64"]; hasArm {
			t.Error("aws-docker should not have aarch64 architecture")
		}
	} else {
		t.Error("aws-docker not found in products")
	}

	// Check aws-unpublished exists but has no entity ID
	if unpub, ok := products["aws-unpublished"]; ok {
		if unpub["x86_64"].EntityID != "" {
			t.Errorf("aws-unpublished should have empty entity ID, got %q", unpub["x86_64"].EntityID)
		}
	} else {
		t.Error("aws-unpublished not found in products")
	}
}

func TestProductsFromDir_Errors(t *testing.T) {
	// Test with non-existent directory - glob doesn't error on non-existent paths,
	// it just returns empty, so we get an empty product map
	products, err := ProductsFromDir("/non/existent/path")
	if err != nil {
		t.Errorf("unexpected error for non-existent directory: %v", err)
	}
	if len(products) != 0 {
		t.Errorf("expected empty product map for non-existent directory, got %d products", len(products))
	}

	// Test with invalid YAML file
	tmpDir := t.TempDir()
	x86Dir := filepath.Join(tmpDir, "x86_64")
	os.MkdirAll(x86Dir, 0755)

	badYAML := filepath.Join(x86Dir, "bad.yaml")
	os.WriteFile(badYAML, []byte("invalid: yaml: content:"), 0644)

	_, err = ProductsFromDir(tmpDir)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}
