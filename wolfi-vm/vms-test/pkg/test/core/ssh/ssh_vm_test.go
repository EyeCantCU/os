//go:build vmtest

package ssh

import (
	"fmt"
	"testing"
	"time"

	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts"
	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts/metrics"
	"chainguard.dev/wolfi-vm/vm-test/pkg/systemd"
	"chainguard.dev/wolfi-vm/vm-test/pkg/vmtest"
)

func TestSSHStartTime(t *testing.T) {
	ctx := vmtest.Context(t)
	st, err := systemd.GetServiceStartMonotonic(ctx, "sshd.service")
	if err != nil {
		t.Errorf("failed to get monotonic: %v\n", err)
	}

	limit := 180 * time.Second
	if st > limit {
		t.Errorf("timeToSSHD = %fs, want < %fs", st.Seconds(), limit.Seconds())
	}
	artifacts.Log(t, metrics.SSHDStartTime, fmt.Sprintf("%f", st.Seconds()), nil)
}
