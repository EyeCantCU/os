package boot

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var (
	procUptime = "/proc/uptime"
	systemctl  = "systemctl"
)

func systemctlShow(ctx context.Context, service string) (map[string]string, error) {
	rmap := map[string]string{}
	cmd := exec.CommandContext(ctx, systemctl, "show", service)
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

func monotonicToDuration(val string) (time.Duration, error) {
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return time.Duration(-1), fmt.Errorf("failed to parse integer from monotonic string %s: %v", val, err)
	}

	return time.Duration(n) * time.Microsecond, nil
}

func getServiceStartMonotonic(ctx context.Context, service string) (time.Duration, error) {
	var info map[string]string
	var err error

	for {
		info, err = systemctlShow(ctx, service)
		if err != nil {
			return -1, fmt.Errorf("failed to check service %q state: %w", service, err)
		}

		if val, ok := info["ActiveEnterTimestampMonotonic"]; ok {
			return monotonicToDuration(val)
		}

		select {
		case <-time.After(time.Second):
			// Wait and retry
		case <-ctx.Done():
			return -1, fmt.Errorf("context expired before service %q became active: %w", service, ctx.Err())
		}
	}
}
