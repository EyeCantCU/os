package vmtest

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	// Import this so that any package importing this also gets the flags declared by artifacts
	_ "chainguard.dev/wolfi-vm/vm-test/pkg/artifacts"
)

// Context returns an appropriate context for the given test.
func Context(t *testing.T) context.Context {
	var ctx context.Context
	var cancel context.CancelFunc

	if deadline, ok := t.Deadline(); ok {
		timeoutFactor := 0.9

		if factorStr := os.Getenv("VMTEST_TIMEOUT_FACTOR"); factorStr != "" {
			if factor, err := strconv.ParseFloat(factorStr, 64); err == nil && factor > 0 && factor <= 1 {
				timeoutFactor = factor
			}
		}

		timeout := time.Until(deadline) * time.Duration(timeoutFactor*100) / 100
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
		} else {
			ctx, cancel = context.WithCancel(context.Background())
		}

	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	t.Cleanup(func() { cancel() })
	return ctx
}
