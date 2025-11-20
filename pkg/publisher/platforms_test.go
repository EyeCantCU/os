// Copyright 2025 Chainguard, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package publisher

import (
	"testing"
)

// TestParsePlatform tests parsing of platform strings (aws, azure, gcp, etc.) to Platform types.
// Validates case sensitivity and returns errors for unknown platforms.
func TestParsePlatform(t *testing.T) {
	tests := []struct {
		input    string
		wantPlat Platform
		wantErr  bool
	}{
		// Recognized platforms
		{
			input:    "aws",
			wantPlat: PlatformAWS,
			wantErr:  false,
		},
		{
			input:    "azure",
			wantPlat: PlatformAzure,
			wantErr:  false,
		},
		{
			input:    "gcp",
			wantPlat: PlatformGCP,
			wantErr:  false,
		},
		{
			input:    "vmware",
			wantPlat: PlatformVMware,
			wantErr:  false,
		},
		{
			input:    "qemu",
			wantPlat: PlatformQEMU,
			wantErr:  false,
		},
		{
			input:    "rpi",
			wantPlat: PlatformRPI,
			wantErr:  false,
		},
		{
			input:    "lxd",
			wantPlat: PlatformLXD,
			wantErr:  false,
		},
		{
			input:    "hyperv",
			wantPlat: PlatformHyperV,
			wantErr:  false,
		},
		// Unrecognized platforms - should return error
		{
			input:    "unknown",
			wantPlat: "",
			wantErr:  true,
		},
		{
			input:    "",
			wantPlat: "",
			wantErr:  true,
		},
		{
			input:    "AWS", // case-sensitive
			wantPlat: "",
			wantErr:  true,
		},
		{
			input:    "aws-ec2",
			wantPlat: "",
			wantErr:  true,
		},
		{
			input:    "invalid-platform",
			wantPlat: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotPlat, err := ParsePlatform(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePlatform(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && gotPlat != tt.wantPlat {
				t.Errorf("ParsePlatform(%q) platform = %v, want %v", tt.input, gotPlat, tt.wantPlat)
			}
		})
	}
}

// TestPlatform_ToCloudPlatforms tests conversion of Platform types to cloud platform string slices.
// Validates platform-specific mappings (AWS→["aws"], QEMU→["qemu"], etc.) and errors for unknown platforms.
func TestPlatform_ToCloudPlatforms(t *testing.T) {
	tests := []struct {
		platform Platform
		want     []string
		wantErr  bool
	}{
		{
			platform: PlatformAWS,
			want:     []string{"aws"},
			wantErr:  false,
		},
		{
			platform: PlatformAzure,
			want:     []string{"azure"},
			wantErr:  false,
		},
		{
			platform: PlatformGCP,
			want:     []string{"gcp"},
			wantErr:  false,
		},
		{
			platform: PlatformVMware,
			want:     []string{"vmware"},
			wantErr:  false,
		},
		{
			platform: PlatformRPI,
			want:     []string{"rpi"},
			wantErr:  false,
		},
		{
			platform: PlatformQEMU,
			want:     []string{"qemu"},
			wantErr:  false,
		},
		{
			platform: PlatformLXD,
			want:     []string{"lxd"},
			wantErr:  false,
		},
		{
			platform: PlatformHyperV,
			want:     []string{"hyperv"},
			wantErr:  false,
		},
		// Unknown platform should return error
		{
			platform: Platform("unknown"),
			want:     nil,
			wantErr:  true,
		},
		{
			platform: Platform(""),
			want:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.platform), func(t *testing.T) {
			got, err := tt.platform.ToCloudPlatforms()

			if (err != nil) != tt.wantErr {
				t.Errorf("Platform(%q).ToCloudPlatforms() error = %v, wantErr %v", tt.platform, err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return // Don't check 'got' if we expected an error
			}

			if len(got) != len(tt.want) {
				t.Errorf("Platform(%q).ToCloudPlatforms() returned %d items, want %d", tt.platform, len(got), len(tt.want))
				return
			}

			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("Platform(%q).ToCloudPlatforms()[%d] = %q, want %q", tt.platform, i, got[i], want)
				}
			}
		})
	}
}
