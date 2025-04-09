//go:build unittest
package boot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// helper to write a fake systemctl binary to simulate systemctl responses
func writeMockSystemctl(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	mockPath := filepath.Join(tmpDir, "systemctl")

	content := `#!/bin/sh
if [ "$1" = "show" ]; then
    for arg in "$@"; do
        case "$arg" in
            --property=ActiveState)
                echo "ActiveState=active"
                exit 0
                ;;
            --property=ActiveEnterTimestamp)
                echo "ActiveEnterTimestamp=Mon 2023-11-20 11:22:33 UTC"
                exit 0
                ;;
        esac
    done
fi
echo "unexpected args: $@" >&2
exit 1
`
	if err := os.WriteFile(mockPath, []byte(content), 0755); err != nil {
		t.Fatalf("failed to write mock systemctl: %v", err)
	}
	return tmpDir
}

func TestFindVMStartTime(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "basic_uptime_parsing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := findVMStartTime(context.Background())
			if err != nil {
				t.Fatalf("findVMStartTime(ctx) = err %v, want nil", err)
			}
		})
	}
}

func TestFindServiceStartTime(t *testing.T) {
	tests := []struct {
		name    string
		service string
	}{
		{
			name:    "mocked_systemctl_start_time",
			service: "mock-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDir := writeMockSystemctl(t)

			// Inject our mock systemctl into PATH
			origPath := os.Getenv("PATH")
			defer os.Setenv("PATH", origPath)
			os.Setenv("PATH", mockDir+":"+origPath)

			startTime, err := findServiceStartTime(context.Background(), tt.service)
			if err != nil {
				t.Fatalf("findServiceStartTime(ctx, %s) = err %v want nil", tt.service, err)
			}

			expected, _ := time.Parse(systemdTimeFormat, "Mon 2023-11-20 11:22:33 UTC")
			if !startTime.Equal(expected) {
				t.Errorf("findServiceStartTime(ctx, %s) = %s, want %s", tt.service, startTime, expected)
			}
		})
	}
}

