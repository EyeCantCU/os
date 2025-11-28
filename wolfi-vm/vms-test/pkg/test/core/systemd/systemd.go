package systemd

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

type cachedJournal struct {
	entries     []string
	mutex       sync.Mutex
	initialized bool
}

var journal cachedJournal

func JournalEntries(t *testing.T, ctx context.Context) []string {
	t.Helper()

	journal.mutex.Lock()
	defer journal.mutex.Unlock()

	if !journal.initialized {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "journalctl", "-b0")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("error executing journalctl -b0: %v", err)
		}

		journal.entries = strings.Split(string(output), "\n")
		journal.initialized = true
	}

	return journal.entries
}
