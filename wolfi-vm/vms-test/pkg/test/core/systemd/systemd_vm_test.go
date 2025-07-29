//go:build vmtest

package systemd

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts"
	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts/files"
	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts/metrics"
	"chainguard.dev/wolfi-vm/vm-test/pkg/systemd"
	"chainguard.dev/wolfi-vm/vm-test/pkg/vmtest"
)

// Wait for systemd to start, fail if it's not in "running" state.
func TestSystemdStatus(t *testing.T) {
	ctx := vmtest.Context(t)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	conn, err := systemd.Connect(ctx)
	if err != nil {
		t.Fatalf("connectSystemd(ctx) = %v, want nil", err)
	}
	defer conn.Close()

	var state string
	var v systemd.Variant
	for ctx.Err() == nil {
		v, err = conn.GetManagerProperty("SystemState")
		if err != nil {
			t.Fatalf("getManagerProperty(SystemState) = err %v, want nil", err)
		}
		state, err = v.AsString()
		if err != nil {
			t.Fatalf("v.AsString() = err %v, want nil", err)
		}

		if state != "starting" && state != "initializing" {
			break
		}
		select {
		case ctxerr := <-ctx.Done():
			t.Fatalf("context expired (%v) before systemd finished starting\nlast state was: %v", ctxerr, state)
		case <-time.After(500 * time.Millisecond):
			continue
		}
	}

	if state != "running" {
		t.Errorf("systemd state is %q, want %q", state, "running")

		failedUnits, err := conn.ListUnitsFiltered([]string{"failed"})
		if err != nil {
			t.Errorf("listFailedUnits() = err %v, want nil", err)
		} else {
			t.Log("failed units:")
			for _, unit := range failedUnits {
				t.Logf("%+v", unit)
			}
		}
	}

	data := map[string]any{}
	for _, prop := range []string{
		"Version", "Features", "NFailedJobs", "NNames", "Tainted",
	} {
		val, err := conn.GetManagerProperty(prop)
		if err != nil {
			t.Errorf("getManagerProperty(%s) = err %v, want nil", prop, err)
			continue
		}
		if i, err := val.AsInt(); err == nil {
			data[prop] = i
		} else {
			data[prop] = val.String()
		}
	}
	artifacts.Log(t, metrics.SystemdState, state, data)
}

func TestCollectLogs(t *testing.T) {
	ctx := vmtest.Context(t)
	timeout := 180 * time.Second
	myCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// wait for graphical.target
	st, err := systemd.GetServiceStartMonotonic(myCtx, "graphical.target")
	if err != nil {
		t.Errorf("fail waiting for graphical.target: %v\n", err)
	}

	artifacts.Log(t, metrics.MultiUserTarget, fmt.Sprintf("%f", st.Seconds()), nil)

	cmds := []struct {
		cmd []string
		id  files.ID
	}{
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
