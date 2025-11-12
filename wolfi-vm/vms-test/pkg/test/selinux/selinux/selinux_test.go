//go:build unittest

package selinux

import (
	"testing"
)

func TestParseSELinuxStatusDisabled(t *testing.T) {
	input := `SELinux status:                 disabled`

	status, err := ParseSELinuxStatus(input)
	if err != nil {
		t.Fatalf("ParseSELinuxStatus failed: %v", err)
	}

	if status.Status != StatusDisabled {
		t.Errorf("Status = %q, want %q", status.Status, StatusDisabled)
	}

	// When disabled, other fields should be empty/zero
	if status.CurrentMode != Unknown {
		t.Errorf("CurrentMode = %q, want %q (disabled systems have no mode)", status.CurrentMode, Unknown)
	}
	if status.MaxKernelPolicyVersion != 0 {
		t.Errorf("MaxKernelPolicyVersion = %d, want 0 (disabled systems have no version)", status.MaxKernelPolicyVersion)
	}
}

func TestParseSELinuxStatusEnabled(t *testing.T) {
	input := `SELinux status:                 enabled
SELinuxfs mount:                /sys/fs/selinux
SELinux root directory:         /etc/selinux
Loaded policy name:             targeted
Current mode:                   enforcing
Mode from config file:          enforcing
Policy MLS status:              enabled
Policy deny_unknown status:     allowed
Memory protection checking:     actual (secure)
Max kernel policy version:      35`

	status, err := ParseSELinuxStatus(input)
	if err != nil {
		t.Fatalf("ParseSELinuxStatus failed: %v", err)
	}

	if status.Status != StatusEnabled {
		t.Errorf("Status = %q, want %q", status.Status, StatusEnabled)
	}
	if status.SELinuxFSMount != "/sys/fs/selinux" {
		t.Errorf("SELinuxFSMount = %q, want %q", status.SELinuxFSMount, "/sys/fs/selinux")
	}
	if status.RootDirectory != "/etc/selinux" {
		t.Errorf("RootDirectory = %q, want %q", status.RootDirectory, "/etc/selinux")
	}
	if status.LoadedPolicyName != "targeted" {
		t.Errorf("LoadedPolicyName = %q, want %q", status.LoadedPolicyName, "targeted")
	}
	if status.CurrentMode != ModeEnforcing {
		t.Errorf("CurrentMode = %q, want %q", status.CurrentMode, ModeEnforcing)
	}
	if status.ModeFromConfigFile != ModeEnforcing {
		t.Errorf("ModeFromConfigFile = %q, want %q", status.ModeFromConfigFile, ModeEnforcing)
	}
	if status.PolicyMLSStatus != StatusEnabled {
		t.Errorf("PolicyMLSStatus = %q, want %q", status.PolicyMLSStatus, StatusEnabled)
	}
	if status.PolicyDenyUnknownStatus != DenyUnknownAllowed {
		t.Errorf("PolicyDenyUnknownStatus = %q, want %q", status.PolicyDenyUnknownStatus, DenyUnknownAllowed)
	}
	if status.MemoryProtectionChecking != MemoryProtectionActual {
		t.Errorf("MemoryProtectionChecking = %q, want %q", status.MemoryProtectionChecking, MemoryProtectionActual)
	}
	if status.MaxKernelPolicyVersion != 35 {
		t.Errorf("MaxKernelPolicyVersion = %d, want %d", status.MaxKernelPolicyVersion, 35)
	}
}

func TestParseSELinuxStatusEmpty(t *testing.T) {
	input := ``

	_, err := ParseSELinuxStatus(input)
	if err == nil {
		t.Error("ParseSELinuxStatus should have failed with empty input")
	}
}

func TestParseSELinuxStatusInvalidVersion(t *testing.T) {
	input := `SELinux status:                 enabled
Max kernel policy version:      not-a-number`

	_, err := ParseSELinuxStatus(input)
	if err == nil {
		t.Error("ParseSELinuxStatus should have failed with invalid version")
	}
}
