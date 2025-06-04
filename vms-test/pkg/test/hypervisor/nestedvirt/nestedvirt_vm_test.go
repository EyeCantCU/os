//go:build vmtest

package nestedvirt

import (
	"os"
	"os/exec"
	"os/user"
	"testing"

	"chainguard.dev/wolfi-vm/vm-test/pkg/vmtest"
)

func TestNestedKVM(t *testing.T) {
	ctx := vmtest.Context(t)
	var cmd *exec.Cmd
	// This won't fail unless it's a broken module, but it will also load
	// kvm_{intel,amd,...}, and if nested virt is disabled those will all
	// fail and nothing will create /dev/kvm.
	if user, err := user.Current(); err == nil && user.Uid != "0" {
		cmd = exec.CommandContext(ctx, "sudo", "modprobe", "kvm")
	} else {
		cmd = exec.CommandContext(ctx, "modprobe", "kvm")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("exec.CommandContext(ctx, modprobe, kvm) = err %v, want nil\noutput:%v", err, out)
	}
	if _, err := os.Stat("/dev/kvm"); err != nil {
		t.Fatalf("os.Stat(/dev/kvm) = err %v want nil", err)
	}
}
