package vmtest

import (
	"context"
	"flag"
	"testing"
	"time"

	// Import this so that any package importing this also gets the flags declared by artifacts
	_ "chainguard.dev/wolfi-vm/vm-test/pkg/artifacts"
)

var (
	deadline time.Time
	timeout  time.Duration
)

func init() {
	var deadlineStr string
	var timeoutStr string

	flag.StringVar(&timeoutStr, "timeout", "", "Test context timeout (e.g., 30s, 2m)")

	if deadlineStr != "" {
		parsed, err := parseFlexibleTime(deadlineStr)
		if err != nil {
			panic("invalid deadline format: " + err.Error())
		}
		deadline = parsed
	}

	if timeoutStr != "" {
		var err error
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			panic("invalid timeout format: " + err.Error())
		}
	}
}

func parseFlexibleTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.ANSIC,
		time.UnixDate,
		time.RubyDate,
		time.Stamp,
		time.Kitchen,
		time.DateTime,
		time.TimeOnly,
	}

	var err error
	for _, format := range formats {
		var t time.Time
		t, err = time.Parse(format, s)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, err
}

// Context returns an appropriate context for the given test.
func Context(t *testing.T) context.Context {
	var ctx context.Context
	var cancel context.CancelFunc

	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	t.Cleanup(func() { cancel() })
	return ctx
}
