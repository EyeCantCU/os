package publisher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPublishConfig(t *testing.T) {
	tests := []struct {
		name        string
		configYAML  string
		wantErr     bool
		errContains string
		validate    func(*testing.T, *PublishConfig)
	}{
		{
			name: "valid config",
			configYAML: `version: 1
cloud: azure
name: azure-python-313-slim
oci_config:
  image: python
  tags:
    - azure-python-3.13-slim
    - azure-python-slim
  disk_formats:
    - vhd
`,
			wantErr: false,
			validate: func(t *testing.T, c *PublishConfig) {
				if c.Version != 1 {
					t.Errorf("Version = %d, want 1", c.Version)
				}
				if c.Cloud != "azure" {
					t.Errorf("Cloud = %q, want %q", c.Cloud, "azure")
				}
				if c.Name != "azure-python-313-slim" {
					t.Errorf("Name = %q, want %q", c.Name, "azure-python-313-slim")
				}
				if c.OCIConfig.Image != "python" {
					t.Errorf("OCIConfig.Image = %q, want %q", c.OCIConfig.Image, "python")
				}
				if len(c.OCIConfig.Tags) != 2 {
					t.Errorf("len(OCIConfig.Tags) = %d, want 2", len(c.OCIConfig.Tags))
				}
				if len(c.OCIConfig.DiskFormats) != 1 {
					t.Errorf("len(OCIConfig.DiskFormats) = %d, want 1", len(c.OCIConfig.DiskFormats))
				}
			},
		},
		{
			name: "config with disk formats",
			configYAML: `version: 1
cloud: qemu
name: qemu-base-slim
oci_config:
  image: base
  tags:
    - qemu-slim
  disk_formats:
    - qcow2
    - vmdk
`,
			wantErr: false,
			validate: func(t *testing.T, c *PublishConfig) {
				if len(c.OCIConfig.DiskFormats) != 2 {
					t.Errorf("len(OCIConfig.DiskFormats) = %d, want 2", len(c.OCIConfig.DiskFormats))
				}
				formats, err := c.GetDiskFormats()
				if err != nil {
					t.Fatalf("GetDiskFormats() error = %v", err)
				}
				if len(formats) != 2 {
					t.Errorf("len(formats) = %d, want 2", len(formats))
				}
				if formats[0] != DiskFormatQcow2 {
					t.Errorf("formats[0] = %v, want DiskFormatQcow2", formats[0])
				}
				if formats[1] != DiskFormatVmdk {
					t.Errorf("formats[1] = %v, want DiskFormatVmdk", formats[1])
				}
			},
		},
		{
			name: "lxd config",
			configYAML: `version: 1
cloud: lxd
name: lxd-python-313-full
oci_config:
  image: python
  tags:
    - lxd-python-3.13-full
  disk_formats:
    - raw.tgz
`,
			wantErr: false,
			validate: func(t *testing.T, c *PublishConfig) {
				if c.Cloud != "lxd" {
					t.Errorf("Cloud = %q, want %q", c.Cloud, "lxd")
				}
				if c.Name != "lxd-python-313-full" {
					t.Errorf("Name = %q, want %q", c.Name, "lxd-python-313-full")
				}
				platform, err := c.GetPlatform()
				if err != nil {
					t.Fatalf("GetPlatform() error = %v", err)
				}
				if platform != PlatformLXD {
					t.Errorf("GetPlatform() = %v, want PlatformLXD", platform)
				}
			},
		},
		{
			name: "hyperv config",
			configYAML: `version: 1
cloud: hyperv
name: hyperv-docker-full
oci_config:
  image: docker
  tags:
    - hyperv-docker-full
  disk_formats:
    - vhd
`,
			wantErr: false,
			validate: func(t *testing.T, c *PublishConfig) {
				if c.Cloud != "hyperv" {
					t.Errorf("Cloud = %q, want %q", c.Cloud, "hyperv")
				}
				if c.Name != "hyperv-docker-full" {
					t.Errorf("Name = %q, want %q", c.Name, "hyperv-docker-full")
				}
				platform, err := c.GetPlatform()
				if err != nil {
					t.Fatalf("GetPlatform() error = %v", err)
				}
				if platform != PlatformHyperV {
					t.Errorf("GetPlatform() = %v, want PlatformHyperV", platform)
				}
			},
		},
		{
			name: "missing version",
			configYAML: `cloud: azure
name: test
oci_config:
  image: test
  tags:
    - test-tag
`,
			wantErr:     true,
			errContains: "unsupported config version",
		},
		{
			name: "missing cloud",
			configYAML: `version: 1
name: test
oci_config:
  image: test
  tags:
    - test-tag
`,
			wantErr:     true,
			errContains: "cloud field is required",
		},
		{
			name: "invalid cloud",
			configYAML: `version: 1
cloud: invalid
name: test
oci_config:
  image: test
  tags:
    - test-tag
`,
			wantErr:     true,
			errContains: "invalid cloud value",
		},
		{
			name: "missing image",
			configYAML: `version: 1
cloud: azure
name: test
oci_config:
  tags:
    - test-tag
`,
			wantErr:     true,
			errContains: "oci_config.image field is required",
		},
		{
			name: "missing tags",
			configYAML: `version: 1
cloud: azure
name: test
oci_config:
  image: test
`,
			wantErr:     true,
			errContains: "oci_config.tags must contain at least one tag",
		},
		{
			name: "invalid disk format",
			configYAML: `version: 1
cloud: azure
name: test
oci_config:
  image: test
  tags:
    - test-tag
  disk_formats:
    - invalid
`,
			wantErr:     true,
			errContains: "invalid disk format",
		},
		{
			name: "missing disk_formats",
			configYAML: `version: 1
cloud: azure
name: test
oci_config:
  image: test
  tags:
    - test-tag
`,
			wantErr:     true,
			errContains: "oci_config.disk_formats is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "publish.yaml")
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			// Load config
			config, err := LoadPublishConfig(configPath)

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadPublishConfig() error = nil, want error containing %q", tt.errContains)
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("LoadPublishConfig() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("LoadPublishConfig() unexpected error = %v", err)
			}

			// Run validation function
			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestPublishConfig_GetPlatform(t *testing.T) {
	config := &PublishConfig{
		Version: 1,
		Cloud:   "aws",
		Name:    "test",
		OCIConfig: OCIConfig{
			Image: "test",
			Tags:  []string{"test"},
		},
	}

	platform, err := config.GetPlatform()
	if err != nil {
		t.Fatalf("GetPlatform() error = %v", err)
	}

	if platform != PlatformAWS {
		t.Errorf("GetPlatform() = %v, want PlatformAWS", platform)
	}
}

func TestPublishConfig_GetDiskFormats(t *testing.T) {
	config := &PublishConfig{
		Version: 1,
		Cloud:   "aws",
		Name:    "test",
		OCIConfig: OCIConfig{
			Image:       "test",
			Tags:        []string{"test"},
			DiskFormats: []string{"vmdk", "raw.tgz"},
		},
	}

	formats, err := config.GetDiskFormats()
	if err != nil {
		t.Fatalf("GetDiskFormats() error = %v", err)
	}

	if len(formats) != 2 {
		t.Errorf("GetDiskFormats() returned %d formats, want 2", len(formats))
	}

	if formats[0] != DiskFormatVmdk {
		t.Errorf("formats[0] = %v, want DiskFormatVmdk", formats[0])
	}

	if formats[1] != DiskFormatRaw {
		t.Errorf("formats[1] = %v, want DiskFormatRaw", formats[1])
	}
}
