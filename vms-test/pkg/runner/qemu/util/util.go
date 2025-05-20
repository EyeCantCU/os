package util

import (
	"context"
	"fmt"
	"net"
	"time"
)

// PollUnixSocket attempts to dial the UNIX socket at socketPath until success or timeout.
// It respects cancellation via the provided context and returns a net.Conn or an error.
func PollUnixSocket(ctx context.Context, socketPath string, timeout time.Duration) (net.Conn, error) {
	const pollInterval = 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		// Try to dial
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			return conn, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context canceled or timeout reached while waiting for socket %q: %w", socketPath, ctx.Err())
		case <-ticker.C:
			// Try again on next tick
		}
	}
}
