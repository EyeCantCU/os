//go:build vmtest

package docker

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"chainguard.dev/wolfi-vm/vm-test/pkg/vmtest"
)

const (
	pullAttempts      = 3
	pullBackoffFactor = 1
)

func TestDockerPullAndRun(t *testing.T) {
	ctx := vmtest.Context(t)
	var pullCmd *exec.Cmd
	var err error
	var out []byte

	for i := 0; i < pullAttempts; i++ {
		pullCmd = exec.CommandContext(ctx, "docker", "pull", "-q", "cgr.dev/chainguard/wolfi-base:latest")
		out, err = pullCmd.CombinedOutput()
		if err == nil {
			break
		}
		time.Sleep(time.Duration(i*pullBackoffFactor) * time.Second)
	}
	if err != nil {
		t.Fatalf("exec.CommandContext(ctx, %s) = err %v after %d attempts, want nil\noutput:%s", pullCmd.String(), err, pullAttempts, string(out))
	}

	// See if we can pull a wolfi-base container image and run it
	runCmd := exec.CommandContext(ctx, "docker", "run", "-q", "cgr.dev/chainguard/wolfi-base:latest", "echo", "Hello Wolfi")

	out, err = runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exec.CommandContext(ctx, %s) = err %v, want nil\noutput:%s", runCmd.String(), err, string(out))
	}

	// We would expect to get "Hello Wolfi"
	output := strings.TrimSpace(string(out))
	expected := "Hello Wolfi"
	if output != expected {
		t.Errorf("docker run output = %q, want %q", output, expected)
	}
}
