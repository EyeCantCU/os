package tar2efi

import (
	"os/exec"
	"runtime"
	"strings"

	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apkoaas/pkg/utils"
)

type qemuInfo struct {
	Command     string
	Console     string
	MachineArgs []string
}

var QemuInfo = map[types.Architecture]qemuInfo{}

func init() {
	arch := types.ParseArchitecture("x86_64")
	mAargs := []string{"-machine", "q35,accel=tcg", "-cpu", "Haswell-v4"}
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" && utils.CanUseKVM() {
		mAargs = []string{"-machine", "q35,accel=kvm", "-cpu", "max"}
	}
	QemuInfo[arch] = qemuInfo{
		Command:     "qemu-system-" + arch.ToQEmu(),
		MachineArgs: mAargs,
		Console:     "ttyS0",
	}

	arch = types.ParseArchitecture("aarch64")
	mAargs = []string{"-machine", "virt,accel=tcg", "-cpu", "cortex-a76"}
	if runtime.GOARCH == "arm64" {
		switch runtime.GOOS {
		case "darwin":
			out, err := exec.Command("sysctl", "kern.hv_support").Output()
			if err == nil && strings.Contains(string(out), "1") {
				mAargs = []string{"-machine", "virt,accel=hvf", "-cpu", "host"}
			}
		case "linux":
			if utils.CanUseKVM() {
				mAargs = []string{"-machine", "virt,accel=kvm", "-cpu", "host"}
			}
		}
	}
	QemuInfo[arch] = qemuInfo{
		Command:     "qemu-system-" + arch.ToQEmu(),
		MachineArgs: mAargs,
		Console:     "ttyAMA0"}
}
