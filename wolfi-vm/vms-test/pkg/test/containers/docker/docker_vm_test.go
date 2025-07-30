//go:build vmtest

package docker

import (
	"os/exec"
	"strings"
	"testing"

	"chainguard.dev/wolfi-vm/vm-test/pkg/vmtest"
)

func TestDockerPullAndRun(t *testing.T) {
	ctx := vmtest.Context(t)
	var cmd *exec.Cmd

	// See if we can pull a wolfi-base container image and run it
	cmd = exec.CommandContext(ctx, "docker", "run", "-q", "cgr.dev/chainguard/wolfi-base:latest", "echo", "Hello Wolfi")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("exec.CommandContext(ctx, %s) = err %v, want nil\noutput:%s", cmd.String(), err, string(out))
		return
	}

	// We would expect to get "Hello Wolfi"
	output := strings.TrimSpace(string(out))
	expected := "Hello Wolfi"
	if output != expected {
		t.Errorf("docker run output = %q, want %q", output, expected)
	}
}
