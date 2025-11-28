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

import "fmt"

// DiskFormat represents a VM disk image format
type DiskFormat string

const (
	DiskFormatRaw   DiskFormat = "raw.tgz"
	DiskFormatQcow2 DiskFormat = "qcow2"
	DiskFormatVmdk  DiskFormat = "vmdk"
	DiskFormatVhd   DiskFormat = "vhd"
	DiskFormatOva   DiskFormat = "ova"
)

// AllDiskFormats returns a list of all supported disk formats
func AllDiskFormats() []DiskFormat {
	return []DiskFormat{
		DiskFormatRaw,
		DiskFormatQcow2,
		DiskFormatVmdk,
		DiskFormatVhd,
		DiskFormatOva,
	}
}

// ParseDiskFormat parses a string into a DiskFormat
// Returns an error if the format is not recognized
func ParseDiskFormat(s string) (DiskFormat, error) {
	format := DiskFormat(s)
	switch format {
	case DiskFormatRaw, DiskFormatQcow2, DiskFormatVmdk, DiskFormatVhd, DiskFormatOva:
		return format, nil
	default:
		return "", fmt.Errorf("unrecognized disk format: %q", s)
	}
}

// WithDiskPrefix returns the format with "disk." prefix (e.g., "disk.vmdk")
func (d DiskFormat) WithDiskPrefix() string {
	return "disk." + string(d)
}

// CollectDiskFormats returns a list of disk formats present in the artifacts
func CollectDiskFormats(artifacts *Artifacts) []DiskFormat {
	formats := []DiskFormat{}
	if len(artifacts.DiskRaw) > 0 {
		formats = append(formats, DiskFormatRaw)
	}
	if len(artifacts.DiskQcow2) > 0 {
		formats = append(formats, DiskFormatQcow2)
	}
	if len(artifacts.DiskVmdk) > 0 {
		formats = append(formats, DiskFormatVmdk)
	}
	if len(artifacts.DiskVhd) > 0 {
		formats = append(formats, DiskFormatVhd)
	}
	if len(artifacts.DiskOva) > 0 {
		formats = append(formats, DiskFormatOva)
	}
	return formats
}
