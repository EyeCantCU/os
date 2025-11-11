//go:build vmtest

package selinux

import (
	"context"
	"testing"
	"time"

	"chainguard.dev/wolfi-vm/vms-test/pkg/vmtest"
)

func TestSELinuxStatus(t *testing.T) {
	ctx := vmtest.Context(t)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	status, err := GetSELinuxStatus(ctx)
	if err != nil {
		t.Fatalf("failed to get SELinux status: %v", err)
	}

	if status.Status != StatusEnabled {
		t.Errorf("SELinux status is %q, wanted %q",
			status.Status, StatusEnabled)
	}

	if status.CurrentMode != ModePermissive && status.CurrentMode != ModeEnforcing {
		t.Errorf("SELinux mode is %q, wanted %q or %q",
			status.CurrentMode, ModePermissive, ModeEnforcing)
	}

	if status.LoadedPolicyName != "targeted" {
		t.Errorf("SELinux policy is %q, wanted %q", status.LoadedPolicyName, "targeted")
	}
}
