package boot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	systemdTimeFormat = "Mon 2006-01-02 15:04:05 MST"
)

// findVMStartTime reads system uptime from /proc/uptime and computes the instance start time.
func findVMStartTime(ctx context.Context) (time.Time, error) {
	uptimeData, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to read /proc/uptime: %w", err)
	}

	fields := strings.Fields(string(uptimeData))
	if len(fields) < 1 {
		return time.Time{}, errors.New("unexpected format in /proc/uptime")
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
		cmd := exec.CommandContext(ctx, "systemctl", "show", "--property=ActiveState", service)
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

	// TODO: use ActiveEnterMonotonicTimestamp, not wall clock timestamp
	cmd := exec.CommandContext(ctx, "systemctl", "show", "--property=ActiveEnterTimestamp", service)
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get ActiveEnterTimestamp for %q: %w", service, err)
	}

	timestamp := strings.TrimPrefix(strings.TrimSpace(string(output)), "ActiveEnterTimestamp=")
	startTime, err := time.Parse(systemdTimeFormat, timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse ActiveEnterTimestamp %q: %w", timestamp, err)
	}

	return startTime, nil
}

