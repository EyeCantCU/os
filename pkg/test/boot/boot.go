package boot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var (
	procUptime = "/proc/uptime"
	systemctl  = "systemctl"
)

// findVMStartTime reads system uptime from /proc/uptime and computes the instance start time.
func findVMStartTime(ctx context.Context) (time.Time, error) {
	uptimeData, err := os.ReadFile(procUptime)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to read %s: %w", procUptime, err)
	}

	fields := strings.Fields(string(uptimeData))
	if len(fields) < 1 {
		return time.Time{}, fmt.Errorf("unexpected format in %s", procUptime)
	}

	uptimeSeconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse uptime value %q: %w", fields[0], err)
	}

	instanceStartTime := time.Now().Add(-time.Duration(uptimeSeconds) * time.Second)
	return instanceStartTime, nil
}

// findServiceStartTime waits for a service to become active, then returns its start time using systemd.
func findServiceStartTime(ctx context.Context, service string) (time.Time, error) {
	for {
		cmd := exec.CommandContext(ctx, systemctl, "show", "--property=ActiveState", service)
		output, err := cmd.Output()
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to check service %q state: %w", service, err)
		}

		if strings.Contains(string(output), "ActiveState=active") {
			break
		}

		select {
		case <-time.After(time.Second):
			// Wait and retry
		case <-ctx.Done():
			return time.Time{}, fmt.Errorf("context expired before service %q became active: %w", service, ctx.Err())
		}
	}

	cmd := exec.CommandContext(ctx, systemctl, "show", "--property=ActiveEnterMonotonicTimestamp", service)
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get ActiveEnterMonotonicTimestamp for %q: %w", service, err)
	}

	timestamp := strings.TrimPrefix(strings.TrimSpace(string(output)), "ActiveEnterMonotonicTimestamp=")
	i, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse ActiveEnterMonotonicTimestamp %q: %w", timestamp, err)
	}
	return time.UnixMicro(i), nil
}
