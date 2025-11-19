package publisher

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PublishConfig represents the structure of a publish.yaml configuration file.
type PublishConfig struct {
	Version   int       `yaml:"version"`
	Cloud     string    `yaml:"cloud"`
	Name      string    `yaml:"name"`
	OCIConfig OCIConfig `yaml:"oci_config"`
}

// OCIConfig represents the OCI-specific configuration for publishing.
type OCIConfig struct {
	Image       string   `yaml:"image"`
	Tags        []string `yaml:"tags"`
	DiskFormats []string `yaml:"disk_formats,omitempty"`
}

// LoadPublishConfig loads and parses a publish.yaml configuration file.
func LoadPublishConfig(path string) (*PublishConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config PublishConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// Validate checks that the configuration is valid.
func (c *PublishConfig) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version: %d (expected 1)", c.Version)
	}

	if c.Cloud == "" {
		return fmt.Errorf("cloud field is required")
	}

	// Validate that cloud is a recognized platform
	if _, err := ParsePlatform(c.Cloud); err != nil {
		return fmt.Errorf("invalid cloud value: %w", err)
	}

	if c.Name == "" {
		return fmt.Errorf("name field is required")
	}

	if c.OCIConfig.Image == "" {
		return fmt.Errorf("oci_config.image field is required")
	}

	if len(c.OCIConfig.Tags) == 0 {
		return fmt.Errorf("oci_config.tags must contain at least one tag")
	}

	// Validate disk formats (required)
	if len(c.OCIConfig.DiskFormats) == 0 {
		return fmt.Errorf("oci_config.disk_formats is required and must contain at least one format")
	}

	for _, format := range c.OCIConfig.DiskFormats {
		if _, err := ParseDiskFormat(format); err != nil {
			return fmt.Errorf("invalid disk format %q: %w", format, err)
		}
	}

	return nil
}

// GetPlatform returns the Platform enum for this config's cloud value.
func (c *PublishConfig) GetPlatform() (Platform, error) {
	return ParsePlatform(c.Cloud)
}

// GetDiskFormats returns the disk formats specified in the config.
// The config validation ensures at least one format is always present.
func (c *PublishConfig) GetDiskFormats() ([]DiskFormat, error) {
	formats := make([]DiskFormat, 0, len(c.OCIConfig.DiskFormats))
	for _, formatStr := range c.OCIConfig.DiskFormats {
		format, err := ParseDiskFormat(formatStr)
		if err != nil {
			return nil, err
		}
		formats = append(formats, format)
	}

	return formats, nil
}
