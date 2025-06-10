//go:build vmtest

package boot

import (
	"context"
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
	timeout := 180 * time.Second
	myCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// wait for multi-user.target
	st, err := getServiceStartMonotonic(myCtx, "multi-user.target")
	if err != nil {
		t.Errorf("fail waiting for multi-user.target: %v\n", err)
	}

	artifacts.Log(t, metrics.MultiUserTarget, fmt.Sprintf("%f", st.Seconds()), nil)

	cmds := []struct {
		cmd []string
		id  files.ID
	}{
		{cmd: []string{"dmesg"}, id: files.Dmesg},
		{cmd: []string{"journalctl", "-b0"}, id: files.JournalCtlB0},
		{cmd: []string{"systemd-analyze"}, id: files.SystemdAnalyze},
		{cmd: []string{"systemd-analyze", "critical-chain"}, id: files.SystemdCriticalChain},
	}

	errors := map[files.ID]error{}
	for _, finfo := range cmds {
		cmd := exec.CommandContext(ctx, finfo.cmd[0], finfo.cmd[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			errors[finfo.id] = fmt.Errorf("Execution of '%s' failed: %v", cmd.String(), err)
		}
		artifacts.File(t, finfo.id, output, err, nil)
	}

	if len(errors) != 0 {
		t.Errorf("File collection failed: %v", errors)
	}
}
