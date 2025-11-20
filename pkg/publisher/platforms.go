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

// Platform represents a target cloud or virtualization platform
type Platform string

const (
	PlatformAWS    Platform = "aws"
	PlatformAzure  Platform = "azure"
	PlatformGCP    Platform = "gcp"
	PlatformVMware Platform = "vmware"
	PlatformQEMU   Platform = "qemu"
	PlatformRPI    Platform = "rpi"
	PlatformLXD    Platform = "lxd"
	PlatformHyperV Platform = "hyperv"
)

// ParsePlatform converts a string to a Platform type
// Returns an error if the platform string is not recognized
func ParsePlatform(s string) (Platform, error) {
	switch s {
	case "aws":
		return PlatformAWS, nil
	case "azure":
		return PlatformAzure, nil
	case "gcp":
		return PlatformGCP, nil
	case "vmware":
		return PlatformVMware, nil
	case "qemu":
		return PlatformQEMU, nil
	case "rpi":
		return PlatformRPI, nil
	case "lxd":
		return PlatformLXD, nil
	case "hyperv":
		return PlatformHyperV, nil
	default:
		return "", fmt.Errorf("unrecognized platform: %q", s)
	}
}

// ToCloudPlatforms returns the cloud platform identifiers for this platform
// Used in attestation metadata to indicate which cloud environments the image supports
// Returns an error if the platform enum value is not recognized
func (p Platform) ToCloudPlatforms() ([]string, error) {
	switch p {
	case PlatformAWS:
		return []string{"aws"}, nil
	case PlatformAzure:
		return []string{"azure"}, nil
	case PlatformGCP:
		return []string{"gcp"}, nil
	case PlatformVMware:
		return []string{"vmware"}, nil
	case PlatformRPI:
		return []string{"rpi"}, nil
	case PlatformQEMU:
		return []string{"qemu"}, nil
	case PlatformLXD:
		return []string{"lxd"}, nil
	case PlatformHyperV:
		return []string{"hyperv"}, nil
	default:
		return nil, fmt.Errorf("unrecognized platform enum value: %v", p)
	}
}
