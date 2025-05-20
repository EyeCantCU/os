//go:build unittest

package boot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetServiceStartTime(t *testing.T) {
	testSystemctlFormat := `#!/bin/sh
if [ "$1" = "show" ]; then
cat <<"EOF"
%s
EOF
    exit
fi
echo "unexpected args:" "$@" >&2
exit 1
`

	tests := []struct {
		name       string
		service    string
		info       map[string]string
		expectTime time.Duration
	}{
		{
			name:    "mocked_systemctl_start_time",
			service: "mock-service",
			info: map[string]string{
				"ActiveEnterTimestampMonotonic": "3023385",
				"SubState":                      "running",
				"ActiveState":                   "active",
			},
			expectTime: time.Duration(3023385) * time.Microsecond,
		},
	}

	toOutput := func(m map[string]string) []byte {
		var sb strings.Builder
		for k, v := range m {
			sb.WriteString(k + "=" + v + "\n")
		}
		return []byte(fmt.Sprintf(testSystemctlFormat, sb.String()))
	}

	testSystemctl := filepath.Join(t.TempDir(), "systemctl")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSystemctlContents := toOutput(tt.info)
			if err := os.WriteFile(testSystemctl, testSystemctlContents, 0755); err != nil {
				t.Fatalf("os.WriteFile(%s, %q, 0755) = %v want nil", testSystemctl, testSystemctlContents, err)
			}

			systemctlOld := systemctl
			systemctl = testSystemctl
			t.Cleanup(func() { systemctl = systemctlOld })

			startTime, err := getServiceStartMonotonic(t.Context(), tt.service)
			if err != nil {
				t.Fatalf("getServiceStartMonotonic(ctx, %s) = err %v want nil", tt.service, err)
			}

			if startTime != tt.expectTime {
				t.Fatalf("%q != %q, want true", startTime, tt.expectTime)
			}
		})
	}
}
