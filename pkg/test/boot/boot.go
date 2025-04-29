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

func systemctlShow(service string) (map[string]string, error) {
	rmap := map[string]string{}
	cmd := exec.Command(systemctl, "show", service)
	output, err := cmd.Output()
	if err != nil {
		return rmap, err
	}
	for _, line := range strings.Split(string(output), "\n") {
		if line == "" {
			continue
		}
		toks := strings.SplitN(line, "=", 2)
		if len(toks) != 2 {
			return rmap, fmt.Errorf("line had no =: %s", line)
		}
		rmap[toks[0]] = toks[1]
	}
	return rmap, nil
}

func getMonotonicProperty(showInfo map[string]string, prop string) (time.Duration, error) {
	errDuration := time.Duration(-1 * time.Second)
	strval, ok := showInfo[prop]
	if !ok {
		return errDuration, fmt.Errorf("no %s field in output of systemctl show %s", prop, showInfo)
	}

	n, err := strconv.ParseInt(strval, 10, 64)
	if err != nil {
		return errDuration, fmt.Errorf("failed to parse integer from %s: %v", strval, err)
	}

	return time.Duration(n) * time.Microsecond, nil
}

func findServiceStartMonotonic(ctx context.Context, service string) (time.Duration, error) {
	for {
		cmd := exec.CommandContext(ctx, systemctl, "show", "--property=ActiveState", service)
		output, err := cmd.Output()
		if err != nil {
			return -1, fmt.Errorf("failed to check service %q state: %w", service, err)
		}

		if strings.Contains(string(output), "ActiveState=active") {
			break
		}

		select {
		case <-time.After(time.Second):
			// Wait and retry
		case <-ctx.Done():
			return -1, fmt.Errorf("context expired before service %q became active: %w", service, ctx.Err())
		}
	}

	info, err := systemctlShow(service)
	if err != nil {
		return -1, err
	}

	return getMonotonicProperty(info, "ActiveEnterTimestampMonotonic")
}
