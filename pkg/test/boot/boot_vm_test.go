//go:build vmtest

package boot

import (
	"fmt"
	"testing"
	"time"

	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/metrics"
	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/vmtest"
)

func TestSSHStartTime(t *testing.T) {
	ctx := vmtest.Context(t)
	st, err := findServiceStartMonotonic(ctx, "sshd.service")
	if err != nil {
		t.Errorf("failed to get monotonic: %v\n", err)
	}

	limit := 180 * time.Second
	if st > limit {
		t.Errorf("timeToSSHD = %fs, want < %fs", st.Seconds(), limit.Seconds())
	}
	metrics.Log(t, metrics.SSHDStartTime, fmt.Sprintf("%f", st.Seconds()), nil)
}
