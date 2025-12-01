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

// TestAllDiskFormats validates retrieval of all 5 supported disk formats
// (raw, qcow2, vmdk, vhd, ova) in the correct order.
func TestAllDiskFormats(t *testing.T) {
	formats := AllDiskFormats()

	// Verify we got exactly 5 formats
	if len(formats) != 5 {
		t.Errorf("AllDiskFormats() returned %d formats, want 5", len(formats))
	}

	// Verify all expected formats are present
	expected := []DiskFormat{
		DiskFormatRaw,
		DiskFormatQcow2,
		DiskFormatVmdk,
		DiskFormatVhd,
		DiskFormatOva,
	}

	for i, want := range expected {
		if i >= len(formats) {
			t.Errorf("AllDiskFormats() missing format at index %d: %v", i, want)
			continue
		}
		if formats[i] != want {
			t.Errorf("AllDiskFormats()[%d] = %v, want %v", i, formats[i], want)
		}
	}
}

// TestDiskFormat_WithDiskPrefix tests adding "disk." prefix to format names
// (e.g., raw → disk.raw, qcow2 → disk.qcow2).
func TestDiskFormat_WithDiskPrefix(t *testing.T) {
	tests := []struct {
		format DiskFormat
		want   string
	}{
		{DiskFormatRaw, "disk.raw"},
		{DiskFormatQcow2, "disk.qcow2"},
		{DiskFormatVmdk, "disk.vmdk"},
		{DiskFormatVhd, "disk.vhd"},
		{DiskFormatOva, "disk.ova"},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			got := tt.format.WithDiskPrefix()
			if got != tt.want {
				t.Errorf("DiskFormat(%q).WithDiskPrefix() = %q, want %q", tt.format, got, tt.want)
			}
		})
	}
}

// TestCollectDiskFormats tests collection of disk formats from artifacts structure.
// Validates detection of formats present and handles empty artifacts and multiple files per format.
func TestCollectDiskFormats(t *testing.T) {
	tests := []struct {
		name      string
		artifacts *Artifacts
		want      []DiskFormat
	}{
		{
			name:      "empty artifacts",
			artifacts: &Artifacts{},
			want:      []DiskFormat{},
		},
		{
			name: "single format - raw",
			artifacts: &Artifacts{
				DiskRaw: []string{"disk.raw"},
			},
			want: []DiskFormat{DiskFormatRaw},
		},
		{
			name: "single format - qcow2",
			artifacts: &Artifacts{
				DiskQcow2: []string{"disk.qcow2"},
			},
			want: []DiskFormat{DiskFormatQcow2},
		},
		{
			name: "multiple formats",
			artifacts: &Artifacts{
				DiskRaw:   []string{"disk.raw"},
				DiskQcow2: []string{"disk.qcow2"},
				DiskVmdk:  []string{"disk.vmdk"},
			},
			want: []DiskFormat{DiskFormatRaw, DiskFormatQcow2, DiskFormatVmdk},
		},
		{
			name: "all formats",
			artifacts: &Artifacts{
				DiskRaw:   []string{"disk.raw"},
				DiskQcow2: []string{"disk.qcow2"},
				DiskVmdk:  []string{"disk.vmdk"},
				DiskVhd:   []string{"disk.vhd"},
				DiskOva:   []string{"disk.ova"},
			},
			want: []DiskFormat{DiskFormatRaw, DiskFormatQcow2, DiskFormatVmdk, DiskFormatVhd, DiskFormatOva},
		},
		{
			name: "multiple files per format",
			artifacts: &Artifacts{
				DiskRaw:   []string{"disk1.raw", "disk2.raw"},
				DiskQcow2: []string{"disk1.qcow2", "disk2.qcow2"},
			},
			want: []DiskFormat{DiskFormatRaw, DiskFormatQcow2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CollectDiskFormats(tt.artifacts)

			if len(got) != len(tt.want) {
				t.Errorf("CollectDiskFormats() returned %d formats, want %d", len(got), len(tt.want))
				return
			}

			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("CollectDiskFormats()[%d] = %v, want %v", i, got[i], want)
				}
			}
		})
	}
}
