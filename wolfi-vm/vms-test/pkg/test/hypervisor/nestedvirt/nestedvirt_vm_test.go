//go:build vmtest

package nestedvirt

import (
	"os"
	"testing"
)

func TestNestedKVM(t *testing.T) {
	if _, err := os.Stat("/dev/kvm"); err != nil {
		t.Fatalf("os.Stat(/dev/kvm) = err %v want nil", err)
	}
}
