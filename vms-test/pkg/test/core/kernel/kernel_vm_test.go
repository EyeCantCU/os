//go:build vmtest

package kernel

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts"
	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts/files"
	"chainguard.dev/wolfi-vm/vm-test/pkg/artifacts/metrics"
	"chainguard.dev/wolfi-vm/vm-test/pkg/systemd"
	"chainguard.dev/wolfi-vm/vm-test/pkg/vmtest"
)

var (
	kernelOopsRe = regexp.MustCompile("] (BUG|Oops|Internal error)")
	kernelWarnRe = regexp.MustCompile("] WARNING:")
)

func TestErrorsInDmesg(t *testing.T) {
	ctx := vmtest.Context(t)
	dmesgout, err := dmesg(ctx)
	if err != nil {
		t.Fatalf("dmesg(ctx) = err %v want nil", err)
	}
	for i, line := range dmesgout {
		if kernelOopsRe.MatchString(line) {
			t.Logf("found kernel oops:")
			trace, err := cutKernelTrace(i, dmesgout)
			if err != nil {
				t.Errorf("cutKernelTrace(%d, dmesg) = err %v, want nil\ndmesg[%d] = %q", i, err, i, dmesgout[i])
			} else {
				t.Log(strings.Join(trace, "\n"))
			}
		}
		if kernelWarnRe.MatchString(line) {
			t.Logf("found kernel warn:")
			trace, err := cutKernelTrace(i, dmesgout)
			if err != nil {
				t.Errorf("cutKernelTrace(%d, dmesg) = err %v, want nil\ndmesg[%d] = %q", i, err, i, dmesgout[i])
			} else {
				t.Log(strings.Join(trace, "\n"))
			}
		}
	}
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
		{cmd: []string{"dmesg"}, id: files.Dmesg},
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
