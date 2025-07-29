//go:build unittest

package kernel

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCutKernelTrace(t *testing.T) {
	log := `[Wed Apr  4 19:51:54 2018] ------------[ cut here ]------------
[Wed Apr  4 19:51:54 2018] WARNING: at kernel/irq/manage.c:1244 __free_irq+0xa7/0x200()
[Wed Apr  4 19:51:54 2018] Trying to free already-free IRQ 18
[Wed Apr  4 19:51:54 2018] Modules linked in: ...
[Wed Apr  4 19:51:54 2018] CPU: 0 PID: 8840 Comm: ifconfig ...
[Wed Apr  4 19:51:54 2018] Hardware name: System manufacturer ...
[Wed Apr  4 19:51:54 2018] Call Trace:
[Wed Apr  4 19:51:54 2018]  [<ffffffff814a9ec3>] ? dump_stack+0xc/0x15
[Wed Apr  4 19:51:54 2018] ---[ end trace 14fdad943159d686 ]---
[Wed Apr  4 19:51:54 2018] ------------[ cut here ]------------
[Wed Apr  4 19:51:54 2018] WARNING: at kernel/irq/manage.c:1244 __free_irq+0xa7/0x200()
[Wed Apr  4 19:51:54 2018] Trying to free already-free IRQ 18
[Wed Apr  4 19:51:54 2018] Modules linked in: ...
[Wed Apr  4 19:51:54 2018] Call Trace:
[Wed Apr  4 19:51:54 2018]  [<ffffffff8109c837>] ? __free_irq+0xa7/0x200
[Wed Apr  4 19:51:54 2018] ---[ end trace 14fdad943159d687 ]---
[Wed Apr  4 19:51:54 2018] ip_tables: (C) 2000-2006 Netfilter Core Team
[Wed Apr  4 19:51:54 2018] nf_conntrack version 0.5.0 ...`

	loglines := strings.Split(log, "\n")
	tests := []struct {
		name     string
		line     int
		lines    []string
		wantSpan []string
	}{
		{
			name:     "First_trace_start",
			line:     0,
			lines:    loglines,
			wantSpan: loglines[0:9],
		},
		{
			name:     "First_trace_end",
			line:     8,
			lines:    loglines,
			wantSpan: loglines[0:9],
		},
		{
			name:     "First_trace_midpoint",
			line:     4,
			lines:    loglines,
			wantSpan: loglines[0:9],
		},
		{
			name:     "Second_trace_midpoint",
			line:     13,
			lines:    loglines,
			wantSpan: loglines[9:16],
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := cutKernelTrace(tt.line, tt.lines)
			if err != nil {
				t.Errorf("cutKernelTrace(%d) = error %v, want nil", tt.line, err)
				return
			}
			if diff := cmp.Diff(tt.wantSpan, got); diff != "" {
				t.Errorf("cutKernelTrace(%d) mismatch (-want +got):\n%s", tt.line, diff)
			}
		})
	}

}

func TestCutKernelTraceError(t *testing.T) {
	lines := []string{
		"[timestamp] unrelated log",
		"[timestamp] ---[ end trace ]---",
	}

	tests := []struct {
		name string
		line int
	}{
		{"LineOutOfBoundsNegative", -1},
		{"LineOutOfBoundsTooLarge", 10},
		{"NoCutHereFound", 1},
		{"NoEndTraceFound", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cutKernelTrace(tt.line, lines)
			if err == nil {
				t.Errorf("cutKernelTrace(%d) = nil error, want error", tt.line)
			}
		})
	}
}
