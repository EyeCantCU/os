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

	// Ignored oops and warnings patterns. Matches against the entire trace.
	// Use the inline modifier "(?s)" to activate dot-all mode, i.e. to make
	// "." match newlines as well.
	ignoredPatterns = []*regexp.Regexp{
		// Ignore failure to initialize ftrace on aarch64. The pattern
		// intentionally does not match on x86_64 which works fine,
		// accomplished by matching on the Program Counter (PC) register
		// not found on x86_64 (called Instruction Pointer (IP/EIP/RIP)
		// there).
		// https://github.com/chainguard-dev/wolfi-vm/issues/494
		regexp.MustCompile("(?s)WARNING:.*pc : ftrace_bug.*Call trace:.*ftrace_process_locs"),
	}
)

func shouldIgnoreTrace(trace []string) bool {
	traceText := strings.Join(trace, "\n")
	for _, pattern := range ignoredPatterns {
		if pattern.MatchString(traceText) {
			return true
		}
	}
	return false
}

func TestErrorsInDmesg(t *testing.T) {
	ctx := vmtest.Context(t)
	dmesgout, err := dmesg(ctx)
	if err != nil {
		t.Fatalf("dmesg(ctx) = err %v want nil", err)
	}
	for i, line := range dmesgout {
		if kernelOopsRe.MatchString(line) {
			trace, err := cutKernelTrace(i, dmesgout)
			if err != nil {
				t.Errorf("cutKernelTrace(%d, dmesg) = err %v, want nil\ndmesg[%d] = %q", i, err, i, dmesgout[i])
			} else if shouldIgnoreTrace(trace) {
				t.Log("ignored known kernel oops:")
				t.Log(strings.Join(trace, "\n"))
			} else {
				t.Errorf("found kernel oops:")
				t.Log(strings.Join(trace, "\n"))
			}
		}
		if kernelWarnRe.MatchString(line) {
			trace, err := cutKernelTrace(i, dmesgout)
			if err != nil {
				t.Errorf("cutKernelTrace(%d, dmesg) = err %v, want nil\ndmesg[%d] = %q", i, err, i, dmesgout[i])
			} else if shouldIgnoreTrace(trace) {
				t.Log("ignored known kernel warning:")
				t.Log(strings.Join(trace, "\n"))
			} else {
				t.Errorf("found kernel warn:")
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
