//go:build arm64

/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package tar2efi

import (
	"os/exec"
	"runtime"
	"strings"

	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apkoaas/pkg/utils"
)

const qemuCommand = "qemu-system-aarch64"

var TargetArch = types.ParseArchitecture("aarch64")

var baseQEMUArgs []string

func init() {
	args := []string{
		"-machine", "virt,accel=tcg",
		"-cpu", "cortex-a76",
	}

	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "kern.hv_support").Output()
		if err == nil && strings.Contains(string(out), "1") {
			args = []string{
				"-machine", "virt", "-cpu", "host", "-accel", "hvf",
			}
		}
	case "linux":
		if utils.CanUseKVM() {
			args = []string{
				"-machine", "virt", "-cpu", "host", "-accel", "kvm",
			}
		}
	}

	baseQEMUArgs = args
}
