//go:build vmtest

package selinux

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
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

func TestNoAVCDenials(t *testing.T) {
	ctx := vmtest.Context(t)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "journalctl", "-b0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Error executing journalctl -b0: %v", err)
	}

	avcDenialPattern := regexp.MustCompile(`avc:\s+denied\s+.*`)
	var avcDenials []string

	for _, line := range strings.Split(string(output), "\n") {
		if denial := avcDenialPattern.FindString(line); denial != "" {
			avcDenials = append(avcDenials, denial)
		}
	}

	if numDenials := len(avcDenials); numDenials > 0 {
		t.Errorf("Found %v AVC denials, expected 0", numDenials)
		for _, denial := range avcDenials {
			t.Error(denial)
		}
	}
}
