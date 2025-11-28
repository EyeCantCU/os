package awspub

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// awspubConfig represents the structure of the awspub YAML files
type awspubConfig struct {
	Awspub struct {
		Images map[string]imageConfig `yaml:"images"`
	} `yaml:"awspub"`
}

// imageConfig represents an image configuration
type imageConfig struct {
	Marketplace *marketplaceConfig `yaml:"marketplace,omitempty"`
}

// marketplaceConfig represents marketplace configuration
type marketplaceConfig struct {
	EntityID string `yaml:"entity_id,omitempty"`
}

// Product represents a product with its metadata
type Product struct {
	Name         string
	Architecture string
	EntityID     string
	FilePath     string
}

// ProductMap represents products grouped by name and architecture
type ProductMap map[string]map[string]*Product

// ProductsFromDir scans the awspub directory and returns products grouped by name
func ProductsFromDir(dir string) (ProductMap, error) {
	productMap := make(ProductMap)

	architectures := []string{"x86_64", "aarch64"}
	for _, arch := range architectures {
		archDir := filepath.Join(dir, arch)
		files, err := filepath.Glob(filepath.Join(archDir, "*.yaml"))
		if err != nil {
			return nil, fmt.Errorf("error globbing %s: %w", archDir, err)
		}

		for _, file := range files {
			product, err := ParseFile(file, arch)
			if err != nil {
				return nil, fmt.Errorf("error parsing %s: %w", file, err)
			}

			if productMap[product.Name] == nil {
				productMap[product.Name] = make(map[string]*Product)
			}
			productMap[product.Name][product.Architecture] = product
		}
	}

	return productMap, nil
}

// ParseFile parses a single awspub YAML file - exported for testing
func ParseFile(filePath, arch string) (*Product, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	entityID, err := ExtractEntityID(data)
	if err != nil {
		return nil, err
	}

	basename := filepath.Base(filePath)
	productName := strings.TrimSuffix(basename, ".yaml")

	return &Product{
		Name:         productName,
		Architecture: arch,
		FilePath:     filePath,
		EntityID:     entityID,
	}, nil
}

// ExtractEntityID parses awspub YAML data and extracts the marketplace entity ID
func ExtractEntityID(data []byte) (string, error) {
	var cfg awspubConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("error unmarshaling YAML: %w", err)
	}

	if len(cfg.Awspub.Images) > 1 {
		return "", fmt.Errorf("multiple images not supported (found %d images)", len(cfg.Awspub.Images))
	}

	// Find the entity ID from the first image with marketplace config
	for _, img := range cfg.Awspub.Images {
		if img.Marketplace != nil && img.Marketplace.EntityID != "" {
			// Check if entity_id is not a placeholder
			if !strings.Contains(img.Marketplace.EntityID, "XXXX") {
				return img.Marketplace.EntityID, nil
			}
		}
	}

	return "", nil // No entity ID found, which is valid
}
