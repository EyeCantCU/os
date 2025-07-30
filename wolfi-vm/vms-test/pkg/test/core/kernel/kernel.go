package kernel

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const (
	dmesgCli = "dmesg"
	sudoCli  = "sudo"
)

// run dmesg, if not root run sudo dmesg
func dmesg(ctx context.Context, args ...string) ([]string, error) {
	cmd := exec.CommandContext(ctx, dmesgCli, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	return strings.Split(string(output), "\n"), nil
}

// cut kernel trace out around line n
func cutKernelTrace(n int, lines []string) ([]string, error) {
	if n < 0 || n >= len(lines) {
		return nil, fmt.Errorf("line number out of bounds")
	}

	start := n
	for start >= 0 {
		if strings.Contains(lines[start], "cut here") {
			break
		}
		start--
	}
	if start == -1 {
		return nil, fmt.Errorf(`"cut here" not found before line 0`)
	}

	end := n
	for end < len(lines) {
		if strings.Contains(lines[end], "end trace") {
			break
		}
		end++
	}
	if end == len(lines) {
		return nil, fmt.Errorf(`"end trace" not found within the rest of the log`)
	}

	return lines[start : end+1], nil
}
