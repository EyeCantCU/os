//go:build vmtest

package boot

import (
	"fmt"
	"testing"

	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/metrics"
	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/vmtest"
)

func TestSSHStartTime(t *testing.T) {
	ctx := vmtest.Context(t)
	vmStartTime, err := findVMStartTime(ctx)
	if err != nil {
		t.Fatalf("findVMStartTime(ctx) = err %v want nil", err)
	}
	sshdStartTime, err := findServiceStartTime(ctx, "sshd.service")
	timeToSSHD := int(sshdStartTime.Sub(vmStartTime).Seconds())
	if timeToSSHD > 20 {
		t.Errorf("timeToSSHD = %d seconds, want < 20", timeToSSHD)
	}
	metrics.Log(t, metrics.SSHDStartTime, fmt.Sprintf("%d", timeToSSHD), nil)
}
