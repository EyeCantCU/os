//go:build vmtest

package boot

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts"
	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts/files"
	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts/metrics"
	"chainguard.dev/wolfi-vm/vm-test/pkg/vmtest"
)

func TestSSHStartTime(t *testing.T) {
	ctx := vmtest.Context(t)
	st, err := getServiceStartMonotonic(ctx, "sshd.service")
	if err != nil {
		t.Errorf("failed to get monotonic: %v\n", err)
	}

	limit := 180 * time.Second
	if st > limit {
		t.Errorf("timeToSSHD = %fs, want < %fs", st.Seconds(), limit.Seconds())
	}
	artifacts.Log(t, metrics.SSHDStartTime, fmt.Sprintf("%f", st.Seconds()), nil)
}

func TestCollectLogs(t *testing.T) {
	ctx := vmtest.Context(t)

	cmds := []struct {
		cmd []string
		id  files.ID
	}{
		{cmd: []string{"dmesg"}, id: files.Dmesg},
		{cmd: []string{"journalctl", "-b0"}, id: files.JournalCtlB0},
		{cmd: []string{"systemd-analyze"}, id: files.SystemdAnalyze},
		{cmd: []string{"systemd-analyze", "critical-chain"}, id: files.SystemdCriticalChain},
	}

	for _, finfo := range cmds {
		cmd := exec.CommandContext(ctx, finfo.cmd[0], finfo.cmd[1:]...)
		output, err := cmd.Output()
		if err != nil {
			t.Errorf("Execution of '%s' failed: %v", cmd.String(), err)
		}
		artifacts.File(t, finfo.id, output, nil)
	}
}
