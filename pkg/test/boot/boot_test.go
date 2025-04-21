//go:build unittest

package boot

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindVMStartTime(t *testing.T) {
	tests := []struct {
		name           string
		uptimeContents string
		expectTime     time.Time
	}{
		{
			name:           "basic_uptime_parsing",
			uptimeContents: "6830 102264.36",
			expectTime:     time.Now().Add(-6830 * time.Second),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testProcUptime := filepath.Join(t.TempDir(), "procUptime")
			if err := os.WriteFile(testProcUptime, []byte(tt.uptimeContents), 0644); err != nil {
				t.Fatalf("os.WriteFile(%s, %s, 0644) = %v, want nil", testProcUptime, tt.uptimeContents, err)
			}
			procUptimeOld := procUptime
			procUptime = testProcUptime
			t.Cleanup(func() { procUptime = procUptimeOld })
			startTime, err := findVMStartTime(t.Context())
			if err != nil {
				t.Fatalf("findVMStartTime(ctx) = err %v, want nil", err)
			}
			// There are minor differences in time.Now when we start the test and when we compre this
			// but as long as it's less than one second it's fine.
			if diff := int(startTime.Sub(tt.expectTime).Seconds()); diff != 0 {
				t.Fatalf("%q.Sub(%q).Seconds() = %d, want 0", startTime, tt.expectTime, diff)
			}
		})
	}
}

func TestFindServiceStartTime(t *testing.T) {
	testSystemctlFormat := `#!/bin/sh
if [ "$1" = "show" ]; then
    for arg in "$@"; do
        case "$arg" in
            --property=ActiveState)
                echo "ActiveState=active"
                exit 0
                ;;
            --property=ActiveEnterMonotonicTimestamp)
                echo "ActiveEnterMonotonicTimestamp=%d"
                exit 0
                ;;
        esac
    done
fi
echo "unexpected args: $@" >&2
exit 1
`

	tests := []struct {
		name       string
		service    string
		timestamp  int
		expectTime time.Time
	}{
		{
			name:       "mocked_systemctl_start_time",
			service:    "mock-service",
			timestamp:  982349843,
			expectTime: time.UnixMicro(982349843),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSystemctl := filepath.Join(t.TempDir(), "systemctl")
			testSystemctlContents := fmt.Sprintf(testSystemctlFormat, tt.timestamp)

			if err := os.WriteFile(testSystemctl, []byte(testSystemctlContents), 0755); err != nil {
				t.Fatalf("os.WriteFile(%s, %q, 0755) = %v want nil", testSystemctl, testSystemctlContents, err)
			}

			systemctlOld := systemctl
			systemctl = testSystemctl
			t.Cleanup(func() { systemctl = systemctlOld })

			startTime, err := findServiceStartTime(t.Context(), tt.service)
			if err != nil {
				t.Fatalf("findServiceStartTime(ctx, %s) = err %v want nil", tt.service, err)
			}

			if !startTime.Equal(tt.expectTime) {
				t.Fatalf("%q.Equal(%q) = false, want true", startTime, tt.expectTime)
			}
		})
	}
}
